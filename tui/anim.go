package tui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// tickMsg drives the title-bar gradient shimmer.
type tickMsg time.Time

// animFPS is gentle on purpose — a slow synthwave sweep, not a strobe.
const animFPS = 12

func tick() tea.Cmd {
	return tea.Tick(time.Second/animFPS, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// gradientText renders s with a synthwave color ramp that slides by `frame`,
// producing an animated shimmer across the wordmark.
func gradientText(s string, frame int) string {
	runes := []rune(s)
	n := len(runes)
	if n == 0 {
		return s
	}
	// A ramp wider than the text so the window can slide and loop seamlessly.
	steps := n * 3
	ramp := lipgloss.Blend1D(steps, cMagenta, cViolet, cCyan, cBright, cCyan, cViolet, cMagenta)
	var b strings.Builder
	for i, r := range runes {
		c := ramp[(i+frame)%len(ramp)]
		b.WriteString(lipgloss.NewStyle().Foreground(c).Bold(true).Render(string(r)))
	}
	return b.String()
}

// scrollbar returns `height` styled cells (top→bottom): a thumb whose size and
// position reflect viewing `visible` of `total` items at `offset`. When
// everything fits, the cells are blank (no bar).
func scrollbar(height, total, visible, offset int) []string {
	out := make([]string, height)
	if height <= 0 {
		return out
	}
	if total <= visible || visible <= 0 {
		for i := range out {
			out[i] = " "
		}
		return out
	}
	thumb := max(1, height*visible/total)
	if thumb > height {
		thumb = height
	}
	track := height - thumb
	maxOff := total - visible
	pos := 0
	if track > 0 && maxOff > 0 {
		pos = min(track, offset*track/maxOff)
	}
	for i := 0; i < height; i++ {
		if i >= pos && i < pos+thumb {
			out[i] = styleScrollThumb.Render("┃")
		} else {
			out[i] = styleScrollTrack.Render("│")
		}
	}
	return out
}

// padTo right-pads s with spaces to width w (ANSI-aware), so a trailing
// scrollbar column lands on the pane's right edge.
func padTo(s string, w int) string {
	diff := w - lipgloss.Width(s)
	if diff <= 0 {
		return s
	}
	return s + strings.Repeat(" ", diff)
}
