package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
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
	files       []string // discovered .jsonl paths
	fileIdx     int
	focus       Focus
	col         *jsonldb.Collection
	schema      jsonldb.Schema
	result      *jsonldb.Result
	columns     []string // visible keys
	page        int      // 1-based
	pageSize    int
	cursor      int // selected row within page
	colCursor   int // selected column index (for sort)
	sortField   string
	sortDesc    bool
	filter      string
	filterSaved string // prior filter value before entering filter mode
	filterErr   error
	mode        Mode
	width       int
	height      int
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

func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) pageCount() int {
	n := m.result.Count()
	if n == 0 {
		return 1
	}
	return (n + m.pageSize - 1) / m.pageSize
}

func (m *Model) pageRows() []jsonldb.Doc {
	return m.result.Page(m.page, m.pageSize).Docs()
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		switch m.mode {
		case ModeList:
			return m.updateList(msg)
		case ModeFilter:
			return m.updateFilter(msg)
		case ModeConfirm:
			return m.updateConfirm(msg)
		}
	}
	return m, nil
}

func (m *Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rows := len(m.pageRows())
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "j", "down":
		if m.cursor < rows-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "l", "right":
		if m.page < m.pageCount() {
			m.page++
			m.cursor = 0
		}
	case "h", "left":
		if m.page > 1 {
			m.page--
			m.cursor = 0
		}
	case "g":
		m.page, m.cursor = 1, 0
	case "G":
		m.page, m.cursor = m.pageCount(), 0
	case "tab":
		if m.focus == FocusTable {
			m.focus = FocusFiles
		} else {
			m.focus = FocusTable
		}
	case "J":
		if m.fileIdx < len(m.files)-1 {
			m.fileIdx++
			m.openCurrent()
		}
	case "K":
		if m.fileIdx > 0 {
			m.fileIdx--
			m.openCurrent()
		}
	case "/":
		m.filterSaved = m.filter
		m.mode = ModeFilter
		return m, nil
	case "L":
		if m.colCursor < len(m.visibleColumns(len(m.columns)))-1 {
			m.colCursor++
		}
	case "H":
		if m.colCursor > 0 {
			m.colCursor--
		}
	case "s":
		cols := m.visibleColumns(len(m.columns))
		if m.colCursor < len(cols) {
			field := cols[m.colCursor]
			if m.sortField == field {
				m.sortDesc = !m.sortDesc
			} else {
				m.sortField = field
				m.sortDesc = false
			}
			m.result = m.result.SortBy(field, m.sortDesc)
			m.page, m.cursor = 1, 0
		}
	case "d":
		if _, ok := m.selectedDoc(); ok {
			m.mode = ModeConfirm
		}
	}
	return m, nil
}

func (m *Model) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		m.applyFilter()
		return m, nil
	case tea.KeyEsc:
		m.filter = m.filterSaved
		m.mode = ModeList
		return m, nil
	case tea.KeyBackspace:
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
		}
		return m, nil
	case tea.KeyRunes:
		m.filter += string(msg.Runes)
		return m, nil
	case tea.KeySpace:
		m.filter += " "
		return m, nil
	}
	return m, nil
}

func (m *Model) applyFilter() {
	res, err := m.col.Query(m.filter)
	if err != nil {
		m.filterErr = err
		return // keep prior result, stay in ModeFilter
	}
	m.filterErr = nil
	m.result = res
	m.page, m.cursor = 1, 0
	m.mode = ModeList
}

func (m *Model) selectedDoc() (jsonldb.Doc, bool) {
	rows := m.pageRows()
	if m.cursor < 0 || m.cursor >= len(rows) {
		return jsonldb.Doc{}, false
	}
	return rows[m.cursor], true
}

func (m *Model) refresh() {
	res, err := m.col.Query(m.filter)
	if err == nil {
		m.result = res
	}
	if m.page > m.pageCount() {
		m.page = m.pageCount()
	}
	if rows := len(m.pageRows()); m.cursor >= rows {
		m.cursor = rows - 1
		if m.cursor < 0 {
			m.cursor = 0
		}
	}
}

func (m *Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		if d, ok := m.selectedDoc(); ok {
			if err := m.col.DeleteAt(d.Line()); err == nil {
				m.refresh()
			}
		}
		m.mode = ModeList
	case "n", "esc":
		m.mode = ModeList
	}
	return m, nil
}

func (m *Model) View() string { return "" }
