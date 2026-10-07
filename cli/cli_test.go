package cli

import (
	"bytes"
	"fmt"
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

// missingSource is a source path that does not exist; jsonldb.Open would
// create it, so a rejected run must leave it absent.
func missingSource(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "typo.jsonl")
}

func TestRunCountWithOutRejected(t *testing.T) {
	src := missingSource(t)
	out := filepath.Join(filepath.Dir(src), "x.jsonl")
	var buf bytes.Buffer
	err := Run(Options{Path: src, Filter: "done=true", Count: true, Out: out}, &buf)
	if err == nil || err.Error() != "--count and --out cannot be combined" {
		t.Fatalf("got %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("stdout = %q, want empty", buf.String())
	}
	for _, p := range []string{src, out} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s must not be created, stat err = %v", p, err)
		}
	}
}

func TestRunOutputFormats(t *testing.T) {
	for _, f := range []string{"json", "jsonl"} {
		var buf bytes.Buffer
		if err := Run(Options{Path: fixture(t), Filter: "done=true", Output: f}, &buf); err != nil {
			t.Fatalf("--output %s: %v", f, err)
		}
		if n := strings.Count(buf.String(), "\n"); n != 2 {
			t.Errorf("--output %s: %d lines, want 2:\n%s", f, n, buf.String())
		}
	}
	for _, f := range []string{"csv", "table", "JSON "} {
		var buf bytes.Buffer
		err := Run(Options{Path: fixture(t), Output: f}, &buf)
		const want = "--output: only json/jsonl are supported (use --out FILE to export to a file)"
		if err == nil || err.Error() != want || buf.Len() != 0 {
			t.Errorf("--output %q: err %v, stdout %q", f, err, buf.String())
		}
	}
}

func TestRunOutMissingDirectory(t *testing.T) {
	src := fixture(t)
	dir := filepath.Dir(src)
	out := filepath.Join(dir, "nodir", "x.jsonl")
	err := Run(Options{Path: src, Out: out}, &bytes.Buffer{})
	want := fmt.Sprintf("--out: directory %q does not exist", filepath.Join(dir, "nodir"))
	if err == nil || err.Error() != want {
		t.Fatalf("got %v, want %q", err, want)
	}
	if strings.Contains(err.Error(), ".lazyjsonl-") {
		t.Errorf("error leaks the temp name: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "nodir")); !os.IsNotExist(err) {
		t.Errorf("directory must not be created, stat err = %v", err)
	}
	if ents, _ := os.ReadDir(dir); len(ents) != 1 {
		t.Errorf("dir should only hold the source file, got %v", ents)
	}
}

// The destination is checked before the source is opened (jsonldb.Open
// creates a missing source), so a bad --out must not create the source.
func TestRunBadOutDoesNotCreateSource(t *testing.T) {
	src := missingSource(t)
	var buf bytes.Buffer
	err := Run(Options{Path: src, Out: filepath.Join(filepath.Dir(src), "nodir", "x.jsonl")}, &buf)
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("got %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("stdout = %q, want empty", buf.String())
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("source must not be created, stat err = %v", err)
	}
}

func TestRunOutDirectoryPartIsAFile(t *testing.T) {
	src := fixture(t)
	err := Run(Options{Path: src, Out: filepath.Join(src, "x.jsonl")}, &bytes.Buffer{})
	if want := fmt.Sprintf("--out: %q is not a directory", src); err == nil || err.Error() != want {
		t.Fatalf("got %v, want %q", err, want)
	}
}

func TestRunOutIsAnExistingDirectory(t *testing.T) {
	src := fixture(t)
	sub := filepath.Join(filepath.Dir(src), "sub")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	err := Run(Options{Path: src, Out: sub}, &bytes.Buffer{})
	if want := fmt.Sprintf("--out: %q is a directory", sub); err == nil || err.Error() != want {
		t.Fatalf("got %v, want %q", err, want)
	}
	if ents, _ := os.ReadDir(sub); len(ents) != 0 {
		t.Errorf("sub should stay empty, got %v", ents)
	}
	if ents, _ := os.ReadDir(filepath.Dir(src)); len(ents) != 2 { // s.jsonl + sub, no temp file left
		t.Errorf("unexpected entries: %v", ents)
	}
}

func TestRunOutUnwritableDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write anywhere")
	}
	src := fixture(t)
	ro := filepath.Join(filepath.Dir(src), "ro")
	if err := os.Mkdir(ro, 0500); err != nil {
		t.Fatal(err)
	}
	err := Run(Options{Path: src, Out: filepath.Join(ro, "x.jsonl")}, &bytes.Buffer{})
	if want := fmt.Sprintf("--out: cannot write in directory %q: permission denied", ro); err == nil || err.Error() != want {
		t.Fatalf("got %v, want %q", err, want)
	}
}

func TestRunSourceIsADirectory(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	err := Run(Options{Path: dir, Count: true}, &buf)
	if want := dir + " is a directory: the CLI needs a .jsonl file (folders open in the TUI)"; err == nil || err.Error() != want {
		t.Fatalf("got %v, want %q", err, want)
	}
	if buf.Len() != 0 {
		t.Errorf("stdout = %q, want empty", buf.String())
	}
}
