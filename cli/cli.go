package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/xZhad/jsonldb"
)

type Options struct {
	Path   string
	Filter string
	Count  bool
	Out    string // when set, Task 3 writes here instead of w
}

// Run opens the collection, applies the filter, and writes results to w.
func Run(opts Options, w io.Writer) error {
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
			return err
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
