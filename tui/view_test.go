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
	out := m.View()
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
	out := m.View()
	if !strings.Contains(out, "filter:") {
		t.Errorf("filter view missing prompt:\n%s", out)
	}
	if !strings.Contains(strings.ToLower(out), "error") && m.filterErr != nil {
		// the error text should be surfaced somehow
		if !strings.Contains(out, m.filterErr.Error()) {
			t.Errorf("filter error not shown:\n%s", out)
		}
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
	out := m.View()
	if !strings.Contains(out, "FILES") {
		t.Errorf("view missing FILES pane:\n%s", out)
	}
	for _, want := range []string{"a.jsonl", "b.jsonl"} {
		if !strings.Contains(out, want) {
			t.Errorf("view missing file %q:\n%s", want, out)
		}
	}
}
