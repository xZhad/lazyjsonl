package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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

func (m *Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
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
	if m.mode == ModeDetail {
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
	cur := m.files[m.fileIdx]
	if len(m.files) > 1 {
		return filepath.Base(filepath.Dir(cur)) + "/"
	}
	return filepath.Base(cur)
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
	start := 0
	if m.fileIdx >= listH {
		start = m.fileIdx - listH + 1
	}
	sb := scrollbar(listH, len(m.files), listH, start)
	lines := []string{padTo(paneTitle(active, "FILES"), contentW) + " "}
	for j := 0; j < listH; j++ {
		i := start + j
		var content string
		switch {
		case i < len(m.files):
			name := filepath.Base(m.files[i])
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
	if contentW < 2 {
		contentW = 2
	}

	cols := m.activeColumns()
	colW := 16
	if n := len(cols); n > 0 {
		colW = (contentW-2)/n - 1
		if colW < 10 {
			colW = 10
		}
		if colW > 22 {
			colW = 22
		}
	}
	if maxCols := (contentW - 2) / (colW + 1); maxCols >= 1 && maxCols < len(cols) {
		cols = cols[:maxCols]
	}

	// header (cell() truncates → no wrap; trailing space = column gap)
	header := "  "
	drillPrefix := strings.Join(m.drillCrumb, ".")
	for i, c := range cols {
		label := c
		// While dived, drop the drill prefix from headers: message.api → .api
		if drillPrefix != "" && strings.HasPrefix(c, drillPrefix+".") {
			label = strings.TrimPrefix(c, drillPrefix)
		}
		if c == m.sortField {
			if m.sortDesc {
				label += " ▼"
			} else {
				label += " ▲"
			}
		}
		st := styleHeader
		if i == m.colCursor && active {
			st = styleSortCol
		}
		header += st.Render(cell(label, colW) + " ")
	}
	titleStr := "RECORDS"
	if len(m.drillCrumb) > 0 {
		titleStr += " · " + strings.Join(m.drillCrumb, " ▸ ")
	}

	rows := m.pageRows()
	regionH := innerH - 2 // lines below title + header
	if regionH < 0 {
		regionH = 0
	}
	sb := scrollbar(regionH, m.result.Count(), m.pageSize, (m.page-1)*m.pageSize)

	lines := make([]string, 0, innerH)
	lines = append(lines, padTo(paneTitle(active, titleStr), contentW)+" ")
	lines = append(lines, padTo(header, contentW)+" ")
	for j := 0; j < regionH; j++ {
		var content string
		switch {
		case j < len(rows):
			if j == m.cursor {
				row := styleSelGut.Render("▌ ")
				for _, c := range cols {
					txt, _ := cellValue(rows[j], c)
					row += styleSel.Render(cell(txt, colW) + " ")
				}
				content = styleSel.Width(contentW).Render(row)
			} else {
				row := "  "
				for _, c := range cols {
					txt, st := cellValue(rows[j], c)
					row += st.Render(cell(txt, colW) + " ")
				}
				content = padTo(row, contentW)
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
	return pane(active).Width(w).Height(h).MaxHeight(h).Render(strings.Join(lines, "\n"))
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
		return "", styleText
	}
	return scalarStr(raw), valueStyle(raw)
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
	default:
		return styleText
	}
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
	case ModeConfirm:
		if d, ok := m.selectedDoc(); ok {
			s := " " + styleDanger.Render(fmt.Sprintf("delete record on line %d?", d.Line())) +
				"  " + styleKey.Render("y") + styleMuted.Render("es / ") + styleKey.Render("n") + styleMuted.Render("o")
			return styleFooter.Width(w).Render(s)
		}
	}
	left := " " + keyHint("j/k", "move") + keyHint("/", "filter") + keyHint("↵", "view") +
		keyHint("s", "sort") + keyHint("space", "dive") + keyHint("c", "cols") + keyHint("tab", "pane") + keyHint("?", "help") + keyHint("q", "quit")
	if len(m.drillCrumb) > 0 {
		left = " " + keyHint("⌫", "back") + keyHint("space", "dive") + keyHint("s", "sort") + keyHint("/", "filter") + keyHint("c", "cols") + keyHint("?", "help") + keyHint("q", "quit")
	}
	right := ""
	if m.status != "" {
		st := styleOK
		if strings.Contains(m.status, "fail") || strings.Contains(m.status, "unavailable") {
			st = styleDanger
		}
		right += st.Render("● "+m.status+"  ")
	}
	right += lipgloss.NewStyle().Foreground(cViolet).Bold(true).Render(fmt.Sprintf("page %d/%d ", m.page, m.pageCount()))
	return bar(w, left, right)
}

// detailContent pretty-prints a record for the detail viewport.
func detailContent(d jsonldb.Doc) string {
	return styleText.Render(prettyJSON(d.Raw()))
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
	hint := " " + keyHint("j/k", "scroll") + keyHint("d/u", "½ page") +
		keyHint("g/G", "top/end") + keyHint("esc", "back") + keyHint("q", "quit")
	footer := styleFooter.Width(w).Render(hint)
	return lipgloss.JoinVertical(lipgloss.Left, box, footer)
}

func (m *Model) renderHelp(w, h int) string {
	rows := [][2]string{
		{"j / k  ↑ ↓", "move cursor (in focused pane)"},
		{"h / l  ← →", "previous / next page"},
		{"g / G", "first / last page"},
		{"H / L  ⌥ ← →", "move column cursor"},
		{"s", "sort by column (toggles ▲/▼)"},
		{"J / K", "next / previous file"},
		{"tab", "switch focus (files ↔ table)"},
		{"space", "dive into focused object column"},
		{"backspace", "back out of a dive"},
		{"enter", "open record detail (j/k scrolls it)"},
		{"/", "filter (DSL, incl. nested: message.role=user)"},
		{"esc", "clear filter"},
		{"c", "choose columns (incl. nested fields)"},
		{"d", "delete record (confirm)"},
		{"e", "export view → .export.jsonl"},
		{"y", "yank record JSON to clipboard"},
		{"r", "reload from disk"},
		{"?", "toggle this help"},
		{"q / ctrl+c", "quit"},
	}
	var b strings.Builder
	b.WriteString(styleApp.Render("lazyjsonl") + styleMuted.Render(" · keybindings") + "\n")
	b.WriteString(gradientRule(52) + "\n")
	for _, kv := range rows {
		b.WriteString(styleKey.Render(cell(kv[0], 14)) + styleText.Render(kv[1]) + "\n")
	}
	b.WriteString("\n" + styleMuted.Render("filter: ") +
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
		if dp := strings.Join(m.drillCrumb, "."); dp != "" && strings.HasPrefix(label, dp+".") {
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
