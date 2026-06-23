package tui

import "charm.land/lipgloss/v2"

// Palette — one signature accent (teal), everything else quiet.
var (
	colTeal   = lipgloss.Color("#2DD4BF") // active pane, titles, selection, accents
	colSlate  = lipgloss.Color("#475569") // idle pane borders
	colMuted  = lipgloss.Color("#94A3B8") // secondary text
	colBright = lipgloss.Color("#E2E8F0") // headers / primary text
	colDanger = lipgloss.Color("#F87171") // errors / destructive
	colSelBg  = lipgloss.Color("#134E4A") // selected-row background (deep teal)
)

var (
	styleApp     = lipgloss.NewStyle().Foreground(colTeal).Bold(true)
	stylePath    = lipgloss.NewStyle().Foreground(colMuted)
	styleHeader  = lipgloss.NewStyle().Foreground(colBright).Bold(true)
	styleHeadHi  = lipgloss.NewStyle().Foreground(colTeal).Bold(true)
	styleMuted   = lipgloss.NewStyle().Foreground(colMuted)
	styleKey     = lipgloss.NewStyle().Foreground(colTeal).Bold(true)
	styleDanger  = lipgloss.NewStyle().Foreground(colDanger).Bold(true)
	styleSel     = lipgloss.NewStyle().Foreground(colBright).Background(colSelBg)
	styleGutter  = lipgloss.NewStyle().Foreground(colTeal)
	styleFooter  = lipgloss.NewStyle().Foreground(colMuted)
	styleOverlay = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colTeal).Padding(1, 3)
)

// pane returns the box style for a pane, teal-bordered when focused, slate when idle.
func pane(active bool) lipgloss.Style {
	bc := colSlate
	if active {
		bc = colTeal
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(bc)
}

// paneTitle styles a pane's title line — teal when the pane is focused.
func paneTitle(active bool, s string) string {
	c := colMuted
	if active {
		c = colTeal
	}
	return lipgloss.NewStyle().Foreground(c).Bold(true).Render(s)
}

// keyHint renders "key desc" with the key in accent.
func keyHint(key, desc string) string {
	return styleKey.Render(key) + styleMuted.Render(" "+desc+"   ")
}

// bar lays left and right on a w-wide line (ANSI-aware spacing).
func bar(w int, left, right string) string {
	gap := w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + lipgloss.NewStyle().Width(gap).Render("") + right
}
