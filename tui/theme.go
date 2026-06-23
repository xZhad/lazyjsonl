package tui

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

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
	cOrange  = lipgloss.Color("#FF9E64") // dates / timestamps
)

var (
	styleApp      = lipgloss.NewStyle().Foreground(cViolet).Bold(true)
	stylePath     = lipgloss.NewStyle().Foreground(cMuted)
	styleCount    = lipgloss.NewStyle().Foreground(cCyan).Bold(true)
	styleHeader   = lipgloss.NewStyle().Foreground(cCyan).Bold(true)
	styleSortCol  = lipgloss.NewStyle().Foreground(cYellow).Bold(true)
	styleMuted    = lipgloss.NewStyle().Foreground(cMuted)
	styleText     = lipgloss.NewStyle().Foreground(cFg)
	styleKey      = lipgloss.NewStyle().Foreground(cCyan).Bold(true)
	styleDanger   = lipgloss.NewStyle().Foreground(cRed).Bold(true)
	styleOK       = lipgloss.NewStyle().Foreground(cGreen).Bold(true)
	styleGutter   = lipgloss.NewStyle().Foreground(cMagenta).Bold(true)
	styleSel      = lipgloss.NewStyle().Foreground(cBright).Background(cSelBg)
	styleSelGut   = lipgloss.NewStyle().Foreground(cMagenta).Background(cSelBg).Bold(true)
	styleTitleBar = lipgloss.NewStyle().Background(cBar)
	styleFooter   = lipgloss.NewStyle().Background(cBar).Foreground(cMuted)
	styleOverlay  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cViolet).
			Background(cBg).Padding(1, 3)
	styleScrollThumb = lipgloss.NewStyle().Foreground(cViolet).Bold(true)
	styleScrollTrack = lipgloss.NewStyle().Foreground(cIdle)
	styleScrollHint  = lipgloss.NewStyle().Foreground(cMuted)

	// value coloring (JSON-ish syntax highlight in table cells)
	styleNum      = lipgloss.NewStyle().Foreground(cCyan)
	styleBoolTrue = lipgloss.NewStyle().Foreground(cGreen)
	styleBoolFls  = lipgloss.NewStyle().Foreground(cRed)
	styleNull     = lipgloss.NewStyle().Foreground(cMuted).Italic(true)
	styleObj      = lipgloss.NewStyle().Foreground(cYellow) // maps/arrays — hints "divable"
	styleDate     = lipgloss.NewStyle().Foreground(cOrange) // date/time-looking strings
)

// pane returns a box style: a thick violet border when focused, a dim rounded
// border when idle — border shape and color both signal focus.
func pane(active bool) lipgloss.Style {
	if active {
		return lipgloss.NewStyle().Border(lipgloss.ThickBorder()).BorderForeground(cViolet)
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cIdle)
}

// gradientRule draws a w-wide horizontal rule with a static synthwave ramp.
func gradientRule(w int) string {
	if w <= 0 {
		return ""
	}
	ramp := lipgloss.Blend1D(w, cMagenta, cViolet, cCyan)
	var b strings.Builder
	for i := 0; i < w; i++ {
		b.WriteString(lipgloss.NewStyle().Foreground(ramp[i]).Render("─"))
	}
	return b.String()
}

// helpStyles themes the bubbles help component to the synthwave palette.
func helpStyles() help.Styles {
	s := help.DefaultDarkStyles()
	s.ShortKey = lipgloss.NewStyle().Foreground(cCyan).Bold(true)
	s.ShortDesc = lipgloss.NewStyle().Foreground(cMuted)
	s.ShortSeparator = lipgloss.NewStyle().Foreground(cIdle)
	s.FullKey = lipgloss.NewStyle().Foreground(cCyan).Bold(true)
	s.FullDesc = lipgloss.NewStyle().Foreground(cFg)
	s.FullSeparator = lipgloss.NewStyle().Foreground(cIdle)
	s.Ellipsis = lipgloss.NewStyle().Foreground(cIdle)
	return s
}

// filterInputStyles themes the filter text input to the synthwave palette.
func filterInputStyles() textinput.Styles {
	s := textinput.DefaultDarkStyles()
	s.Focused.Text = lipgloss.NewStyle().Foreground(cFg)
	s.Focused.Prompt = lipgloss.NewStyle().Foreground(cCyan)
	s.Focused.Placeholder = lipgloss.NewStyle().Foreground(cIdle)
	s.Cursor.Color = cMagenta
	s.Cursor.Blink = true
	return s
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

// bar lays left and right on a w-wide line (ANSI-aware spacing), over the bar
// bg. The left side is truncated if needed so the bar never wraps.
func bar(w int, left, right string) string {
	maxLeft := w - lipgloss.Width(right) - 1
	if maxLeft > 0 && lipgloss.Width(left) > maxLeft {
		left = ansi.Truncate(left, maxLeft, "…")
	}
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
