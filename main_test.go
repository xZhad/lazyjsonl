package main

import (
	"strings"
	"testing"
)

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

func TestParseArgsOutputFlag(t *testing.T) {
	opts, runCLI, err := parseArgs([]string{"data.jsonl", "--output", "json"})
	if err != nil {
		t.Fatalf("parseArgs with --output json: %v", err)
	}
	if !runCLI {
		t.Errorf("expected runCLI true when --output present")
	}
	if opts.Output != "json" {
		t.Errorf("opts.Output = %q, want 'json'", opts.Output)
	}
	if opts.Path != "data.jsonl" {
		t.Errorf("opts.Path = %q, want 'data.jsonl'", opts.Path)
	}
}

func TestParseArgsUnknownFlag(t *testing.T) {
	_, _, err := parseArgs([]string{"data.jsonl", "--unknown"})
	if err == nil {
		t.Errorf("expected error for unknown flag")
	}
	if err != nil && !strings.Contains(err.Error(), "unknown flag") {
		t.Errorf("error should mention 'unknown flag', got: %v", err)
	}
}

func TestParseArgsExtraPositional(t *testing.T) {
	_, _, err := parseArgs([]string{"data.jsonl", "extra.jsonl"})
	if err == nil {
		t.Errorf("expected error for extra positional argument")
	}
	if err != nil && !strings.Contains(err.Error(), "unexpected argument") {
		t.Errorf("error should mention 'unexpected argument', got: %v", err)
	}
}
