package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func fixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "s.jsonl")
	body := `{"id":"a","topic":"ml","dur":1500,"done":true}
{"id":"b","topic":"go","dur":900,"done":false}
{"id":"c","topic":"ml","dur":1200,"done":true}
`
	if err := os.WriteFile(p, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestNewModel(t *testing.T) {
	m, err := New(fixture(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer m.col.Close()
	if m.mode != ModeList {
		t.Errorf("initial mode = %v, want ModeList", m.mode)
	}
	if m.result.Count() != 3 {
		t.Errorf("initial result count = %d, want 3", m.result.Count())
	}
	// columns: id/topic/dur/done all presence 1.0 -> sorted by key asc within ties
	cols := m.visibleColumns(10)
	if len(cols) != 4 {
		t.Errorf("columns = %v, want 4", cols)
	}
	// single file -> files has one entry
	if len(m.files) != 1 {
		t.Errorf("files = %v, want 1", m.files)
	}
}

func TestDefaultColumnsCap(t *testing.T) {
	m, _ := New(fixture(t))
	defer m.col.Close()
	if got := m.visibleColumns(2); len(got) != 2 {
		t.Errorf("visibleColumns(2) = %v, want 2", got)
	}
}

func TestNewFromDirectory(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"b.jsonl", "a.jsonl", "notjson.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("{\"x\":1}\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	m, err := New(dir)
	if err != nil {
		t.Fatalf("New(dir): %v", err)
	}
	defer m.col.Close()
	// only .jsonl files, sorted: a.jsonl, b.jsonl
	if len(m.files) != 2 {
		t.Fatalf("files = %v, want 2", m.files)
	}
	if filepath.Base(m.files[0]) != "a.jsonl" {
		t.Errorf("files[0] = %q, want a.jsonl (sorted)", m.files[0])
	}
}
