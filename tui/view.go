package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
)

const colWidth = 16

func cell(s string, w int) string {
	r := []rune(s)
	if len(r) > w {
		if w <= 1 {
			return string(r[:w])
		}
		return string(r[:w-1]) + "…"
	}
	return s + strings.Repeat(" ", w-len(r))
}

func (m *Model) View() tea.View {
	var content string
	switch m.mode {
	case ModeDetail:
		content = m.viewDetail()
	default:
		content = m.viewList()
	}
	v := tea.NewView(content)
	v.AltScreen = true // full-screen (v2: alt-screen is a View field, not a program option)
	return v
}

func (m *Model) viewList() string {
	var b strings.Builder
	cols := m.activeColumns()

	// FILES pane (only when browsing multiple files)
	if len(m.files) > 1 {
		focusMark := " "
		if m.focus == FocusFiles {
			focusMark = "*"
		}
		b.WriteString("FILES" + focusMark + "\n")
		for i, f := range m.files {
			sel := "  "
			if i == m.fileIdx {
				sel = "> "
			}
			b.WriteString(sel + filepath.Base(f) + "\n")
		}
		b.WriteByte('\n')
	}

	// header
	for _, c := range cols {
		b.WriteString(cell(c, colWidth))
		b.WriteByte(' ')
	}
	b.WriteByte('\n')

	// rows
	rows := m.pageRows()
	for i, d := range rows {
		marker := "  "
		if i == m.cursor {
			marker = "> "
		}
		b.WriteString(marker)
		for _, c := range cols {
			v := d.GetString(c)
			if v == "" {
				if raw, ok := d.Get(c); ok {
					v = fmt.Sprintf("%v", raw)
				}
			}
			b.WriteString(cell(v, colWidth))
			b.WriteByte(' ')
		}
		b.WriteByte('\n')
	}

	// footer
	b.WriteByte('\n')
	b.WriteString(fmt.Sprintf("%d records  page %d/%d", m.result.Count(), m.page, m.pageCount()))
	if m.filter != "" {
		b.WriteString("  filter: " + m.filter)
	}
	if m.status != "" {
		b.WriteString("  [" + m.status + "]")
	}
	if m.showHelp {
		b.WriteString("\n\nKEYS: j/k move · h/l page · H/L column · s sort · / filter · enter detail · d delete · c columns · y yank · e export · r reload · g/G top/bottom · q quit")
	}

	// filter input / error line
	if m.mode == ModeFilter {
		b.WriteString("\nfilter: " + m.filter + "▌")
		if m.filterErr != nil {
			b.WriteString("\n  error: " + m.filterErr.Error())
		}
	}
	if m.mode == ModeConfirm {
		if d, ok := m.selectedDoc(); ok {
			b.WriteString(fmt.Sprintf("\ndelete record on line %d? (y/n)", d.Line()))
		}
	}
	return b.String()
}

func (m *Model) viewDetail() string {
	return string(m.detail.Raw()) + "\n\n(press any key to return)"
}
