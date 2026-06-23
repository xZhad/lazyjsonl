package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// cell pads or truncates s to exactly w runes.
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
	v.AltScreen = true // full-screen (v2: alt-screen is a View field)
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
	left := styleApp.Render(" lazyjsonl ") + stylePath.Render("· "+m.currentLabel())
	right := styleMuted.Render(fmt.Sprintf("%d records ", m.result.Count()))
	return bar(w, left, right)
}

func (m *Model) renderBody(w, h int) string {
	if len(m.files) <= 1 {
		return m.tablePane(w, h, m.focus == FocusTable)
	}
	filesW := 30
	if filesW > w/2 {
		filesW = w / 2
	}
	files := m.filesPane(filesW, h, m.focus == FocusFiles)
	table := m.tablePane(w-filesW, h, m.focus == FocusTable)
	return lipgloss.JoinHorizontal(lipgloss.Top, files, table)
}

// filesPane renders the left file list, scrolled to keep the selection in view.
func (m *Model) filesPane(w, h int, active bool) string {
	inner := w - 2
	innerH := h - 2
	listH := innerH - 1 // title line
	if listH < 1 {
		listH = 1
	}
	start := 0
	if m.fileIdx >= listH {
		start = m.fileIdx - listH + 1
	}
	lines := []string{paneTitle(active, "FILES")}
	for i := start; i < len(m.files) && i < start+listH; i++ {
		name := cell(filepath.Base(m.files[i]), inner-2)
		if i == m.fileIdx {
			lines = append(lines, styleSel.Width(inner).Render(styleGutter.Render("▌")+" "+name))
		} else {
			lines = append(lines, styleMuted.Render("  "+name))
		}
	}
	return pane(active).Width(inner).Height(innerH).Render(strings.Join(lines, "\n"))
}

// tablePane renders the record grid: header (with sort arrow + column cursor), rows.
func (m *Model) tablePane(w, h int, active bool) string {
	inner := w - 2
	innerH := h - 2
	cols := m.activeColumns()
	colW := 16
	if n := len(cols); n > 0 {
		colW = (inner-2)/n - 1 // each column renders colW + 1 trailing space
		if colW < 10 {
			colW = 10
		}
		if colW > 22 {
			colW = 22
		}
	}
	// show only as many columns as fit the pane width (no line wrap)
	if maxCols := (inner - 2) / (colW + 1); maxCols >= 1 && maxCols < len(cols) {
		cols = cols[:maxCols]
	}

	title := paneTitle(active, "RECORDS · "+filepath.Base(m.files[m.fileIdx]))

	// header
	var hdr strings.Builder
	hdr.WriteString("  ")
	for i, c := range cols {
		label := c
		if c == m.sortField {
			if m.sortDesc {
				label += " ▼"
			} else {
				label += " ▲"
			}
		}
		cs := cell(label, colW) + " "
		if i == m.colCursor && active {
			hdr.WriteString(styleHeadHi.Render(cs))
		} else {
			hdr.WriteString(styleHeader.Render(cs))
		}
	}

	lines := []string{title, hdr.String()}
	for i, d := range m.pageRows() {
		var rb strings.Builder
		for _, c := range cols {
			v := d.GetString(c)
			if v == "" {
				if raw, ok := d.Get(c); ok {
					v = fmt.Sprintf("%v", raw)
				}
			}
			rb.WriteString(cell(v, colW) + " ")
		}
		if i == m.cursor {
			lines = append(lines, styleSel.Width(inner).Render(styleGutter.Render("▌")+" "+rb.String()))
		} else {
			lines = append(lines, "  "+rb.String())
		}
	}
	return pane(active).Width(inner).Height(innerH).Render(strings.Join(lines, "\n"))
}

func (m *Model) renderFooter(w int) string {
	switch m.mode {
	case ModeFilter:
		s := " " + styleKey.Render("/") + " " + m.filter + styleKey.Render("▌")
		if m.filterErr != nil {
			s += "   " + styleDanger.Render(m.filterErr.Error())
		}
		return styleFooter.Width(w).Render(s)
	case ModeConfirm:
		if d, ok := m.selectedDoc(); ok {
			s := " " + styleDanger.Render(fmt.Sprintf("delete record on line %d?", d.Line())) +
				"  " + styleKey.Render("y") + styleMuted.Render("es / ") + styleKey.Render("n") + styleMuted.Render("o")
			return styleFooter.Width(w).Render(s)
		}
	}
	left := " " + keyHint("j/k", "move") + keyHint("h/l", "page") + keyHint("/", "filter") +
		keyHint("↵", "view") + keyHint("s", "sort") + keyHint("tab", "pane") + keyHint("?", "help") + keyHint("q", "quit")
	right := styleMuted.Render(fmt.Sprintf("page %d/%d ", m.page, m.pageCount()))
	if m.status != "" {
		right = styleKey.Render("● ") + styleMuted.Render(m.status+"  ") + right
	}
	return bar(w, left, right)
}

func (m *Model) renderDetail(w, h int) string {
	title := paneTitle(true, "RECORD")
	body := prettyJSON(m.detail.Raw())
	box := pane(true).Width(w - 2).Height(h - 3).Render(title + "\n" + body)
	footer := styleFooter.Width(w).Render(" " + keyHint("esc", "back") + keyHint("q", "quit"))
	return lipgloss.JoinVertical(lipgloss.Left, box, footer)
}

func (m *Model) renderHelp(w, h int) string {
	rows := [][2]string{
		{"j / k  ↑ ↓", "move row cursor"},
		{"h / l  ← →", "previous / next page"},
		{"g / G", "first / last page"},
		{"H / L", "move column cursor"},
		{"s", "sort by column (toggles ▲/▼)"},
		{"J / K", "next / previous file"},
		{"tab", "switch focus (files ↔ table)"},
		{"enter", "open record detail"},
		{"/", "filter (jsonldb query DSL)"},
		{"c", "toggle all columns"},
		{"d", "delete record (confirm)"},
		{"e", "export current view → .export.jsonl"},
		{"y", "yank record JSON to clipboard"},
		{"r", "reload from disk"},
		{"?", "toggle this help"},
		{"q / ctrl+c", "quit"},
	}
	var b strings.Builder
	b.WriteString(styleApp.Render("lazyjsonl") + styleMuted.Render(" · keybindings") + "\n\n")
	for _, kv := range rows {
		b.WriteString(styleKey.Render(cell(kv[0], 14)) + styleMuted.Render(kv[1]) + "\n")
	}
	b.WriteString("\n" + styleMuted.Render("filter examples: ") + styleHeader.Render("done=true   topic~=ml   a=1 (b=2 |= c=3)   !x   n>=5"))
	box := styleOverlay.Render(strings.TrimRight(b.String(), "\n"))
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}

// prettyJSON indents a raw JSON line for the detail view.
func prettyJSON(raw []byte) string {
	var out bytes.Buffer
	if err := json.Indent(&out, raw, "", "  "); err != nil {
		return string(raw)
	}
	return out.String()
}
