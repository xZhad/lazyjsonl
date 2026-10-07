package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/xZhad/lazyjsonl/cli"
	"github.com/xZhad/lazyjsonl/tui"
	"golang.org/x/term"
)

// errHelp is parseArgs' signal that usage was requested.
var errHelp = errors.New("help requested")

const usage = `lazyjsonl: inspect, filter and export JSONL files.

Usage: lazyjsonl [path] [flags]

Path (default "."; the CLI needs a single .jsonl file):
  data.jsonl   a single file
  logs/        a folder: every *.jsonl in it (TUI only)
  .            the current directory (TUI only)
  ./-name      a path starting with "-" must be written like this

Flags:
  --filter <dsl>   filter expression, e.g. 'completed=true topic~=pomo'
  --count          print the number of matches
  --out <file>     write matches to a file (atomic) instead of stdout
  --output <fmt>   json or jsonl (JSON lines either way)
  -h, --help       show this help

Mode: any of --filter, --count, --out or --output runs the CLI;
otherwise the TUI opens when stdout is a terminal and the CLI runs when it is piped.
`

func parseArgs(args []string) (cli.Options, bool, error) {
	var opts cli.Options
	explicitCLI := false
	i := 0
	for i < len(args) {
		a := args[i]
		switch a {
		case "-h", "--help":
			return opts, false, errHelp
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
			if strings.HasPrefix(a, "-") {
				return opts, false, fmt.Errorf("unknown flag: %s (see --help)", a)
			}
			if opts.Path != "" {
				return opts, false, fmt.Errorf("unexpected argument: %s", a)
			}
			opts.Path = a
			i++
		}
	}
	if opts.Path == "" {
		opts.Path = "." // no path given → current directory
	}
	runCLI := explicitCLI || !term.IsTerminal(int(os.Stdout.Fd()))
	return opts, runCLI, nil
}

func main() {
	opts, runCLI, err := parseArgs(os.Args[1:])
	if errors.Is(err, errHelp) {
		fmt.Print(usage)
		return
	}
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
