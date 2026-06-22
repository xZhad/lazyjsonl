package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/xZhad/jsonldb"
)

type Mode int

const (
	ModeList Mode = iota
	ModeDetail
	ModeFilter
	ModeConfirm
)

type Focus int

const (
	FocusTable Focus = iota
	FocusFiles
)

type Model struct {
	files     []string // discovered .jsonl paths
	fileIdx   int
	focus     Focus
	col       *jsonldb.Collection
	schema    jsonldb.Schema
	result    *jsonldb.Result
	columns   []string // visible keys
	page      int      // 1-based
	pageSize  int
	cursor    int // selected row within page
	colCursor int // selected column index (for sort)
	sortDesc  bool
	filter    string
	filterErr error
	mode      Mode
	width     int
	height    int
}

// discoverFiles returns the .jsonl files for a directory path (sorted), or [path] for a file.
func discoverFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{path}, nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".jsonl" {
			files = append(files, filepath.Join(path, e.Name()))
		}
	}
	sort.Strings(files)
	return files, nil
}

// New accepts a file or directory and opens the first .jsonl file.
func New(path string) (*Model, error) {
	files, err := discoverFiles(path)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no .jsonl files found in %s", path)
	}
	m := &Model{
		files:    files,
		fileIdx:  0,
		focus:    FocusTable,
		page:     1,
		pageSize: 20,
		mode:     ModeList,
	}
	if len(files) > 1 {
		m.focus = FocusFiles
	}
	if err := m.openCurrent(); err != nil {
		return nil, err
	}
	return m, nil
}

// openCurrent opens files[fileIdx], rebuilding schema/result/columns.
func (m *Model) openCurrent() error {
	if m.col != nil {
		m.col.Close()
	}
	c, err := jsonldb.Open(m.files[m.fileIdx])
	if err != nil {
		return err
	}
	res, err := c.Query("") // empty DSL = match all
	if err != nil {
		c.Close()
		return err
	}
	m.col = c
	m.schema = c.Schema()
	m.result = res
	m.columns = defaultColumns(m.schema, 8)
	m.filter = ""
	m.filterErr = nil
	m.page, m.cursor, m.colCursor = 1, 0, 0
	return nil
}

func defaultColumns(s jsonldb.Schema, max int) []string {
	cols := make([]string, 0, max)
	for _, f := range s { // Schema is presence-sorted
		if len(cols) >= max {
			break
		}
		cols = append(cols, f.Key)
	}
	return cols
}

func (m *Model) visibleColumns(max int) []string {
	if max >= len(m.columns) {
		return m.columns
	}
	return m.columns[:max]
}
