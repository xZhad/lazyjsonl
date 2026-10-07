package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/xZhad/jsonldb"
)

type Options struct {
	Path   string
	Filter string
	Count  bool
	Out    string // when set, write here instead of w
	Output string // json or jsonl, accepted for CLI compatibility; JSON lines is the only format
}

// Run opens the collection, applies the filter, and writes results to w.
func Run(opts Options, w io.Writer) error {
	if opts.Output != "" && opts.Output != "json" && opts.Output != "jsonl" {
		return errors.New("--output: only json/jsonl are supported (use --out FILE to export to a file)")
	}
	if opts.Count && opts.Out != "" {
		return errors.New("--count and --out cannot be combined")
	}
	if opts.Out != "" {
		if err := checkDest(opts.Out); err != nil {
			return err
		}
	}
	if fi, err := os.Stat(opts.Path); err == nil && fi.IsDir() {
		return fmt.Errorf("%s is a directory: the CLI needs a .jsonl file (folders open in the TUI)", opts.Path)
	}
	c, err := jsonldb.Open(opts.Path)
	if err != nil {
		return err
	}
	defer c.Close()

	res, err := c.Query(opts.Filter)
	if err != nil {
		return fmt.Errorf("filter: %w", err)
	}

	if opts.Count {
		_, err := fmt.Fprintln(w, res.Count())
		return err
	}

	out := w
	var tmpName, finalName string
	if opts.Out != "" {
		f, err := os.CreateTemp(filepath.Dir(opts.Out), ".lazyjsonl-*.tmp")
		if err != nil {
			if pe, ok := errors.AsType[*fs.PathError](err); ok { // drop the hidden temp name
				err = pe.Err
			}
			return fmt.Errorf("--out: cannot write in directory %q: %w", filepath.Dir(opts.Out), err)
		}
		tmpName, finalName = f.Name(), opts.Out
		defer os.Remove(tmpName)
		defer f.Close()
		out = f
	}

	for _, d := range res.Docs() {
		if _, err := out.Write(d.Raw()); err != nil {
			return err
		}
		if _, err := out.Write([]byte{'\n'}); err != nil {
			return err
		}
	}

	if opts.Out != "" {
		if f, ok := out.(*os.File); ok {
			if err := f.Sync(); err != nil {
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		}
		return os.Rename(tmpName, finalName)
	}
	return nil
}

// checkDest rejects an --out whose directory is missing or not a directory, or
// whose path is an existing directory, before anything is opened or created.
func checkDest(out string) error {
	dir := filepath.Dir(out)
	switch fi, err := os.Stat(dir); {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("--out: directory %q does not exist", dir)
	case err == nil && !fi.IsDir():
		return fmt.Errorf("--out: %q is not a directory", dir)
	}
	if fi, err := os.Stat(out); err == nil && fi.IsDir() {
		return fmt.Errorf("--out: %q is a directory", out)
	}
	return nil
}
