package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/xZhad/jsonldb"
)

func kp(r rune) tea.KeyPressMsg {
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
	mi, _ := m.Update(kp('j'))
	m = mi.(*Model)
	if m.cursor != 1 {
		t.Errorf("cursor after j = %d, want 1", m.cursor)
	}
	// down clamps within page (page 1 has 2 rows: indices 0,1)
	mi, _ = m.Update(kp('j'))
	m = mi.(*Model)
	if m.cursor != 1 {
		t.Errorf("cursor clamped = %d, want 1", m.cursor)
	}
	// next page
	mi, _ = m.Update(kp('l'))
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
	mi, _ = m.Update(kp('l'))
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
	mi, _ := m.Update(kp('\t'))
	_ = mi
	// J -> next file (b.jsonl), 1 doc, fresh result
	mi, _ = m.Update(kp('J'))
	m = mi.(*Model)
	if m.fileIdx != 1 {
		t.Errorf("fileIdx after J = %d, want 1", m.fileIdx)
	}
	if m.result.Count() != 1 {
		t.Errorf("count after switch = %d, want 1 (b.jsonl)", m.result.Count())
	}
	// J again clamps (only 2 files)
	mi, _ = m.Update(kp('J'))
	m = mi.(*Model)
	if m.fileIdx != 1 {
		t.Errorf("fileIdx clamped = %d, want 1", m.fileIdx)
	}
	// K -> back to a.jsonl
	mi, _ = m.Update(kp('K'))
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
	mi, _ := m.Update(kp('/'))
	m = mi.(*Model)
	if m.mode != ModeFilter {
		t.Fatalf("mode = %v, want ModeFilter", m.mode)
	}
	// type "done=true"
	for _, r := range "done=true" {
		mi, _ = m.Update(kp(r))
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
	mi, _ = m.Update(kp('/'))
	m = mi.(*Model)
	m.filterInput.SetValue("done=")
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
	mi, _ := m.Update(kp('/'))
	m = mi.(*Model)
	for _, r := range "done=true" {
		mi, _ = m.Update(kp(r))
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
	mi, _ = m.Update(kp('/'))
	m = mi.(*Model)
	if m.mode != ModeFilter {
		t.Fatalf("mode = %v, want ModeFilter", m.mode)
	}
	if m.filterSaved != "done=true" {
		t.Fatalf("filterSaved = %q, want 'done=true'", m.filterSaved)
	}

	// Type extra characters into the input: value -> "done=trueextra".
	// (m.filter holds the *applied* filter and only changes on enter.)
	for _, r := range "extra" {
		mi, _ = m.Update(kp(r))
		m = mi.(*Model)
	}
	if got := m.filterInput.Value(); got != "done=trueextra" {
		t.Fatalf("input value after typing = %q, want 'done=trueextra'", got)
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
		mi, _ := m.Update(kp('L'))
		m = mi.(*Model)
	}
	if m.colCursor != durIdx {
		t.Fatalf("colCursor = %d, want %d", m.colCursor, durIdx)
	}
	// sort ascending by dur
	mi, _ := m.Update(kp('s'))
	m = mi.(*Model)
	rows := m.pageRows()
	if rows[0].GetString("id") != "b" { // dur 900 is smallest
		t.Errorf("asc sort first id = %q, want b", rows[0].GetString("id"))
	}
	// sort again toggles to desc
	mi, _ = m.Update(kp('s'))
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
	mi, _ := m.Update(kp('d'))
	m = mi.(*Model)
	if m.mode != ModeConfirm {
		t.Fatalf("mode = %v, want ModeConfirm", m.mode)
	}
	mi, _ = m.Update(kp('y'))
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

func TestColumnPicker(t *testing.T) {
	m, _ := New(fixture(t))
	defer m.col.Close()
	// c opens the picker, preselecting current columns
	mi, _ := m.Update(kp('c'))
	m = mi.(*Model)
	if m.mode != ModeColumns {
		t.Fatalf("c should open column picker, mode=%v", m.mode)
	}
	if len(m.pickList) == 0 {
		t.Fatal("pick list empty")
	}
	// toggle the first candidate off, then apply with enter
	first := m.pickList[0].key
	mi, _ = m.Update(kp(' ')) // space toggles
	m = mi.(*Model)
	if m.picked[first] {
		t.Errorf("space should have toggled %q off", first)
	}
	mi, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = mi.(*Model)
	if m.mode != ModeList {
		t.Errorf("enter should apply + return to list, mode=%v", m.mode)
	}
	for _, c := range m.columns {
		if c == first {
			t.Errorf("deselected column %q still shown", first)
		}
	}
}

func TestHelpToggle(t *testing.T) {
	m, _ := New(fixture(t))
	defer m.col.Close()
	mi, _ := m.Update(kp('?'))
	m = mi.(*Model)
	if !m.showHelp {
		t.Errorf("? should toggle help on")
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
	mi, _ := m.Update(kp('e'))
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
	mi, _ = m.Update(kp('r'))
	m = mi.(*Model)
	if m.result.Count() != 3 {
		t.Errorf("count after reload = %d, want 3", m.result.Count())
	}
}

func TestDrillIntoColumn(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "x.jsonl"), []byte(`{"id":"a","message":{"role":"user","content":"hi"}}
{"id":"b","message":{"role":"assistant","content":"yo"}}
`), 0644)
	m, _ := New(dir)
	defer m.col.Close()
	m.showAllColumns = true
	m.columns = []string{"id", "message"}
	m.colCursor = 1 // focus the message (object) column
	mi, _ := m.Update(kp(' '))
	m = mi.(*Model)
	got := strings.Join(m.columns, ",")
	if !strings.Contains(got, "message.role") || !strings.Contains(got, "message.content") {
		t.Fatalf("dive columns = %v", m.columns)
	}
	if len(m.drillPath) != 1 || m.drillPath[0] != "message" {
		t.Errorf("crumb = %v", m.drillPath)
	}
	mi, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	m = mi.(*Model)
	if strings.Join(m.columns, ",") != "id,message" {
		t.Errorf("after backspace columns = %v", m.columns)
	}
}

func TestPickerInDrillListsSubfields(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "x.jsonl"), []byte(`{"id":"a","message":{"role":"user","content":"hi"}}
{"id":"b","message":{"role":"assistant","content":"yo","model":"x"}}
`), 0644)
	m, _ := New(dir)
	defer m.col.Close()
	m.showAllColumns = true
	m.columns = []string{"id", "message"}
	m.colCursor = 1
	mi, _ := m.Update(kp(' ')) // dive into message
	m = mi.(*Model)
	m.openColumnPicker()
	if len(m.pickList) == 0 {
		t.Fatal("empty pick list in drill")
	}
	for _, r := range m.pickList {
		if !strings.HasPrefix(r.key, "message.") {
			t.Errorf("pick row %q not under message.", r.key)
		}
	}
	keys := map[string]bool{}
	for _, r := range m.pickList {
		keys[r.key] = true
	}
	for _, want := range []string{"message.role", "message.content", "message.model"} {
		if !keys[want] {
			t.Errorf("missing pick row %q (got %v)", want, m.pickList)
		}
	}
}

func send(m *Model, msg tea.Msg) *Model { mi, _ := m.Update(msg); return mi.(*Model) }

func TestMultiLevelDrillTrim(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "x.jsonl"), []byte(
		`{"id":"a","message":{"role":"user","usage":{"input":10,"output":20}}}
{"id":"b","message":{"role":"assistant","usage":{"input":5,"output":7}}}
`), 0644)
	m, _ := New(dir)
	defer m.col.Close()
	m.showAllColumns = true
	m.columns = []string{"id", "message"}
	m.colCursor = 1
	m = send(m, kp(' ')) // dive into message → message.role, message.usage, ...
	// focus message.usage and dive again
	for i, c := range m.columns {
		if c == "message.usage" {
			m.colCursor = i
		}
	}
	m = send(m, kp(' ')) // dive into message.usage → message.usage.input/output
	if got := strings.Join(m.columns, ","); !strings.Contains(got, "message.usage.input") {
		t.Fatalf("level-2 columns = %v", m.columns)
	}
	if pfx := m.drillPrefix(); pfx != "message.usage" {
		t.Errorf("drillPrefix = %q, want message.usage", pfx)
	}
	if cr := strings.Join(m.drillCrumbs(), "/"); cr != "message/usage" {
		t.Errorf("drillCrumbs = %q, want message/usage", cr)
	}
	// header trim drops the full prefix → ".input" not "message.usage.input"
	if pfx := m.drillPrefix(); !strings.HasPrefix(strings.TrimPrefix("message.usage.input", pfx), ".") {
		t.Errorf("trim of message.usage.input by %q did not yield .input", pfx)
	}
}

func TestFileSearch(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"alpha.jsonl", "beta.jsonl", "gamma.jsonl"} {
		os.WriteFile(filepath.Join(dir, n), []byte(`{"x":1}`+"\n"), 0644)
	}
	m, _ := New(dir)
	defer m.col.Close()
	m.focus = FocusFiles
	m = send(m, kp('/'))
	if m.mode != ModeFileSearch {
		t.Fatalf("mode = %v, want ModeFileSearch", m.mode)
	}
	for _, r := range "bet" {
		m = send(m, kp(r))
	}
	cf := m.curFiles()
	if len(cf) != 1 || filepath.Base(cf[0]) != "beta.jsonl" {
		t.Fatalf("filtered files = %v, want [beta.jsonl]", cf)
	}
	m = send(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.mode != ModeList {
		t.Errorf("mode after enter = %v, want ModeList", m.mode)
	}
	// esc clears the filter back to all files
	m.focus = FocusFiles
	m = send(m, kp('/'))
	m = send(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if len(m.curFiles()) != 3 {
		t.Errorf("after esc curFiles = %d, want 3", len(m.curFiles()))
	}
}

func TestIsDateString(t *testing.T) {
	yes := []string{"2026-06-21T13:51:32.672Z", "2026-06-21", "2026-06-21 13:51:32", "2026/06/21"}
	no := []string{"", "hello", "deepseek-v4", "12345", "opencode-go"}
	for _, s := range yes {
		if !isDateString(s) {
			t.Errorf("isDateString(%q) = false, want true", s)
		}
	}
	for _, s := range no {
		if isDateString(s) {
			t.Errorf("isDateString(%q) = true, want false", s)
		}
	}
}

func TestNullCellValue(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "x.jsonl"), []byte(`{"a":null,"b":1}`+"\n"), 0644)
	m, _ := New(dir)
	defer m.col.Close()
	d := m.pageRows()[0]
	txt, st := cellValue(d, "a")
	if txt != "null" {
		t.Errorf("null cell text = %q, want null", txt)
	}
	if st.Render("null") != styleNull.Render("null") {
		t.Errorf("null cell style not styleNull")
	}
	// absent key → blank
	if txt, _ := cellValue(d, "zzz"); txt != "" {
		t.Errorf("absent key text = %q, want empty", txt)
	}
}

func TestEscBacksOutOfDive(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "x.jsonl"), []byte(
		`{"id":"a","message":{"role":"user"}}`+"\n"), 0644)
	m, _ := New(dir)
	defer m.col.Close()
	m.showAllColumns = true
	m.columns = []string{"id", "message"}
	m.colCursor = 1
	m = send(m, kp(' ')) // dive into message
	if len(m.drillPath) != 1 {
		t.Fatalf("drillPath after dive = %d, want 1", len(m.drillPath))
	}
	m = send(m, tea.KeyPressMsg{Code: tea.KeyEscape}) // esc backs out
	if len(m.drillPath) != 0 {
		t.Errorf("drillPath after esc = %d, want 0", len(m.drillPath))
	}
}
