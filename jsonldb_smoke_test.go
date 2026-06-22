package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xZhad/jsonldb"
)

func TestJsonldbResolvable(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.jsonl")
	if err := os.WriteFile(p, []byte("{\"id\":\"a\"}\n{\"id\":\"b\"}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	c, err := jsonldb.Open(p)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer c.Close()
	if c.Count() != 2 {
		t.Fatalf("Count = %d, want 2", c.Count())
	}
}
