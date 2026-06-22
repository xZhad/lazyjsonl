package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "s.jsonl")
	body := `{"id":"a","done":true}
{"id":"b","done":false}
{"id":"c","done":true}
`
	if err := os.WriteFile(p, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRunFilterJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := Run(Options{Path: fixture(t), Filter: "done=true"}, &buf); err != nil {
		t.Fatalf("Run: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2:\n%s", len(lines), buf.String())
	}
	if !strings.Contains(lines[0], `"id":"a"`) {
		t.Errorf("line0 = %q", lines[0])
	}
}

func TestRunCount(t *testing.T) {
	var buf bytes.Buffer
	if err := Run(Options{Path: fixture(t), Filter: "done=true", Count: true}, &buf); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "2" {
		t.Errorf("count output = %q, want 2", buf.String())
	}
}

func TestRunBadFilter(t *testing.T) {
	var buf bytes.Buffer
	if err := Run(Options{Path: fixture(t), Filter: "done="}, &buf); err == nil {
		t.Errorf("expected error for malformed filter")
	}
}

func TestRunExportToFile(t *testing.T) {
	src := fixture(t)
	out := filepath.Join(filepath.Dir(src), "done.jsonl")
	var buf bytes.Buffer
	if err := Run(Options{Path: src, Filter: "done=true", Out: out}, &buf); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("stdout should be empty when Out set, got %q", buf.String())
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read out: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(got)), "\n")
	if len(lines) != 2 {
		t.Errorf("out has %d lines, want 2", len(lines))
	}
}
