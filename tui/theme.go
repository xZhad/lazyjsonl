package tui

import "charm.land/lipgloss/v2"

// Palette — synthwave, from the user's WezTerm theme.
var (
	cBg      = lipgloss.Color("#130A25") // deep purple background
	cBar     = lipgloss.Color("#1E1140") // title/footer bar background
	cFg      = lipgloss.Color("#F9CDF6") // soft pink foreground
	cBright  = lipgloss.Color("#F4EDFF") // bright text
	cViolet  = lipgloss.Color("#9658FF") // primary accent (active borders, app)
	cCyan    = lipgloss.Color("#54DDFF") // headers, keys
	cMagenta = lipgloss.Color("#FF40B9") // gutter / selection accent
	cYellow  = lipgloss.Color("#FFC102") // sort column, warnings
	cGreen   = lipgloss.Color("#56FF65") // success status
	cRed     = lipgloss.Color("#FF4146") // errors / destructive
	cSelBg   = lipgloss.Color("#80205D") // selected-row background (magenta)
	cMuted   = lipgloss.Color("#8A6FB8") // muted purple (secondary text)
	cIdle    = lipgloss.Color("#54368E") // idle pane border
)

var (
	styleApp     = lipgloss.NewStyle().Foreground(cViolet).Bold(true)
	stylePath    = lipgloss.NewStyle().Foreground(cMuted)
	styleCount   = lipgloss.NewStyle().Foreground(cCyan).Bold(true)
	styleHeader  = lipgloss.NewStyle().Foreground(cCyan).Bold(true)
	styleSortCol = lipgloss.NewStyle().Foreground(cYellow).Bold(true)
	styleMuted   = lipgloss.NewStyle().Foreground(cMuted)
	styleText    = lipgloss.NewStyle().Foreground(cFg)
	styleKey     = lipgloss.NewStyle().Foreground(cCyan).Bold(true)
	styleDanger  = lipgloss.NewStyle().Foreground(cRed).Bold(true)
	styleOK      = lipgloss.NewStyle().Foreground(cGreen).Bold(true)
	styleGutter  = lipgloss.NewStyle().Foreground(cMagenta).Bold(true)
	styleSel     = lipgloss.NewStyle().Foreground(cBright).Background(cSelBg)
	styleSelGut  = lipgloss.NewStyle().Foreground(cMagenta).Background(cSelBg).Bold(true)
	styleTitleBar = lipgloss.NewStyle().Background(cBar)
	styleFooter   = lipgloss.NewStyle().Background(cBar).Foreground(cMuted)
	styleCursor   = lipgloss.NewStyle().Background(cMagenta).Foreground(cBg) // block cursor
	styleOverlay  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cViolet).
			Background(cBg).Padding(1, 3)
	styleScrollThumb = lipgloss.NewStyle().Foreground(cViolet).Bold(true)
	styleScrollTrack = lipgloss.NewStyle().Foreground(cIdle)
	styleScrollHint  = lipgloss.NewStyle().Foreground(cMuted)
)

// pane returns a box style, violet-bordered when focused, dim-purple when idle.
func pane(active bool) lipgloss.Style {
	bc := cIdle
	if active {
		bc = cViolet
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(bc)
}

func paneTitle(active bool, s string) string {
	c := cMuted
	if active {
		c = cViolet
	}
	return lipgloss.NewStyle().Foreground(c).Bold(true).Render(s)
}

func keyHint(key, desc string) string {
	return styleKey.Render(key) + styleMuted.Render(" "+desc+"   ")
}

// bar lays left and right on a w-wide line (ANSI-aware spacing), over the bar bg.
func bar(w int, left, right string) string {
	gap := w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return styleTitleBar.Width(w).Render(left + lipgloss.NewStyle().Inline(true).Render(spaces(gap)) + right)
}

func spaces(n int) string {
	if n < 0 {
		n = 0
	}
	s := ""
	for i := 0; i < n; i++ {
		s += " "
	}
	return s
}
