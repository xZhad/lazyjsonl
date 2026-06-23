package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
