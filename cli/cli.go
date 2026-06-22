package cli

import (
	"fmt"
	"io"

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

	for _, d := range res.Docs() {
		if _, err := w.Write(d.Raw()); err != nil {
			return err
		}
		if _, err := w.Write([]byte{'\n'}); err != nil {
			return err
		}
	}
	return nil
}
