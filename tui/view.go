package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/xZhad/jsonldb"
)

// cell pads or truncates s to exactly w runes (used for plain measuring only).
func cell(s string, w int) string {
	if w <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) > w {
		if w == 1 {
			return "…"
		}
		return string(r[:w-1]) + "…"
	}
	return s + strings.Repeat(" ", w-len(r))
}

// clip truncates s to at most n runes with an ellipsis (no padding).
func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
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
			txt, _ := cellValue(d, c)
			txt = clip(txt, 32)
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

	tblLines := strings.Split(tbl.String(), "\n")
	sb := scrollbar(len(tblLines), m.result.Count(), m.pageSize, (m.page-1)*m.pageSize)
	body := make([]string, len(tblLines))
	for i, ln := range tblLines {
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
