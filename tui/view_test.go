package tui

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestViewListRendersColumnsAndFooter(t *testing.T) {
	m, _ := New(fixture(t))
	defer m.col.Close()
	m.width, m.height = 80, 24
	out := m.View().Content
	for _, want := range []string{"id", "topic", "dur"} {
		if !strings.Contains(out, want) {
			t.Errorf("List view missing column %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "3 records") {
		t.Errorf("footer missing record count:\n%s", out)
	}
}

func TestViewFilterShowsError(t *testing.T) {
	m, _ := New(fixture(t))
	defer m.col.Close()
	m.width, m.height = 80, 24
	m.mode = ModeFilter
	m.filter = "done="
	m.applyFilter() // sets filterErr, stays ModeFilter
	out := m.View().Content
	// footer shows the in-progress filter text being edited
	if !strings.Contains(out, "done=") {
		t.Errorf("filter view missing the filter text:\n%s", out)
	}
	// and surfaces the parse error inline
	if m.filterErr != nil && !strings.Contains(out, m.filterErr.Error()) {
		t.Errorf("filter error not shown:\n%s", out)
	}
}

func TestCellPadTruncate(t *testing.T) {
	if got := cell("hello", 3); len([]rune(got)) != 3 {
		t.Errorf("truncate width = %d, want 3", len([]rune(got)))
	}
	if got := cell("hi", 5); len([]rune(got)) != 5 {
		t.Errorf("pad width = %d, want 5", len([]rune(got)))
	}
}

func TestViewShowsFilesPane(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.jsonl", "b.jsonl"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("{\"id\":\"x\"}\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	m, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer m.col.Close()
	m.width, m.height = 80, 24
	out := m.View().Content
	if !strings.Contains(out, "FILES") {
		t.Errorf("view missing FILES pane:\n%s", out)
	}
	for _, want := range []string{"a.jsonl", "b.jsonl"} {
		if !strings.Contains(out, want) {
			t.Errorf("view missing file %q:\n%s", want, out)
		}
	}
}

func TestViewAltScreen(t *testing.T) {
	m, err := New(fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	defer m.col.Close()
	if !m.View().AltScreen {
		t.Error("View().AltScreen must be true (full-screen TUI)")
	}
}

var prettyNumberCases = []struct{ in, want string }{
	// carry across the rounding digit (the old float path printed "11.", "21.", "9991.")
	{"1.999", "2"},
	{"2.996", "3"},
	{"999.995", "1,000"},
	{"0.005", "0.01"},
	{"0.994", "0.99"},
	{"0.995", "1"},
	{"-1.005", "-1.01"}, // half-up on the magnitude
	{"123456.789", "123,456.79"},
	{"12345678901234567890.125", "12,345,678,901,234,567,890.13"}, // exact, beyond float64
	{"999999999999999.999", "1,000,000,000,000,000"},
	// trailing zeros trimmed
	{"250.0", "250"},
	{"100.5", "100.5"},
	{"1234.5", "1,234.5"},
	{"0.0", "0"},
	{"-0.0", "0"},
	{"0", "0"},
	{"1000000000000000.0", "1,000,000,000,000,000"},
	// integers beyond 2^53 / 2^63 keep every digit
	{"1234567890123456789", "1,234,567,890,123,456,789"},
	{"9007199254740993", "9,007,199,254,740,993"},
	{"-9007199254740993", "-9,007,199,254,740,993"},
	{"9223372036854775807", "9,223,372,036,854,775,807"},
	{"-1000", "-1,000"},
	{"999", "999"},
	// non-zero values that round to 0.00 keep three significant digits
	{"0.001", "0.001"},
	{"0.004", "0.004"},
	{"-0.001", "-0.001"},
	{"0.0012345", "0.00123"},
	{"0.0000001", "1e-07"},
	// exponent forms: plain decimals inside [1e-4, 1e15), 4 significant digits outside
	{"1e-7", "1e-07"},
	{"1e-5", "1e-05"},
	{"1e-4", "0.0001"},
	{"1.5e3", "1,500"},
	{"1E3", "1,000"},
	{"1.2345e14", "123,450,000,000,000"},
	{"1e15", "1e+15"},
	{"1e22", "1e+22"},
	{"-1.23456e30", "-1.235e+30"},
	{"0e0", "0"},
	{"-0", "0"}, // no negative zero
	{"-0e5", "0"},
	{"-0.0e3", "0"},
	{"1e-999", "1e-999"}, // underflows float64 but is not zero: show the text
	{"-1e-999", "-1e-999"},
	// odd input falls back to the original text and never panics
	{"", ""},
	{"-", "-"},
	{"+5", "+5"},
	{"007", "007"},
	{"1.", "1."},
	{".5", ".5"},
	{"0x10", "0x10"},
	{"NaN", "NaN"},
	{"Infinity", "Infinity"},
	{"-Infinity", "-Infinity"},
	{"1e999", "1e999"},
}

func TestPrettyNumbers(t *testing.T) {
	for _, c := range prettyNumberCases {
		if got := prettyText("amount", json.Number(c.in), "fallback"); got != c.want {
			t.Errorf("pretty(amount, %q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// float64 values (not json.Number) take the same exact path via FormatFloat('f', -1).
func TestPrettyFloat64Values(t *testing.T) {
	for in, want := range map[float64]string{
		1.999: "2", 0.001: "0.001", 123456.789: "123,456.79", 250: "250", 0: "0", 1e22: "10,000,000,000,000,000,000,000",
		0.1 + 0.2: "0.3",
	} {
		if got := formatNumberCol("amount", in); got != want {
			t.Errorf("formatNumberCol(amount, %v) = %q, want %q", in, got, want)
		}
	}
}

// Name-guess formatters keep priority over the exact path and work from json.Number.
func TestNameGuessFormattersWithJSONNumber(t *testing.T) {
	for _, c := range []struct{ col, in, want string }{
		{"latency_ms", "1500", "1.5s"},
		{"latency_ms", "250", "250ms"},
		{"wait_sec", "90", "1m30s"},
		{"duration_min", "90", "1h30m"},
		{"size_bytes", "2048", "2.0 KB"},
		{"size", "1048576", "1.0 MB"},
		{"amount", "1466.666", "1,466.67"},
	} {
		if got := formatNumberCol(c.col, json.Number(c.in)); got != c.want {
			t.Errorf("formatNumberCol(%q, %s) = %q, want %q", c.col, c.in, got, c.want)
		}
	}
}

func TestNumbersRenderExactlyInTable(t *testing.T) {
	var body strings.Builder
	cases := []struct{ in, pretty string }{}
	for _, c := range prettyNumberCases {
		switch c.in {
		case "1.999", "2.996", "999.995", "0.001", "1e-7", "1e22", "1e15", "123456.789", "-1.005", "250.0", "100.5", "0", "0.0", "-0",
			"1234567890123456789", "9007199254740993", "-9007199254740993", "9223372036854775807":
			cases = append(cases, struct{ in, pretty string }{c.in, c.want})
		}
	}
	for _, c := range cases {
		body.WriteString(`{"n":` + c.in + "}\n")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "n.jsonl"), []byte(body.String()), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer m.col.Close()
	m = send(m, tea.WindowSizeMsg{Width: 100, Height: 40})
	// the table row text, via the same displayText the table cell uses
	for i, d := range m.pageRows() {
		if got := m.displayText(d, "n"); got != cases[i].pretty {
			t.Errorf("pretty %s = %q, want %q", cases[i].in, got, cases[i].pretty)
		}
	}
	out := ansi.Strip(m.View().Content)
	for _, want := range []string{"1,234,567,890,123,456,789", "9,007,199,254,740,993", "-9,007,199,254,740,993", "9,223,372,036,854,775,807", "123,456.79", "-1.01", "1,000"} {
		if !strings.Contains(out, want) {
			t.Errorf("pretty on: %q missing from the table:\n%s", want, out)
		}
	}
	m = send(m, kp('#'))
	out = ansi.Strip(m.View().Content)
	for _, want := range []string{"1234567890123456789", "9007199254740993", "-9007199254740993", "9223372036854775807", "123456.789", "-1.005", "1.999"} {
		if !strings.Contains(out, want) {
			t.Errorf("pretty off: raw %q missing from the table:\n%s", want, out)
		}
	}
	if strings.Contains(out, "9,007") {
		t.Errorf("pretty off: grouped digits leaked:\n%s", out)
	}
}

// Stats min/max/sum/mean/median go through the exact formatter, not a float cast.
func TestStatsUseExactFormatter(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "s.jsonl"), []byte(`{"n":0.001}`+"\n"+`{"n":1999.996}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer m.col.Close()
	m.openStats("n")
	out := ansi.Strip(m.renderStats(60, 20))
	for _, want := range []string{"0.001", "2,000"} { // min keeps its digits; max carries into the integer part
		if !strings.Contains(out, want) {
			t.Errorf("stats missing %q:\n%s", want, out)
		}
	}
}

// The stats popup and the table cell must agree on very large values.
func TestStatsAgreeWithTableOnHugeValues(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "s.jsonl"), []byte(`{"n":1e22}`+"\n"+`{"n":2e22}`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer m.col.Close()
	if cell := m.displayText(m.pageRows()[0], "n"); cell != "1e+22" {
		t.Fatalf("table cell = %q, want 1e+22", cell)
	}
	m.openStats("n")
	out := ansi.Strip(m.renderStats(60, 20))
	for _, want := range []string{"1e+22", "2e+22", "3e+22"} { // min, max, sum
		if !strings.Contains(out, want) {
			t.Errorf("stats missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "10,000,000,000,000,000,000,000") {
		t.Errorf("stats printed a digit string where the table prints an exponent:\n%s", out)
	}
}

// footerModel opens a two-file folder at w×h (focus on the file list).
func footerModel(t *testing.T, w, h int) *Model {
	t.Helper()
	dir := t.TempDir()
	for _, n := range []string{"a.jsonl", "b.jsonl"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("{\"id\":\"r1\",\"topic\":\"ml\"}\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	m, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Close() })
	return send(m, tea.WindowSizeMsg{Width: w, Height: h})
}

// Every prompt footer is exactly one line at any width: long input scrolls
// inside the text input, hints and errors are clipped. Otherwise the footer
// wraps, the frame outgrows the terminal and the rows/cursor scroll away.
func TestFooterPromptsStayOneLine(t *testing.T) {
	long := strings.Repeat("x", 2500) + "END"
	enter := tea.KeyPressMsg{Code: tea.KeyEnter}
	prompts := map[string]func(*Model) *Model{
		"filter": func(m *Model) *Model {
			m = send(m, tea.KeyPressMsg{Code: tea.KeyTab})
			m = send(m, kp('/'))
			m.filterInput.SetValue("topic~=" + long)
			return m
		},
		"filter error": func(m *Model) *Model {
			m = send(m, tea.KeyPressMsg{Code: tea.KeyTab})
			m = send(m, kp('/'))
			m.filterErr = errors.New(strings.Repeat("e", 300))
			return m
		},
		"filter error + long input": func(m *Model) *Model {
			m = send(m, tea.KeyPressMsg{Code: tea.KeyTab})
			m = send(m, kp('/'))
			m.filterInput.SetValue("topic~=" + long)
			m.filterInput.SetCursor(200) // mid-text, not at the end
			m.View()                     // drawn wide first; the error then narrows the input
			m.filterErr = errors.New(strings.Repeat("e", 300))
			return m
		},
		"file search": func(m *Model) *Model {
			m = send(m, kp('/'))
			m.fileInput.SetValue(long)
			return m
		},
		"jump": func(m *Model) *Model {
			m = send(m, tea.KeyPressMsg{Code: tea.KeyTab})
			m = send(m, kp(':'))
			m.jumpInput.SetValue(long)
			return m
		},
		"detail find": func(m *Model) *Model {
			m = send(m, tea.KeyPressMsg{Code: tea.KeyTab})
			m = send(m, enter)
			m = send(m, kp('/'))
			m.detailInput.SetValue(long)
			return m
		},
	}
	wantMode := map[string]Mode{
		"filter": ModeFilter, "filter error": ModeFilter, "filter error + long input": ModeFilter,
		"file search": ModeFileSearch, "jump": ModeJump, "detail find": ModeDetailSearch,
	}
	for name, setup := range prompts {
		for _, sz := range [][2]int{{80, 24}, {40, 24}, {40, 10}, {80, 10}} {
			m := setup(footerModel(t, sz[0], sz[1]))
			if want := wantMode[name]; m.mode != want {
				t.Fatalf("%s: setup: mode = %v, want %v", name, m.mode, want)
			}
			lines := strings.Split(ansi.Strip(m.View().Content), "\n")
			if len(lines) != sz[1] {
				t.Errorf("%s at %dx%d: frame has %d lines, want %d", name, sz[0], sz[1], len(lines), sz[1])
				continue
			}
			for _, l := range lines {
				if w := ansi.StringWidth(l); w > sz[0] {
					t.Errorf("%s at %dx%d: line is %d wide: %q", name, sz[0], sz[1], w, l)
				}
			}
			foot := strings.TrimRight(lines[len(lines)-1], " ")
			if strings.HasPrefix(name, "filter error") {
				if !strings.HasSuffix(foot, "…") {
					t.Errorf("%s at %dx%d: long error not clipped to the footer: %q", name, sz[0], sz[1], foot)
				}
			} else if !strings.Contains(foot, "END") {
				t.Errorf("%s at %dx%d: the end of the text (cursor) is not visible: %q", name, sz[0], sz[1], foot)
			}
			if !strings.Contains(strings.Join(lines, "\n"), "r1") {
				t.Errorf("%s at %dx%d: rows pushed out:\n%s", name, sz[0], sz[1], strings.Join(lines, "\n"))
			}
		}
	}
}

// At normal widths the prompt footers keep their full hint text.
func TestFooterPromptHintsAtNormalWidth(t *testing.T) {
	m := footerModel(t, 100, 24)
	m = send(m, kp('/'))
	if f := ansi.Strip(m.renderFooter(100)); !strings.Contains(f, "search") || !strings.Contains(f, "↑↓ pick · ↵ open · esc clear") {
		t.Errorf("file search footer: %q", f)
	}
	m = send(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	m = send(m, kp(':'))
	if f := ansi.Strip(m.renderFooter(100)); !strings.Contains(f, "jump to #") || !strings.Contains(f, "1–1 · ↵ go · esc cancel") {
		t.Errorf("jump footer: %q", f)
	}
	m = send(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	m = send(m, tea.KeyPressMsg{Code: tea.KeyTab})
	m = send(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m = send(m, kp('/'))
	if f := ansi.Strip(m.View().Content); !strings.Contains(f, "find") || !strings.Contains(f, "↵ keep · esc clear") {
		t.Errorf("detail find footer: %q", f)
	}
}

// The delete-confirm footer is one line even on the narrowest terminal.
func TestFooterConfirmStaysOneLine(t *testing.T) {
	for _, w := range []int{24, 30, 40} {
		m := footerModel(t, w, 12)
		m = send(m, tea.KeyPressMsg{Code: tea.KeyTab})
		m = send(m, kp('d'))
		if m.mode != ModeConfirm {
			t.Fatalf("mode = %v, want ModeConfirm", m.mode)
		}
		if h := lipgloss.Height(m.renderFooter(w)); h != 1 {
			t.Errorf("width %d: confirm footer is %d lines", w, h)
		}
		if n := len(strings.Split(ansi.Strip(m.View().Content), "\n")); n != 12 {
			t.Errorf("width %d: frame has %d lines, want 12", w, n)
		}
	}
}

// The normal-mode footers (file/table list and the record detail view) are
// one line at every supported size too, so toggling a prompt never makes the
// frame jump.
func TestFooterNormalModesStayOneLine(t *testing.T) {
	tab := tea.KeyPressMsg{Code: tea.KeyTab}
	views := map[string]func(*Model) *Model{
		"list (files focus)": func(m *Model) *Model { return m },
		"list (table focus)": func(m *Model) *Model { return send(m, tab) },
		"detail": func(m *Model) *Model {
			m = send(m, tab)
			return send(m, tea.KeyPressMsg{Code: tea.KeyEnter})
		},
	}
	wantMode := map[string]Mode{"list (files focus)": ModeList, "list (table focus)": ModeList, "detail": ModeDetail}
	for name, setup := range views {
		for _, sz := range [][2]int{{80, 24}, {40, 10}, {30, 10}, {24, 8}} {
			m := setup(footerModel(t, sz[0], sz[1]))
			if m.mode != wantMode[name] {
				t.Fatalf("%s: setup: mode = %v, want %v", name, m.mode, wantMode[name])
			}
			lines := strings.Split(ansi.Strip(m.View().Content), "\n")
			if len(lines) != sz[1] {
				t.Errorf("%s at %dx%d: frame has %d lines, want %d", name, sz[0], sz[1], len(lines), sz[1])
				continue
			}
			for _, l := range lines {
				if w := ansi.StringWidth(l); w > sz[0] {
					t.Errorf("%s at %dx%d: line is %d wide: %q", name, sz[0], sz[1], w, l)
				}
			}
			if name != "detail" {
				if h := lipgloss.Height(m.renderFooter(sz[0])); h != 1 {
					t.Errorf("%s at %dx%d: footer is %d lines", name, sz[0], sz[1], h)
				}
			}
		}
	}
}
