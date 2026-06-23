package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/color"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/NimbleMarkets/ntcharts/v2/barchart"
	"github.com/NimbleMarkets/ntcharts/v2/canvas"
	"github.com/NimbleMarkets/ntcharts/v2/canvas/runes"
	"github.com/NimbleMarkets/ntcharts/v2/linechart"
	"github.com/NimbleMarkets/ntcharts/v2/linechart/timeserieslinechart"
	"github.com/NimbleMarkets/ntcharts/v2/linechart/wavelinechart"
	"github.com/NimbleMarkets/ntcharts/v2/sparkline"
	"github.com/charmbracelet/x/ansi"
	"github.com/xZhad/jsonldb"
)

// cell pads or truncates s to exactly w display cells (wide-char aware, so CJK
// and emoji don't misalign the grid).
func cell(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if ansi.StringWidth(s) > w {
		s = ansi.Truncate(s, w, "…")
	}
	if pad := w - ansi.StringWidth(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

// clip truncates s to at most n display cells with an ellipsis (no padding).
func clip(s string, n int) string {
	if ansi.StringWidth(s) <= n {
		return s
	}
	return ansi.Truncate(s, n, "…")
}

func (m *Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion // enable wheel scrolling
	return v
}

func (m *Model) render() string {
	w, h := m.width, m.height
	if w < 24 || h < 8 {
		return "terminal too small — resize"
	}
	if m.showHelp {
		return m.renderHelp(w, h)
	}
	if m.mode == ModeColumns {
		return m.renderColumns(w, h)
	}
	if m.mode == ModeStats {
		return m.renderStats(w, h)
	}
	if m.mode == ModeGroup {
		return m.renderGroup(w, h)
	}
	if m.mode == ModeChart {
		return m.renderChart(w, h)
	}
	if m.mode == ModeDetail || m.mode == ModeDetailSearch {
		return m.renderDetail(w, h)
	}
	title := m.renderTitle(w)
	footer := m.renderFooter(w)
	bodyH := h - lipgloss.Height(title) - lipgloss.Height(footer)
	if bodyH < 3 {
		bodyH = 3
	}
	return lipgloss.JoinVertical(lipgloss.Left, title, m.renderBody(w, bodyH), footer)
}

func (m *Model) currentLabel() string {
	if len(m.files) > 1 {
		return filepath.Base(filepath.Dir(m.files[0])) + "/"
	}
	return filepath.Base(m.files[0])
}

func (m *Model) renderTitle(w int) string {
	left := " " + gradientText("lazyjsonl", m.frame) + " " + stylePath.Render("· "+m.currentLabel())
	right := styleCount.Render(fmt.Sprintf("%d records ", m.result.Count()))
	return bar(w, left, right)
}

func (m *Model) renderBody(w, h int) string {
	if len(m.files) <= 1 {
		return m.tablePane(w, h, m.focus == FocusTable)
	}
	filesW := 32
	if filesW > w/2 {
		filesW = w / 2
	}
	files := m.filesPane(filesW, h, m.focus == FocusFiles)
	table := m.tablePane(w-filesW, h, m.focus == FocusTable)
	return lipgloss.JoinHorizontal(lipgloss.Top, files, table)
}

func (m *Model) filesPane(w, h int, active bool) string {
	inner := w - 2
	innerH := h - 2
	contentW := inner - 1 // rightmost column reserved for the scrollbar
	if contentW < 2 {
		contentW = 2
	}
	listH := innerH - 1
	if listH < 1 {
		listH = 1
	}
	cf := m.curFiles()
	start := 0
	if m.fileIdx >= listH {
		start = m.fileIdx - listH + 1
	}
	title := paneTitle(active, "FILES")
	if m.fileFilter != "" {
		title += styleScrollHint.Render(fmt.Sprintf("  %d/%d", len(cf), len(m.files)))
	}
	sb := scrollbar(listH, len(cf), listH, start)
	lines := []string{padTo(title, contentW) + " "}
	for j := 0; j < listH; j++ {
		i := start + j
		var content string
		switch {
		case i < len(cf):
			name := filepath.Base(cf[i])
			if i == m.fileIdx {
				row := styleSelGut.Render("▌ ") + styleSel.Render(cell(name, contentW-2))
				content = styleSel.Width(contentW).Render(row)
			} else {
				content = padTo(styleMuted.Render("  "+cell(name, contentW-2)), contentW)
			}
		default:
			content = padTo("", contentW)
		}
		sc := " "
		if j < len(sb) {
			sc = sb[j]
		}
		lines = append(lines, content+sc)
	}
	// lipgloss .Width/.Height are TOTAL (border included) → pass full w/h
	return pane(active).Width(w).Height(h).MaxHeight(h).Render(strings.Join(lines, "\n"))
}

func (m *Model) tablePane(w, h int, active bool) string {
	inner := w - 2
	innerH := h - 2
	if innerH < 1 {
		innerH = 1
	}
	contentW := inner - 1 // rightmost column reserved for the scrollbar
	if contentW < 4 {
		contentW = 4
	}

	pageDocs := m.pageRows()
	allCols := m.activeColumns()

	// Empty state: a centered, styled message instead of a bare grid.
	if len(pageDocs) == 0 {
		msg := "no records"
		if m.filter != "" {
			msg = "no matches for ‘" + m.filter + "’ — esc to clear"
		}
		title := paneTitle(active, "RECORDS")
		box := lipgloss.Place(inner, innerH-1, lipgloss.Center, lipgloss.Center, styleMuted.Render(msg))
		return pane(active).Width(w).Height(h).MaxHeight(h).Render(padTo(title, inner) + "\n" + box)
	}

	// Window of columns starting at colOffset (horizontal scroll). Greedily keep
	// the columns that fit contentW (each costs width + 2 padding + 1 separator),
	// so short-value columns aren't starved to a "…" header by lipgloss.
	var headers []string
	var data [][]string // column-major while building, transposed below
	used := 0
	for _, c := range allCols[min(m.colOffset, len(allCols)):] {
		label := m.headerLabel(c)
		colCells := make([]string, len(pageDocs))
		wmax := len([]rune(label))
		for r, d := range pageDocs {
			txt := clip(m.displayText(d, c), 32)
			colCells[r] = txt
			if l := len([]rune(txt)); l > wmax {
				wmax = l
			}
		}
		if len(headers) > 0 && used+wmax+3 > contentW {
			break
		}
		used += wmax + 3
		headers = append(headers, label)
		data = append(data, colCells)
	}
	cols := allCols[m.colOffset : m.colOffset+len(headers)] // visible window
	curVis := m.colCursor - m.colOffset                     // cursor index within window

	rowsOut := make([][]string, len(pageDocs))
	for r := range pageDocs {
		cells := make([]string, len(headers))
		for ci := range headers {
			cells[ci] = data[ci][r]
		}
		rowsOut[r] = cells
	}

	tbl := table.New().
		Headers(headers...).
		Rows(rowsOut...).
		Wrap(false).
		BorderColumn(true).BorderHeader(true).BorderRow(false).
		BorderTop(false).BorderBottom(false).BorderLeft(false).BorderRight(false).
		BorderStyle(lipgloss.NewStyle().Foreground(cIdle)).
		Width(contentW).
		StyleFunc(func(row, col int) lipgloss.Style {
			switch {
			case row == table.HeaderRow:
				if col < len(cols) && (cols[col] == m.sortField || (active && col == curVis)) {
					return styleSortCol.Padding(0, 1)
				}
				return styleHeader.Padding(0, 1)
			case row == m.cursor:
				return styleSel.Padding(0, 1)
			case row < len(pageDocs) && col < len(cols):
				_, st := cellValue(pageDocs[row], cols[col])
				st = st.Padding(0, 1)
				if row%2 == 1 { // zebra striping
					st = st.Background(cZebra)
				}
				return st
			default:
				return styleText.Padding(0, 1)
			}
		})

	// Pad the table to a fixed region (pane content minus the title line) so
	// every page fills the pane identically — the bottom border stays put and
	// short last pages don't shift the layout up.
	tableRegion := innerH - 1
	if tableRegion < 1 {
		tableRegion = 1
	}
	tblLines := strings.Split(tbl.String(), "\n")
	sb := scrollbar(tableRegion, m.result.Count(), m.pageSize, (m.page-1)*m.pageSize)
	body := make([]string, tableRegion)
	for i := 0; i < tableRegion; i++ {
		ln := ""
		if i < len(tblLines) {
			ln = tblLines[i]
		}
		sc := " "
		if i < len(sb) {
			sc = sb[i]
		}
		body[i] = padTo(ln, contentW) + sc
	}

	titleStr := "RECORDS"
	if len(m.drillPath) > 0 {
		titleStr += " · " + strings.Join(m.drillCrumbs(), " ▸ ")
	}
	title := paneTitle(active, titleStr)
	if len(allCols) > len(headers) { // columns overflow → show window position
		title += styleScrollHint.Render(fmt.Sprintf("   ‹ %d–%d / %d ›",
			m.colOffset+1, m.colOffset+len(headers), len(allCols)))
	}
	content := padTo(title, inner) + "\n" + strings.Join(body, "\n")
	return pane(active).Width(w).Height(h).MaxHeight(h).Render(content)
}

// cellValue resolves a column's value for a row and the style to draw it in
// (numbers cyan, booleans green/red, null dim, objects/arrays yellow).
func cellValue(d jsonldb.Doc, col string) (string, lipgloss.Style) {
	var raw any
	var ok bool
	if strings.Contains(col, ".") {
		raw, ok = d.Path(col)
	} else {
		raw, ok = d.Get(col)
	}
	if !ok {
		return "", styleText // key absent → blank cell
	}
	switch x := raw.(type) {
	case nil:
		return "∅", styleNull // explicit JSON null
	case bool:
		if x {
			return "✓", styleBoolTrue
		}
		return "✗", styleBoolFls
	}
	return scalarStr(raw), valueStyle(raw)
}

// colGlyph returns a small type indicator for a column, sniffed from the first
// present, non-null value on the current page. Strings get no glyph (the common
// case stays uncluttered); typed columns get a cue.
func (m *Model) colGlyph(c string) string {
	for _, d := range m.pageRows() {
		var v any
		var ok bool
		if strings.Contains(c, ".") {
			v, ok = d.Path(c)
		} else {
			v, ok = d.Get(c)
		}
		if !ok || v == nil {
			continue
		}
		switch x := v.(type) {
		case bool:
			return "⊙"
		case json.Number, float64, int, int64:
			return "#"
		case map[string]any:
			return "◇"
		case []any:
			return "▦"
		case string:
			if isDateString(x) {
				return "◷"
			}
			return ""
		}
		return ""
	}
	return ""
}

func valueStyle(v any) lipgloss.Style {
	switch x := v.(type) {
	case nil:
		return styleNull
	case bool:
		if x {
			return styleBoolTrue
		}
		return styleBoolFls
	case json.Number, float64, int, int64:
		return styleNum
	case map[string]any, []any:
		return styleObj
	case string:
		if isDateString(x) {
			return styleDate
		}
		return styleText
	default:
		return styleText
	}
}

// dateLayouts are the formats we treat as "date-like" for coloring.
var dateLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02",
	"2006/01/02",
	time.RFC1123Z,
	time.RFC1123,
}

// isDateString reports whether s parses as a timestamp in a common layout.
// Length-gated so it stays cheap on the table's hot path.
func isDateString(s string) bool {
	if len(s) < 8 || len(s) > 40 {
		return false
	}
	c := s[0]
	if (c < '0' || c > '9') && (c < 'A' || c > 'Z') { // year digit or weekday name
		return false
	}
	for _, l := range dateLayouts {
		if _, err := time.Parse(l, s); err == nil {
			return true
		}
	}
	return false
}

// docFloat extracts a numeric value at field (plain or dotted) from a doc.
func docFloat(d jsonldb.Doc, field string) (float64, bool) {
	var v any
	var ok bool
	if strings.Contains(field, ".") {
		v, ok = d.Path(field)
	} else {
		v, ok = d.Get(field)
	}
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case json.Number:
		f, e := n.Float64()
		return f, e == nil
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

// fmtNum formats a float without trailing zeros.
func fmtNum(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// sparkBar renders integer counts as a row of block glyphs (a mini histogram).
func sparkBar(counts []int) string {
	blocks := []rune(" ▁▂▃▄▅▆▇█")
	maxC := 1
	for _, c := range counts {
		if c > maxC {
			maxC = c
		}
	}
	ramp := lipgloss.Blend1D(len(counts), cViolet, cMagenta, cCyan)
	var b strings.Builder
	for i, c := range counts {
		idx := c * (len(blocks) - 1) / maxC
		b.WriteString(lipgloss.NewStyle().Foreground(ramp[i]).Render(string(blocks[idx])))
	}
	return b.String()
}

func (m *Model) renderStats(w, h int) string {
	s := m.stats
	var b strings.Builder
	b.WriteString(styleApp.Render("Stats") + styleMuted.Render("  "+s.field) + "\n")
	b.WriteString(gradientRule(34) + "\n")
	rows := [][2]string{
		{"count", fmt.Sprintf("%d", s.count)},
		{"min", thousands(s.min)},
		{"max", thousands(s.max)},
		{"sum", thousands(s.sum)},
		{"mean", thousands(s.mean)},
		{"median", thousands(s.median)},
	}
	for _, kv := range rows {
		b.WriteString(styleKey.Render(cell(kv[0], 9)) + styleNum.Render(kv[1]) + "\n")
	}
	if len(s.hist) > 0 {
		b.WriteString("\n" + styleMuted.Render("distribution") + "\n" + sparkBar(s.hist) + "\n")
	}
	b.WriteString("\n" + styleMuted.Render("any key to close"))
	box := styleOverlay.Render(strings.TrimRight(b.String(), "\n"))
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}

func (m *Model) renderGroup(w, h int) string {
	col := m.groupMeasureCol()
	sortNames := []string{"count↓", "key↑", "sum↓", "avg↓"}
	var b strings.Builder
	info := fmt.Sprintf("   %d groups · sort %s · s sort · m measure", len(m.groupRows), sortNames[m.groupSort])
	if col != "" {
		info += " (" + col + ")"
	}
	info += " · ↵ filter · v chart · esc"
	b.WriteString(styleApp.Render("Group by ") + styleHeader.Render(m.groupField) + styleMuted.Render(info) + "\n")
	b.WriteString(gradientRule(66) + "\n")

	hdr := "  " + styleHeader.Render(cell("value", 22)) + " " + styleHeader.Render(cell("count", 7))
	if col != "" {
		hdr += " " + styleHeader.Render(cell("sum", 11)) + " " + styleHeader.Render(cell("avg", 11))
	}
	b.WriteString(hdr + "\n")

	listH := h - 12
	if listH < 3 {
		listH = 3
	}
	start := 0
	if m.groupCursor >= listH {
		start = m.groupCursor - listH + 1
	}
	maxCount := 1
	for _, r := range m.groupRows {
		if r.count > maxCount {
			maxCount = r.count
		}
	}
	for i := start; i < len(m.groupRows) && i < start+listH; i++ {
		gr := m.groupRows[i]
		mark := "  "
		key := styleText.Render(cell(gr.key, 22))
		if i == m.groupCursor {
			mark = styleGutter.Render("▌ ")
			key = styleSel.Render(cell(gr.key, 22))
		}
		line := mark + key + " " + styleNum.Render(cell(fmt.Sprintf("%d", gr.count), 7))
		if col != "" {
			line += " " + styleNum.Render(cell(fmtNum(gr.sum), 11)) + " " + styleNum.Render(cell(fmt.Sprintf("%.2f", gr.avg), 11))
		} else {
			barW := gr.count * 16 / maxCount
			if barW < 1 && gr.count > 0 {
				barW = 1
			}
			line += " " + styleScrollThumb.Render(strings.Repeat("█", barW))
		}
		b.WriteString(line + "\n")
	}
	box := styleOverlay.Render(strings.TrimRight(b.String(), "\n"))
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}

var barPalette = []color.Color{cMagenta, cViolet, cCyan, cYellow, cGreen, cOrange}

// numericValues collects up to capN numeric values of field across the result.
func (m *Model) numericValues(field string, capN int) []float64 {
	var out []float64
	for _, d := range m.result.Docs() {
		if f, ok := docFloat(d, field); ok {
			out = append(out, f)
			if len(out) >= capN {
				break
			}
		}
	}
	return out
}

// numericColumns / dateColumns sniff column types from the current page.
func (m *Model) numericColumns() []string {
	var out []string
	for _, c := range m.activeColumns() {
		for _, d := range m.pageRows() {
			if _, ok := docFloat(d, c); ok {
				out = append(out, c)
				break
			}
		}
	}
	return out
}

func (m *Model) dateColumns() []string {
	var out []string
	for _, c := range m.activeColumns() {
		for _, d := range m.pageRows() {
			if _, ok := docTime(d, c); ok {
				out = append(out, c)
				break
			}
		}
	}
	return out
}

// docTime parses a timestamp at field (plain or dotted) using the date layouts.
func docTime(d jsonldb.Doc, field string) (time.Time, bool) {
	var v any
	var ok bool
	if strings.Contains(field, ".") {
		v, ok = d.Path(field)
	} else {
		v, ok = d.Get(field)
	}
	s, isStr := v.(string)
	if !ok || !isStr {
		return time.Time{}, false
	}
	for _, l := range dateLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func (m *Model) renderChart(w, h int) string {
	if m.chartStep == 0 {
		return m.chartPicker(w, h, "Chart type", chartTypes)
	}
	if m.chartStep == 1 {
		title, items, _ := m.chartNextPrompt()
		return m.chartPicker(w, h, title, items)
	}
	// step 2 — render the chart full-screen
	chartW, chartH := w-4, h-4
	if chartW < 8 {
		chartW = 8
	}
	if chartH < 4 {
		chartH = 4
	}
	var body string
	switch m.chartType {
	case chartBar:
		body = m.buildBarChart(chartW, chartH)
	case chartLine:
		body = m.buildLineChart(chartW, chartH)
	case chartScatter:
		body = m.buildScatter(chartW, chartH)
	case chartSparkline:
		body = m.buildSparkline(chartW, chartH)
	case chartTimeSeries:
		body = m.buildTimeSeries(chartW, chartH)
	case chartHeatmap:
		body = m.buildHeatmap(chartW, chartH)
	case chartCalendar:
		body = m.buildCalendar(chartW, chartH)
	}
	title := paneTitle(true, "CHART · "+chartTypes[m.chartType]+" · "+strings.Join(m.chartPicks, " / "))
	box := pane(true).Width(w).Height(h - 1).MaxHeight(h - 1).Render(title + "\n" + body)
	footer := styleFooter.Width(w).Render(" " + keyHint("esc", "back") + keyHint("q", "quit"))
	return lipgloss.JoinVertical(lipgloss.Left, box, footer)
}

func (m *Model) chartPicker(w, h int, title string, items []string) string {
	var b strings.Builder
	b.WriteString(styleApp.Render(title) + styleMuted.Render("   ↑↓ select · ↵ next · esc back") + "\n")
	b.WriteString(gradientRule(42) + "\n")
	if len(items) == 0 {
		b.WriteString(styleMuted.Render("  (no matching columns)"))
		box := styleOverlay.Render(b.String())
		return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
	}
	listH := h - 10
	if listH < 3 {
		listH = 3
	}
	start := 0
	if m.chartCursor >= listH {
		start = m.chartCursor - listH + 1
	}
	for i := start; i < len(items) && i < start+listH; i++ {
		mark := "  "
		name := styleText.Render(items[i])
		if i == m.chartCursor {
			mark = styleGutter.Render("▌ ")
			name = styleSel.Render(" " + items[i] + " ")
		}
		b.WriteString(mark + name + "\n")
	}
	box := styleOverlay.Render(strings.TrimRight(b.String(), "\n"))
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}

// barMeasure computes a group's measure value from picks [cat, measure, valCol?].
func (m *Model) barMeasure(res *jsonldb.Result) float64 {
	measure, valCol := "count", ""
	if len(m.chartPicks) > 1 {
		measure = m.chartPicks[1]
	}
	if len(m.chartPicks) > 2 {
		valCol = m.chartPicks[2]
	}
	var v float64
	switch measure {
	case "sum":
		v, _ = res.Sum(valCol)
	case "avg":
		v, _ = res.Avg(valCol)
	case "min":
		v, _ = res.Min(valCol)
	case "max":
		v, _ = res.Max(valCol)
	default:
		v = float64(res.Count())
	}
	return v
}

func (m *Model) buildBarChart(w, h int) string {
	groups := m.result.GroupBy(m.chartPicks[0])
	type kv struct {
		k string
		v float64
	}
	rows := make([]kv, 0, len(groups))
	for k, res := range groups {
		rows = append(rows, kv{k, m.barMeasure(res)})
	}
	sort.Slice(rows, func(a, b int) bool {
		if rows[a].v != rows[b].v {
			return rows[a].v > rows[b].v
		}
		return rows[a].k < rows[b].k
	})
	if len(rows) > 12 {
		rows = rows[:12]
	}
	bc := barchart.New(w, h)
	data := make([]barchart.BarData, len(rows))
	for i, r := range rows {
		st := lipgloss.NewStyle().Foreground(barPalette[i%len(barPalette)])
		data[i] = barchart.BarData{
			Label:  clip(r.k, 10),
			Values: []barchart.BarValue{{Name: r.k, Value: r.v, Style: st}},
		}
	}
	bc.PushAll(data)
	bc.Draw()
	return bc.View()
}

func (m *Model) buildSparkline(w, h int) string {
	col := m.chartPicks[0]
	vals := m.numericValues(col, w*2)
	if len(vals) == 0 {
		return styleMuted.Render("‘" + col + "’ has no numeric values")
	}
	sl := sparkline.New(w, h, sparkline.WithStyle(lipgloss.NewStyle().Foreground(cCyan)))
	sl.PushAll(vals)
	sl.Draw()
	return sl.View()
}

func minMax(vals []float64) (float64, float64) {
	lo, hi := vals[0], vals[0]
	for _, v := range vals {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	if lo == hi {
		hi = lo + 1 // avoid a zero-height range
	}
	return lo, hi
}

func (m *Model) buildLineChart(w, h int) string {
	col := m.chartPicks[0]
	vals := m.numericValues(col, w)
	if len(vals) == 0 {
		return styleMuted.Render("‘" + col + "’ has no numeric values")
	}
	lo, hi := minMax(vals)
	wl := wavelinechart.New(w, h)
	wl.SetViewXYRange(0, float64(len(vals)-1), lo, hi)
	wl.SetStyles(runes.ArcLineStyle, lipgloss.NewStyle().Foreground(cCyan))
	for i, v := range vals {
		wl.Plot(canvas.Float64Point{X: float64(i), Y: v})
	}
	wl.Draw()
	return wl.View()
}

func (m *Model) buildScatter(w, h int) string {
	xc, yc := m.chartPicks[0], m.chartPicks[1]
	var xs, ys []float64
	for _, d := range m.result.Docs() {
		xv, ok1 := docFloat(d, xc)
		yv, ok2 := docFloat(d, yc)
		if ok1 && ok2 {
			xs = append(xs, xv)
			ys = append(ys, yv)
			if len(xs) >= 2000 {
				break
			}
		}
	}
	if len(xs) == 0 {
		return styleMuted.Render("no rows with both ‘" + xc + "’ and ‘" + yc + "’ numeric")
	}
	xlo, xhi := minMax(xs)
	ylo, yhi := minMax(ys)
	lc := linechart.New(w, h, xlo, xhi, ylo, yhi)
	lc.DrawXYAxisAndLabel()
	st := lipgloss.NewStyle().Foreground(cMagenta)
	for i := range xs {
		lc.DrawRuneWithStyle(canvas.Float64Point{X: xs[i], Y: ys[i]}, '•', st)
	}
	return lc.View()
}

func (m *Model) buildTimeSeries(w, h int) string {
	tc, yc := m.chartPicks[0], m.chartPicks[1]
	type tp struct {
		t time.Time
		v float64
	}
	var pts []tp
	for _, d := range m.result.Docs() {
		t, ok1 := docTime(d, tc)
		v, ok2 := docFloat(d, yc)
		if ok1 && ok2 {
			pts = append(pts, tp{t, v})
		}
	}
	if len(pts) == 0 {
		return styleMuted.Render("no rows with time ‘" + tc + "’ and numeric ‘" + yc + "’")
	}
	sort.Slice(pts, func(a, b int) bool { return pts[a].t.Before(pts[b].t) })
	ys := make([]float64, len(pts))
	for i, p := range pts {
		ys[i] = p.v
	}
	ylo, yhi := minMax(ys)
	ts := timeserieslinechart.New(w, h)
	ts.SetTimeRange(pts[0].t, pts[len(pts)-1].t)
	ts.SetViewTimeRange(pts[0].t, pts[len(pts)-1].t)
	ts.SetYRange(ylo, yhi)
	ts.SetViewYRange(ylo, yhi)
	ts.SetStyle(lipgloss.NewStyle().Foreground(cCyan))
	for _, p := range pts {
		ts.Push(timeserieslinechart.TimePoint{Time: p.t, Value: p.v})
	}
	ts.DrawBraille()
	return ts.View()
}

// cellText returns a column's value as a plain string (for cross-tab keys).
func (m *Model) cellText(d jsonldb.Doc, col string) string {
	var v any
	var ok bool
	if strings.Contains(col, ".") {
		v, ok = d.Path(col)
	} else {
		v, ok = d.Get(col)
	}
	if !ok {
		return ""
	}
	return scalarStr(v)
}

// distinctVals returns up to capN first-seen distinct values of col.
func (m *Model) distinctVals(col string, capN int) []string {
	if capN < 1 {
		capN = 1
	}
	seen := map[string]bool{}
	var out []string
	for _, d := range m.result.Docs() {
		v := m.cellText(d, col)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
		if len(out) >= capN {
			break
		}
	}
	return out
}

// buildHeatmap renders a labeled cross-tab of counts for two categorical
// columns, each cell shaded by count intensity (synthwave ramp).
func (m *Model) buildHeatmap(w, h int) string {
	xcol, ycol := m.chartPicks[0], m.chartPicks[1]
	const labelW, cellW = 12, 8
	xs := m.distinctVals(xcol, (w-labelW)/cellW)
	ys := m.distinctVals(ycol, h-2)
	if len(xs) == 0 || len(ys) == 0 {
		return styleMuted.Render("no data to cross-tab")
	}
	xi := map[string]int{}
	for i, v := range xs {
		xi[v] = i
	}
	yi := map[string]int{}
	for i, v := range ys {
		yi[v] = i
	}
	counts := make([][]int, len(ys))
	for i := range counts {
		counts[i] = make([]int, len(xs))
	}
	maxC := 1
	for _, d := range m.result.Docs() {
		a, okx := xi[m.cellText(d, xcol)]
		b, oky := yi[m.cellText(d, ycol)]
		if okx && oky {
			counts[b][a]++
			if counts[b][a] > maxC {
				maxC = counts[b][a]
			}
		}
	}
	ramp := lipgloss.Blend1D(12, cBg, cIdle, cViolet, cMagenta)

	var sb strings.Builder
	sb.WriteString(styleMuted.Render(cell(clip(ycol+"\\"+xcol, labelW-1), labelW)))
	for _, x := range xs {
		sb.WriteString(styleHeader.Render(cell(clip(x, cellW-1), cellW)))
	}
	sb.WriteString("\n")
	for r, y := range ys {
		sb.WriteString(styleText.Render(cell(clip(y, labelW-1), labelW)))
		for c := range xs {
			n := counts[r][c]
			bg := ramp[n*(len(ramp)-1)/maxC]
			txt := ""
			if n > 0 {
				txt = fmt.Sprintf("%d", n)
			}
			sb.WriteString(lipgloss.NewStyle().Background(bg).Foreground(cBright).
				Width(cellW).Align(lipgloss.Center).Render(txt))
		}
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

// displayText is the cell text as shown in the table: the cellValue glyph/
// string, then smart-formatted (durations, bytes, thousands, relative time)
// when m.pretty is on.
func (m *Model) displayText(d jsonldb.Doc, col string) string {
	txt, _ := cellValue(d, col)
	if !m.pretty {
		return txt
	}
	raw, ok := docRaw(d, col)
	if !ok {
		return txt
	}
	return prettyText(col, raw, txt)
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case float64:
		return x, true
	}
	return 0, false
}

// prettyText smart-formats a value, falling back to `fallback` when no special
// formatting applies.
func prettyText(col string, raw any, fallback string) string {
	switch x := raw.(type) {
	case json.Number, float64:
		f, _ := toFloat(raw)
		return formatNumberCol(col, f)
	case string:
		if isDateString(x) {
			return relTime(x)
		}
	}
	return fallback
}

func formatNumberCol(col string, f float64) string {
	lc := strings.ToLower(col)
	switch {
	case strings.HasSuffix(lc, "_ms") || strings.Contains(lc, "latency") || strings.Contains(lc, "millis"):
		return humanDur(f)
	case strings.HasSuffix(lc, "_sec") || strings.HasSuffix(lc, "_secs") || strings.HasSuffix(lc, "_seconds"):
		return humanDur(f * 1000)
	case strings.HasSuffix(lc, "_min") || strings.HasSuffix(lc, "_minutes"):
		return humanDur(f * 60000)
	case strings.Contains(lc, "bytes") || strings.HasSuffix(lc, "_size") || lc == "size":
		return humanBytes(f)
	default:
		return thousands(f)
	}
}

func humanDur(ms float64) string {
	switch {
	case ms < 1000:
		return fmt.Sprintf("%gms", math.Round(ms))
	case ms < 60000:
		return fmt.Sprintf("%.1fs", ms/1000)
	case ms < 3600000:
		return fmt.Sprintf("%dm%ds", int(ms)/60000, (int(ms)%60000)/1000)
	default:
		return fmt.Sprintf("%dh%dm", int(ms)/3600000, (int(ms)%3600000)/60000)
	}
}

func humanBytes(n float64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	for n >= 1024 && i < len(units)-1 {
		n /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%g B", n)
	}
	return fmt.Sprintf("%.1f %s", n, units[i])
}

// thousands groups the integer part with commas (keeps up to 2 decimals).
func thousands(f float64) string {
	neg := f < 0
	if neg {
		f = -f
	}
	whole := int64(f)
	frac := ""
	if f != math.Trunc(f) {
		frac = strings.TrimRight(fmt.Sprintf("%.2f", f-float64(whole)), "0")
		frac = strings.TrimPrefix(frac, "0") // ".25"
	}
	s := fmt.Sprintf("%d", whole)
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	res := string(out) + frac
	if neg {
		res = "-" + res
	}
	return res
}

// relTime renders a timestamp string as a relative age ("3h", "2d", "5mo").
func relTime(s string) string {
	var t time.Time
	for _, l := range dateLayouts {
		if pt, err := time.Parse(l, s); err == nil {
			t = pt
			break
		}
	}
	if t.IsZero() {
		return s
	}
	d := time.Since(t)
	future := d < 0
	if future {
		d = -d
	}
	var r string
	switch {
	case d < time.Minute:
		r = "just now"
	case d < time.Hour:
		r = fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		r = fmt.Sprintf("%dh", int(d.Hours()))
	case d < 30*24*time.Hour:
		r = fmt.Sprintf("%dd", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		r = fmt.Sprintf("%dmo", int(d.Hours()/24/30))
	default:
		r = fmt.Sprintf("%dy", int(d.Hours()/24/365))
	}
	if r == "just now" {
		return r
	}
	if future {
		return "in " + r
	}
	return r + " ago"
}

// docRaw returns a column's raw value (plain or dotted path).
func docRaw(d jsonldb.Doc, field string) (any, bool) {
	if strings.Contains(field, ".") {
		return d.Path(field)
	}
	return d.Get(field)
}

// dslLiteral renders a value as a DSL literal (typed for numbers/bools/null,
// quoted for strings). ok=false for containers, which can't be filtered.
func dslLiteral(v any) (string, bool) {
	switch x := v.(type) {
	case bool:
		if x {
			return "true", true
		}
		return "false", true
	case json.Number:
		return x.String(), true
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), true
	case string:
		return strconv.Quote(x), true
	case nil:
		return "null", true
	case map[string]any, []any:
		return "", false
	default:
		return strconv.Quote(fmt.Sprintf("%v", x)), true
	}
}

// buildCalendar renders a GitHub-style contribution grid: counts per day for a
// date column, laid out as weekday rows × week columns, shaded by intensity.
func (m *Model) buildCalendar(w, h int) string {
	col := m.chartPicks[0]
	counts := map[string]int{}
	var minD, maxD time.Time
	for _, d := range m.result.Docs() {
		t, ok := docTime(d, col)
		if !ok {
			continue
		}
		day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		counts[day.Format("2006-01-02")]++
		if minD.IsZero() || day.Before(minD) {
			minD = day
		}
		if day.After(maxD) {
			maxD = day
		}
	}
	if minD.IsZero() {
		return styleMuted.Render("no dates in ‘" + col + "’")
	}
	start := minD.AddDate(0, 0, -int(minD.Weekday())) // back to Sunday
	weeks := int(maxD.Sub(start).Hours()/24)/7 + 1
	if maxWeeks := (w - 5) / 2; maxWeeks >= 1 && weeks > maxWeeks {
		start = start.AddDate(0, 0, (weeks-maxWeeks)*7) // keep the most recent weeks
		weeks = maxWeeks
	}
	maxC := 1
	for _, c := range counts {
		if c > maxC {
			maxC = c
		}
	}
	ramp := lipgloss.Blend1D(8, cBg, cIdle, cViolet, cMagenta)
	labels := []string{"   ", "Mon", "   ", "Wed", "   ", "Fri", "   "}
	var b strings.Builder
	for wd := 0; wd < 7; wd++ {
		b.WriteString(styleMuted.Render(labels[wd]) + " ")
		for wk := 0; wk < weeks; wk++ {
			day := start.AddDate(0, 0, wk*7+wd)
			if day.After(maxD) || day.Before(minD) {
				b.WriteString("  ")
				continue
			}
			n := counts[day.Format("2006-01-02")]
			b.WriteString(lipgloss.NewStyle().Foreground(ramp[n*(len(ramp)-1)/maxC]).Render("██"))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n" + styleMuted.Render(fmt.Sprintf("%s → %s · max %d/day",
		minD.Format("2006-01-02"), maxD.Format("2006-01-02"), maxC)))
	return strings.TrimRight(b.String(), "\n")
}

func scalarStr(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", x)
	}
}

func (m *Model) renderFooter(w int) string {
	switch m.mode {
	case ModeFilter:
		s := styleKey.Render(" / ") + m.filterInput.View()
		if m.filterErr != nil {
			s += "  " + styleDanger.Render(m.filterErr.Error())
		}
		return styleFooter.Width(w).Render(s)
	case ModeFileSearch:
		s := styleKey.Render(" search ") + m.fileInput.View() +
			styleMuted.Render("   ↑↓ pick · ↵ open · esc clear")
		return styleFooter.Width(w).Render(s)
	case ModeJump:
		s := styleKey.Render(" jump to # ") + m.jumpInput.View() +
			styleMuted.Render(fmt.Sprintf("   1–%d · ↵ go · esc cancel", m.result.Count()))
		return styleFooter.Width(w).Render(s)
	case ModeConfirm:
		if d, ok := m.selectedDoc(); ok {
			s := " " + styleDanger.Render(fmt.Sprintf("delete record on line %d?", d.Line())) +
				"  " + styleKey.Render("y") + styleMuted.Render("es / ") + styleKey.Render("n") + styleMuted.Render("o")
			return styleFooter.Width(w).Render(s)
		}
	}
	m.help.Styles = helpStyles(cBar)
	m.help.SetWidth(w)
	left := " " + m.help.ShortHelpView(m.shortKeys()) // bar() truncates to fit
	right := ""
	if m.status != "" {
		st := styleOK
		if strings.Contains(m.status, "fail") || strings.Contains(m.status, "unavailable") {
			st = styleDanger
		}
		right += st.Render("● " + m.status + "  ")
	}
	right += lipgloss.NewStyle().Foreground(cViolet).Bold(true).Render(fmt.Sprintf("page %d/%d ", m.page, m.pageCount()))
	return bar(w, left, right)
}

var (
	reJSONKey = regexp.MustCompile(`^("(?:[^"\\]|\\.)*")\s*:\s*(.*)$`)
	reJSONNum = regexp.MustCompile(`^-?\d+(\.\d+)?([eE][+-]?\d+)?$`)
)

// detailContent pretty-prints a record with JSON syntax highlighting for the
// detail viewport (keys cyan, strings pink, dates orange, numbers cyan,
// bools green/red, null dim, punctuation dim).
func detailContent(d jsonldb.Doc) string {
	lines := strings.Split(prettyJSON(d.Raw()), "\n")
	for i, ln := range lines {
		lines[i] = colorizeJSONLine(ln)
	}
	return strings.Join(lines, "\n")
}

func colorizeJSONLine(ln string) string {
	trimmed := strings.TrimLeft(ln, " ")
	indent := ln[:len(ln)-len(trimmed)]
	if mm := reJSONKey.FindStringSubmatch(trimmed); mm != nil {
		return indent + styleJKey.Render(mm[1]) + styleJPunct.Render(": ") + colorizeJSONValue(mm[2])
	}
	return indent + colorizeJSONValue(trimmed)
}

func colorizeJSONValue(s string) string {
	comma := ""
	if strings.HasSuffix(s, ",") {
		comma = styleJPunct.Render(",")
		s = s[:len(s)-1]
	}
	switch {
	case s == "", s == "{", s == "}", s == "[", s == "]":
		return styleJPunct.Render(s) + comma
	case s == "true":
		return styleBoolTrue.Render(s) + comma
	case s == "false":
		return styleBoolFls.Render(s) + comma
	case s == "null":
		return styleNull.Render(s) + comma
	case strings.HasPrefix(s, `"`):
		st := styleJStr
		if isDateString(strings.Trim(s, `"`)) {
			st = styleDate
		}
		return st.Render(s) + comma
	case reJSONNum.MatchString(s):
		return styleNum.Render(s) + comma
	default:
		return styleText.Render(s) + comma
	}
}

func (m *Model) renderDetail(w, h int) string {
	inner := w - 2
	contentW := inner - 1
	if contentW < 2 {
		contentW = 2
	}
	vh := m.detailVP.Height()
	total := m.detailVP.TotalLineCount()
	off := m.detailVP.YOffset()
	sb := scrollbar(vh, total, vh, off)

	title := paneTitle(true, "RECORD")
	if total > vh {
		title += styleScrollHint.Render(fmt.Sprintf("   line %d–%d / %d",
			off+1, min(off+vh, total), total))
	}

	vpLines := strings.Split(m.detailVP.View(), "\n")
	lines := []string{padTo(title, contentW) + " "}
	for j := 0; j < vh; j++ {
		var content string
		if j < len(vpLines) {
			content = padTo(vpLines[j], contentW)
		} else {
			content = padTo("", contentW)
		}
		sc := " "
		if j < len(sb) {
			sc = sb[j]
		}
		lines = append(lines, content+sc)
	}
	box := pane(true).Width(w).Height(h - 1).MaxHeight(h - 1).Render(strings.Join(lines, "\n"))
	var hint string
	if m.mode == ModeDetailSearch {
		hint = styleKey.Render(" find ") + m.detailInput.View() +
			styleMuted.Render("   ↵ keep · esc clear")
	} else {
		hint = " " + keyHint("j/k", "scroll") + keyHint("/", "find") + keyHint("n/N", "next/prev") +
			keyHint("g/G", "top/end") + keyHint("esc", "back") + keyHint("q", "quit")
	}
	footer := styleFooter.Width(w).Render(hint)
	return lipgloss.JoinVertical(lipgloss.Left, box, footer)
}

func (m *Model) renderHelp(w, h int) string {
	m.help.ShowAll = true
	m.help.Styles = helpStyles(cBg)
	var b strings.Builder
	b.WriteString(styleApp.Render("lazyjsonl") + styleMuted.Render(" · keybindings") + "\n")
	b.WriteString(gradientRule(60) + "\n\n")
	b.WriteString(m.help.FullHelpView(fullGroups()))
	b.WriteString("\n\n" + styleMuted.Render("filter: ") +
		styleHeader.Render("done=true") + styleMuted.Render("  ") +
		styleHeader.Render("topic~=ml") + styleMuted.Render("  ") +
		styleHeader.Render("a=1 (b=2 |= c=3)") + styleMuted.Render("  ") +
		styleHeader.Render("!x  n>=5"))
	box := styleOverlay.Render(strings.TrimRight(b.String(), "\n"))
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}

func (m *Model) renderColumns(w, h int) string {
	var b strings.Builder
	b.WriteString(styleApp.Render("Columns") +
		styleMuted.Render("   space toggle · a all · N none · ↵ apply · esc cancel") + "\n")
	b.WriteString(gradientRule(58) + "\n")
	listH := h - 8
	if listH < 4 {
		listH = 4
	}
	start := 0
	if m.pickCursor >= listH {
		start = m.pickCursor - listH + 1
	}
	for i := start; i < len(m.pickList) && i < start+listH; i++ {
		r := m.pickList[i]
		box := styleMuted.Render("○")
		if m.picked[r.key] {
			box = styleOK.Render("✓")
		}
		mark := "  "
		if i == m.pickCursor {
			mark = styleGutter.Render("▌ ")
		}
		label := r.key
		if dp := m.drillPrefix(); dp != "" && strings.HasPrefix(label, dp+".") {
			label = strings.TrimPrefix(label, dp)
		}
		var name string
		if r.depth > 0 {
			name = "  " + styleMuted.Render("· ") + styleText.Render(label)
		} else {
			name = styleHeader.Render(label)
		}
		b.WriteString(mark + box + " " + name + "\n")
	}
	box := styleOverlay.Render(strings.TrimRight(b.String(), "\n"))
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}

func prettyJSON(raw []byte) string {
	var out bytes.Buffer
	if err := json.Indent(&out, raw, "", "  "); err != nil {
		return string(raw)
	}
	return out.String()
}
