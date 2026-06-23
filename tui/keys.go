package tui

import "charm.land/bubbles/v2/key"

// keyMap holds every binding, used to drive the bubbles help component (short
// footer hints + full overlay). The actual key handling lives in updateList;
// these bindings exist for display, so their keys mirror that switch.
type keyMap struct {
	Move     key.Binding
	Page     key.Binding
	Ends     key.Binding
	Column   key.Binding
	Sort     key.Binding
	Stats    key.Binding
	Group    key.Binding
	Chart    key.Binding
	File     key.Binding
	Tab      key.Binding
	Dive     key.Binding
	Back     key.Binding
	Enter    key.Binding
	Jump     key.Binding
	Filter   key.Binding
	CellFilt key.Binding
	Search   key.Binding
	Esc      key.Binding
	Cols     key.Binding
	Delete   key.Binding
	Export   key.Binding
	Yank     key.Binding
	YankCell key.Binding
	Reload   key.Binding
	Format   key.Binding
	Mouse    key.Binding
	Help     key.Binding
	Quit     key.Binding
}

var keys = keyMap{
	Move:     key.NewBinding(key.WithKeys("j", "k", "up", "down"), key.WithHelp("j/k", "move")),
	Page:     key.NewBinding(key.WithKeys("[", "]", "pgup", "pgdown"), key.WithHelp("[ ]", "page")),
	Ends:     key.NewBinding(key.WithKeys("g", "G"), key.WithHelp("g/G", "first/last")),
	Column:   key.NewBinding(key.WithKeys("h", "l", "left", "right"), key.WithHelp("h/l", "column")),
	Sort:     key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sort")),
	Stats:    key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "stats")),
	Group:    key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "group by")),
	Chart:    key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "visualize")),
	File:     key.NewBinding(key.WithKeys("J", "K"), key.WithHelp("J/K", "file")),
	Tab:      key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "pane")),
	Dive:     key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "dive")),
	Back:     key.NewBinding(key.WithKeys("esc", "backspace"), key.WithHelp("esc", "back")),
	Enter:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "view")),
	Jump:     key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "jump to #")),
	Filter:   key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
	CellFilt: key.NewBinding(key.WithKeys("f", "F"), key.WithHelp("f/F", "filter to/≠ cell")),
	Search:   key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	Esc:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear")),
	Cols:     key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "columns")),
	Delete:   key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
	Export:   key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "export")),
	Yank:     key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "yank record")),
	YankCell: key.NewBinding(key.WithKeys("Y"), key.WithHelp("Y", "yank cell")),
	Reload:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reload")),
	Format:   key.NewBinding(key.WithKeys("#"), key.WithHelp("#", "raw/pretty values")),
	Mouse:    key.NewBinding(key.WithKeys("wheel"), key.WithHelp("wheel", "scroll")),
	Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}

// shortKeys returns the context-appropriate footer bindings.
func (m *Model) shortKeys() []key.Binding {
	if m.focus == FocusFiles {
		return []key.Binding{keys.Move, keys.Search, keys.File, keys.Tab, keys.Help, keys.Quit}
	}
	if len(m.drillPath) > 0 {
		return []key.Binding{keys.Back, keys.Dive, keys.Column, keys.Sort, keys.Filter, keys.Cols, keys.Help, keys.Quit}
	}
	return []key.Binding{keys.Move, keys.Column, keys.Page, keys.Filter, keys.Enter, keys.Sort, keys.Stats, keys.Group, keys.Dive, keys.Cols, keys.Help, keys.Quit}
}

// fullGroups returns the columns for the full help overlay.
func fullGroups() [][]key.Binding {
	return [][]key.Binding{
		{keys.Move, keys.Column, keys.Page, keys.Ends, keys.File, keys.Tab, keys.Jump},
		{keys.Enter, keys.Sort, keys.Stats, keys.Group, keys.Chart, keys.Dive, keys.Back, keys.Cols, keys.Filter, keys.CellFilt, keys.Esc},
		{keys.Yank, keys.YankCell, keys.Delete, keys.Export, keys.Reload, keys.Format, keys.Mouse, keys.Help, keys.Quit},
	}
}
