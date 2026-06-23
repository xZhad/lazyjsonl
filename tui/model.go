package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/xZhad/jsonldb"
)

type Mode int

const (
	ModeList Mode = iota
	ModeDetail
	ModeFilter
	ModeConfirm
	ModeColumns
)

// pickRow is one candidate column in the column picker (top-level or nested).
type pickRow struct {
	key   string // full dotted key (e.g. "message.role")
	depth int    // 0 = top-level, 1 = nested child
}

type Focus int

const (
	FocusTable Focus = iota
	FocusFiles
)

type Model struct {
	files          []string // discovered .jsonl paths
	fileIdx        int
	focus          Focus
	col            *jsonldb.Collection
	schema         jsonldb.Schema
	result         *jsonldb.Result
	columns        []string // visible keys
	page           int      // 1-based
	pageSize       int
	cursor         int // selected row within page
	colCursor      int // selected column index (for sort)
	sortField      string
	sortDesc       bool
	filter         string
	filterSaved    string // prior filter value before entering filter mode
	filterErr      error
	mode           Mode
	detail         jsonldb.Doc // selected doc in detail view
	width          int
	height         int
	showAllColumns bool
	showHelp       bool
	defaultCap     int
	status         string
	// column picker
	pickList   []pickRow
	picked     map[string]bool
	pickCursor int
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
		files:      files,
		fileIdx:    0,
		focus:      FocusTable,
		page:       1,
		pageSize:   20,
		mode:       ModeList,
		defaultCap: 8,
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
	cap := m.defaultCap
	if cap == 0 {
		cap = 8
	}
	m.columns = defaultColumns(m.schema, cap)
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

func (m *Model) activeColumns() []string {
	if m.showAllColumns {
		return m.columns
	}
	return m.visibleColumns(m.defaultCap)
}

func copyToClipboard(b []byte) error {
	cmd := exec.Command("pbcopy")
	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	in.Write(b)
	in.Close()
	return cmd.Wait()
}

// exportCurrentView writes all docs in m.result atomically to a sibling file
// named <basename-without-ext>.export.jsonl in the same directory as m.files[m.fileIdx].
func (m *Model) exportCurrentView() error {
	src := m.files[m.fileIdx]
	dir := filepath.Dir(src)
	base := filepath.Base(src)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	dest := filepath.Join(dir, stem+".export.jsonl")

	tmp, err := os.CreateTemp(dir, ".lazyjsonl-export-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	for _, d := range m.result.Docs() {
		if _, err := tmp.Write(d.Raw()); err != nil {
			tmp.Close()
			return err
		}
		if _, err := tmp.Write([]byte{'\n'}); err != nil {
			tmp.Close()
			return err
		}
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, dest)
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
		// rows that fit the table pane: height − title − footer − border − pane-title − header
		ps := msg.Height - 6
		if ps < 1 {
			ps = 1
		}
		m.pageSize = ps
		if m.page > m.pageCount() {
			m.page = m.pageCount()
		}
		if rows := len(m.pageRows()); m.cursor >= rows {
			m.cursor = rows - 1
			if m.cursor < 0 {
				m.cursor = 0
			}
		}
		return m, nil
	case tea.KeyPressMsg:
		switch m.mode {
		case ModeList:
			return m.updateList(msg)
		case ModeDetail:
			m.mode = ModeList
			return m, nil
		case ModeFilter:
			return m.updateFilter(msg)
		case ModeConfirm:
			return m.updateConfirm(msg)
		case ModeColumns:
			return m.updateColumns(msg)
		}
	}
	return m, nil
}

func (m *Model) updateList(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Help overlay: any key closes it.
	if m.showHelp {
		m.showHelp = false
		return m, nil
	}
	// Clear status on every key except keys that set it (y, e).
	k := msg.String()
	if k != "y" && k != "e" {
		m.status = ""
	}

	rows := len(m.pageRows())
	switch k {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "j", "down":
		if m.focus == FocusFiles {
			if m.fileIdx < len(m.files)-1 {
				m.fileIdx++
				if err := m.openCurrent(); err != nil {
					m.status = "open failed"
				}
			}
		} else if m.cursor < rows-1 {
			m.cursor++
		}
	case "k", "up":
		if m.focus == FocusFiles {
			if m.fileIdx > 0 {
				m.fileIdx--
				if err := m.openCurrent(); err != nil {
					m.status = "open failed"
				}
			}
		} else if m.cursor > 0 {
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
			if err := m.openCurrent(); err != nil {
				m.status = "open failed"
			}
		}
	case "K":
		if m.fileIdx > 0 {
			m.fileIdx--
			if err := m.openCurrent(); err != nil {
				m.status = "open failed"
			}
		}
	case "/":
		m.filterSaved = m.filter
		m.mode = ModeFilter
		return m, nil
	case "esc":
		if m.filter != "" {
			m.filter = ""
			m.applyFilter() // re-query all
		}
	case "c":
		m.openColumnPicker()
		return m, nil
	case "?":
		m.showHelp = !m.showHelp
	case "y":
		if d, ok := m.selectedDoc(); ok {
			if err := copyToClipboard(d.Raw()); err != nil {
				m.status = "clipboard unavailable"
			} else {
				m.status = "yanked"
			}
		}
	case "e":
		src := m.files[m.fileIdx]
		base := filepath.Base(src)
		ext := filepath.Ext(base)
		stem := strings.TrimSuffix(base, ext)
		exportBase := stem + ".export.jsonl"
		if err := m.exportCurrentView(); err != nil {
			m.status = "export failed"
		} else {
			m.status = "exported to " + exportBase
		}
	case "L", "alt+right":
		if m.colCursor < len(m.activeColumns())-1 {
			m.colCursor++
		}
	case "H", "alt+left":
		if m.colCursor > 0 {
			m.colCursor--
		}
	case "s":
		cols := m.activeColumns()
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
	case "enter":
		if d, ok := m.selectedDoc(); ok {
			m.detail = d
			m.mode = ModeDetail
		}
	case "r":
		if err := m.col.Reload(); err != nil {
			m.status = "reload failed"
		} else {
			m.refresh()
		}
	}
	return m, nil
}

func (m *Model) updateFilter(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.applyFilter()
		return m, nil
	case "esc":
		m.filter = m.filterSaved
		m.mode = ModeList
		return m, nil
	case "backspace":
		if len(m.filter) > 0 {
			m.filter = m.filter[:len(m.filter)-1]
		}
		return m, nil
	default:
		if msg.Text != "" {
			m.filter += msg.Text
		}
		return m, nil
	}
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

func (m *Model) updateConfirm(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		if d, ok := m.selectedDoc(); ok {
			if err := m.col.DeleteAt(d.Line()); err != nil {
				m.status = "delete failed"
			} else {
				m.refresh()
			}
		}
		m.mode = ModeList
	case "n", "esc":
		m.mode = ModeList
	}
	return m, nil
}

// openColumnPicker builds the candidate list (top-level schema fields + one
// level of nested object children) and enters the picker, preselecting the
// currently-shown columns.
func (m *Model) openColumnPicker() {
	m.picked = map[string]bool{}
	for _, c := range m.columns {
		m.picked[c] = true
	}
	m.buildPickList()
	m.pickCursor = 0
	m.mode = ModeColumns
}

func (m *Model) buildPickList() {
	m.pickList = nil
	sample, _ := m.col.First()
	for _, f := range m.schema {
		m.pickList = append(m.pickList, pickRow{key: f.Key, depth: 0})
		if v, ok := sample.Get(f.Key); ok {
			if obj, ok := v.(map[string]any); ok {
				keys := make([]string, 0, len(obj))
				for k := range obj {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					m.pickList = append(m.pickList, pickRow{key: f.Key + "." + k, depth: 1})
				}
			}
		}
	}
}

func (m *Model) updateColumns(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.pickCursor < len(m.pickList)-1 {
			m.pickCursor++
		}
	case "k", "up":
		if m.pickCursor > 0 {
			m.pickCursor--
		}
	case " ", "space":
		if m.pickCursor < len(m.pickList) {
			k := m.pickList[m.pickCursor].key
			m.picked[k] = !m.picked[k]
		}
	case "a":
		for _, r := range m.pickList {
			m.picked[r.key] = true
		}
	case "N":
		m.picked = map[string]bool{}
	case "enter", "c", "tab", "esc", "q", "ctrl+c":
		m.applyColumnPick()
		m.mode = ModeList
	}
	return m, nil
}

func (m *Model) applyColumnPick() {
	var cols []string
	for _, r := range m.pickList {
		if m.picked[r.key] {
			cols = append(cols, r.key)
		}
	}
	if len(cols) == 0 {
		return // keep current columns; never show zero
	}
	m.columns = cols
	m.showAllColumns = true // selection is explicit now
	if m.colCursor >= len(cols) {
		m.colCursor = 0
	}
}

func (m *Model) Close() error { return m.col.Close() }

