package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/xZhad/jsonldb"
)

func key(r rune) tea.KeyPressMsg {
	if r >= 'A' && r <= 'Z' {
		return tea.KeyPressMsg{Code: rune(r - 'A' + 'a'), Text: string(r), Mod: tea.ModShift}
	}
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

func TestListNavigation(t *testing.T) {
	m, _ := New(fixture(t))
	defer m.col.Close()
	m.pageSize = 2 // 3 docs -> 2 pages

	// down moves cursor
	mi, _ := m.Update(key('j'))
	m = mi.(*Model)
	if m.cursor != 1 {
		t.Errorf("cursor after j = %d, want 1", m.cursor)
	}
	// down clamps within page (page 1 has 2 rows: indices 0,1)
	mi, _ = m.Update(key('j'))
	m = mi.(*Model)
	if m.cursor != 1 {
		t.Errorf("cursor clamped = %d, want 1", m.cursor)
	}
	// next page
	mi, _ = m.Update(key('l'))
	m = mi.(*Model)
	if m.page != 2 {
		t.Errorf("page after l = %d, want 2", m.page)
	}
	if m.cursor != 0 {
		t.Errorf("cursor should reset to 0 on page change, got %d", m.cursor)
	}
	if len(m.pageRows()) != 1 {
		t.Errorf("page 2 rows = %d, want 1", len(m.pageRows()))
	}
	// next page clamps (only 2 pages)
	mi, _ = m.Update(key('l'))
	m = mi.(*Model)
	if m.page != 2 {
		t.Errorf("page clamped = %d, want 2", m.page)
	}
}

func fixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "s.jsonl")
	body := `{"id":"a","topic":"ml","dur":1500,"done":true}
{"id":"b","topic":"go","dur":900,"done":false}
{"id":"c","topic":"ml","dur":1200,"done":true}
`
	if err := os.WriteFile(p, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFileSwitching(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.jsonl"), []byte("{\"id\":\"a1\"}\n{\"id\":\"a2\"}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.jsonl"), []byte("{\"id\":\"b1\"}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { m.col.Close() }()

	// starts on a.jsonl (sorted first), 2 docs
	if m.result.Count() != 2 {
		t.Fatalf("initial count = %d, want 2 (a.jsonl)", m.result.Count())
	}
	// tab toggles focus
	mi, _ := m.Update(key('\t'))
	_ = mi
	// J -> next file (b.jsonl), 1 doc, fresh result
	mi, _ = m.Update(key('J'))
	m = mi.(*Model)
	if m.fileIdx != 1 {
		t.Errorf("fileIdx after J = %d, want 1", m.fileIdx)
	}
	if m.result.Count() != 1 {
		t.Errorf("count after switch = %d, want 1 (b.jsonl)", m.result.Count())
	}
	// J again clamps (only 2 files)
	mi, _ = m.Update(key('J'))
	m = mi.(*Model)
	if m.fileIdx != 1 {
		t.Errorf("fileIdx clamped = %d, want 1", m.fileIdx)
	}
	// K -> back to a.jsonl
	mi, _ = m.Update(key('K'))
	m = mi.(*Model)
	if m.fileIdx != 0 || m.result.Count() != 2 {
		t.Errorf("after K: fileIdx=%d count=%d, want 0/2", m.fileIdx, m.result.Count())
	}
}

func TestNewModel(t *testing.T) {
	m, err := New(fixture(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer m.col.Close()
	if m.mode != ModeList {
		t.Errorf("initial mode = %v, want ModeList", m.mode)
	}
	if m.result.Count() != 3 {
		t.Errorf("initial result count = %d, want 3", m.result.Count())
	}
	// columns: id/topic/dur/done all presence 1.0 -> sorted by key asc within ties
	cols := m.visibleColumns(10)
	if len(cols) != 4 {
		t.Errorf("columns = %v, want 4", cols)
	}
	// single file -> files has one entry
	if len(m.files) != 1 {
		t.Errorf("files = %v, want 1", m.files)
	}
}

func TestDefaultColumnsCap(t *testing.T) {
	m, _ := New(fixture(t))
	defer m.col.Close()
	if got := m.visibleColumns(2); len(got) != 2 {
		t.Errorf("visibleColumns(2) = %v, want 2", got)
	}
}

func TestNewFromDirectory(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"b.jsonl", "a.jsonl", "notjson.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("{\"x\":1}\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	m, err := New(dir)
	if err != nil {
		t.Fatalf("New(dir): %v", err)
	}
	defer m.col.Close()
	// only .jsonl files, sorted: a.jsonl, b.jsonl
	if len(m.files) != 2 {
		t.Fatalf("files = %v, want 2", m.files)
	}
	if filepath.Base(m.files[0]) != "a.jsonl" {
		t.Errorf("files[0] = %q, want a.jsonl (sorted)", m.files[0])
	}
}

func TestFilterApplyAndError(t *testing.T) {
	m, _ := New(fixture(t))
	defer m.col.Close()

	// enter filter mode
	mi, _ := m.Update(key('/'))
	m = mi.(*Model)
	if m.mode != ModeFilter {
		t.Fatalf("mode = %v, want ModeFilter", m.mode)
	}
	// type "done=true"
	for _, r := range "done=true" {
		mi, _ = m.Update(key(r))
		m = mi.(*Model)
	}
	mi, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = mi.(*Model)
	if m.mode != ModeList {
		t.Errorf("mode after enter = %v, want ModeList", m.mode)
	}
	if m.result.Count() != 2 {
		t.Errorf("filtered count = %d, want 2", m.result.Count())
	}
	if m.filterErr != nil {
		t.Errorf("unexpected filterErr: %v", m.filterErr)
	}

	// bad filter: keeps prior result, sets error
	mi, _ = m.Update(key('/'))
	m = mi.(*Model)
	m.filter = "done="
	mi, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = mi.(*Model)
	if m.filterErr == nil {
		t.Errorf("expected filterErr for bad DSL")
	}
	if m.result.Count() != 2 {
		t.Errorf("result should be unchanged on parse error, got %d", m.result.Count())
	}
	if m.mode != ModeFilter {
		t.Errorf("should stay in ModeFilter on error, got %v", m.mode)
	}
}

func TestFilterEscRestores(t *testing.T) {
	m, _ := New(fixture(t))
	defer m.col.Close()

	// Apply an initial filter to have a known prior filter value
	mi, _ := m.Update(key('/'))
	m = mi.(*Model)
	for _, r := range "done=true" {
		mi, _ = m.Update(key(r))
		m = mi.(*Model)
	}
	mi, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = mi.(*Model)
	if m.filter != "done=true" {
		t.Fatalf("filter after apply = %q, want 'done=true'", m.filter)
	}
	if m.result.Count() != 2 {
		t.Fatalf("filtered count = %d, want 2", m.result.Count())
	}

	// Enter filter mode again (filterSaved should be "done=true")
	mi, _ = m.Update(key('/'))
	m = mi.(*Model)
	if m.mode != ModeFilter {
		t.Fatalf("mode = %v, want ModeFilter", m.mode)
	}
	if m.filterSaved != "done=true" {
		t.Fatalf("filterSaved = %q, want 'done=true'", m.filterSaved)
	}

	// Type extra characters: "extra" -> filter becomes "done=trueextra"
	for _, r := range "extra" {
		mi, _ = m.Update(key(r))
		m = mi.(*Model)
	}
	if m.filter != "done=trueextra" {
		t.Fatalf("filter after typing = %q, want 'done=trueextra'", m.filter)
	}

	// Press esc: filter should be restored to filterSaved ("done=true"), mode should be ModeList
	mi, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = mi.(*Model)
	if m.mode != ModeList {
		t.Errorf("mode after esc = %v, want ModeList", m.mode)
	}
	if m.filter != "done=true" {
		t.Errorf("filter after esc = %q, want 'done=true' (restored)", m.filter)
	}
}

func TestColumnSort(t *testing.T) {
	m, _ := New(fixture(t))
	defer m.col.Close()
	m.pageSize = 10
	// find the index of "dur" in visible columns; move colCursor there
	cols := m.visibleColumns(10)
	durIdx := -1
	for i, c := range cols {
		if c == "dur" {
			durIdx = i
		}
	}
	if durIdx < 0 {
		t.Fatal("dur not in columns")
	}
	for i := 0; i < durIdx; i++ {
		mi, _ := m.Update(key('L'))
		m = mi.(*Model)
	}
	if m.colCursor != durIdx {
		t.Fatalf("colCursor = %d, want %d", m.colCursor, durIdx)
	}
	// sort ascending by dur
	mi, _ := m.Update(key('s'))
	m = mi.(*Model)
	rows := m.pageRows()
	if rows[0].GetString("id") != "b" { // dur 900 is smallest
		t.Errorf("asc sort first id = %q, want b", rows[0].GetString("id"))
	}
	// sort again toggles to desc
	mi, _ = m.Update(key('s'))
	m = mi.(*Model)
	rows = m.pageRows()
	if rows[0].GetString("id") != "a" { // dur 1500 largest
		t.Errorf("desc sort first id = %q, want a", rows[0].GetString("id"))
	}
}

func TestDeleteRow(t *testing.T) {
	m, _ := New(fixture(t))
	defer m.col.Close()
	m.pageSize = 10
	// cursor on first row (id "a"), delete it
	mi, _ := m.Update(key('d'))
	m = mi.(*Model)
	if m.mode != ModeConfirm {
		t.Fatalf("mode = %v, want ModeConfirm", m.mode)
	}
	mi, _ = m.Update(key('y'))
	m = mi.(*Model)
	if m.mode != ModeList {
		t.Errorf("mode after confirm = %v, want ModeList", m.mode)
	}
	if m.result.Count() != 2 {
		t.Errorf("count after delete = %d, want 2", m.result.Count())
	}
	if _, ok := findID(m, "a"); ok {
		t.Errorf("id a should be deleted")
	}
}

func TestColumnToggleAndHelp(t *testing.T) {
	m, _ := New(fixture(t))
	defer m.col.Close()
	full := len(m.columns)
	_ = m.columns // unchanged; cap default is 8 so all 4 show
	// force a small cap to see toggle effect
	m.defaultCap = 2
	if len(m.visibleColumns(m.defaultCap)) != 2 {
		t.Fatalf("expected capped 2")
	}
	mi, _ := m.Update(key('c'))
	m = mi.(*Model)
	if !m.showAllColumns {
		t.Errorf("c should toggle showAllColumns on")
	}
	if len(m.activeColumns()) != full {
		t.Errorf("showAll should reveal all %d columns, got %d", full, len(m.activeColumns()))
	}
	mi, _ = m.Update(key('?'))
	m = mi.(*Model)
	if !m.showHelp {
		t.Errorf("? should toggle help")
	}
}

func findID(m *Model, id string) (jsonldb.Doc, bool) {
	for _, d := range m.result.Docs() {
		if d.GetString("id") == id {
			return d, true
		}
	}
	return jsonldb.Doc{}, false
}

func TestExportKey(t *testing.T) {
	p := fixture(t)
	m, err := New(p)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer m.col.Close()
	m.pageSize = 10

	// Press 'e' to export current view
	mi, _ := m.Update(key('e'))
	m = mi.(*Model)

	// status should be set to "exported to ..."
	if m.status == "" {
		t.Errorf("status should be set after export, got empty")
	}
	if !strings.HasPrefix(m.status, "exported to ") {
		t.Errorf("status = %q, want prefix 'exported to '", m.status)
	}

	// The export file should exist in the same dir as the source
	dir := filepath.Dir(p)
	exportPath := filepath.Join(dir, "s.export.jsonl")
	data, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("export file not found: %v", err)
	}

	// Count lines (each doc is one line)
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	wantLines := m.result.Count()
	if len(lines) != wantLines {
		t.Errorf("export file has %d lines, want %d", len(lines), wantLines)
	}
}

func TestDetailAndReload(t *testing.T) {
	m, _ := New(fixture(t))
	defer m.col.Close()
	m.pageSize = 10

	mi, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = mi.(*Model)
	if m.mode != ModeDetail {
		t.Fatalf("mode = %v, want ModeDetail", m.mode)
	}
	if m.detail.GetString("id") != "a" {
		t.Errorf("detail id = %q, want a", m.detail.GetString("id"))
	}
	mi, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = mi.(*Model)
	if m.mode != ModeList {
		t.Errorf("esc should return to list, got %v", m.mode)
	}
	// reload is a no-op here but must not error or change count
	mi, _ = m.Update(key('r'))
	m = mi.(*Model)
	if m.result.Count() != 3 {
		t.Errorf("count after reload = %d, want 3", m.result.Count())
	}
}
