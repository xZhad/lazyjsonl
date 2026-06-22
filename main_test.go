package main

import "testing"

func TestParseArgsCLIMode(t *testing.T) {
	opts, runCLI, err := parseArgs([]string{"data.jsonl", "--filter", "done=true", "--count"})
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if !runCLI {
		t.Errorf("expected runCLI true when --filter/--count present")
	}
	if opts.Path != "data.jsonl" || opts.Filter != "done=true" || !opts.Count {
		t.Errorf("opts = %+v", opts)
	}
}

func TestParseArgsOut(t *testing.T) {
	opts, runCLI, err := parseArgs([]string{"data.jsonl", "--filter", "x=1", "--out", "o.jsonl"})
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if !runCLI || opts.Out != "o.jsonl" {
		t.Errorf("opts = %+v runCLI=%v", opts, runCLI)
	}
}

func TestParseArgsNoPath(t *testing.T) {
	if _, _, err := parseArgs([]string{"--count"}); err == nil {
		t.Errorf("expected error when no path given")
	}
}
