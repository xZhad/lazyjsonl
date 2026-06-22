package main

import (
	"errors"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/xZhad/lazyjsonl/cli"
	"github.com/xZhad/lazyjsonl/tui"
	"golang.org/x/term"
)

func parseArgs(args []string) (cli.Options, bool, error) {
	var opts cli.Options
	explicitCLI := false
	i := 0
	for i < len(args) {
		a := args[i]
		switch a {
		case "--filter":
			if i+1 >= len(args) {
				return opts, false, errors.New("--filter needs a value")
			}
			opts.Filter = args[i+1]
			explicitCLI = true
			i += 2
		case "--count":
			opts.Count = true
			explicitCLI = true
			i++
		case "--out":
			if i+1 >= len(args) {
				return opts, false, errors.New("--out needs a value")
			}
			opts.Out = args[i+1]
			explicitCLI = true
			i += 2
		case "--output":
			if i+1 >= len(args) {
				return opts, false, errors.New("--output needs a value")
			}
			opts.Output = args[i+1]
			explicitCLI = true
			i += 2
		default:
			if len(a) > 2 && a[:2] == "--" {
				return opts, false, fmt.Errorf("unknown flag: %s", a)
			}
			if opts.Path != "" {
				return opts, false, fmt.Errorf("unexpected argument: %s", a)
			}
			opts.Path = a
			i++
		}
	}
	if opts.Path == "" {
		return opts, false, errors.New("usage: lazyjsonl <file.jsonl|dir> [--filter DSL] [--count] [--out FILE] [--output FORMAT]")
	}
	runCLI := explicitCLI || !term.IsTerminal(int(os.Stdout.Fd()))
	return opts, runCLI, nil
}

func main() {
	opts, runCLI, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if runCLI {
		if err := cli.Run(opts, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	m, err := tui.New(opts.Path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer m.Close()
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
