package tui

import (
	"charm.land/bubbles/v2/key"
)

// boardKeys holds the key bindings jt matches against. navigate, topBottom,
// scroll and write only appear in help footers; other bindings do the matching.
type boardKeys struct {
	left, right, up, down, top, bottom key.Binding
	view, comments, edit, yank, open   key.Binding
	move, assign                       key.Binding
	refresh, projects, quit, close     key.Binding
	filter, nextTab, prevTab           key.Binding
	forceQuit, interrupt, confirm      key.Binding
	deny, accept, send, blur           key.Binding
	toggle, undo, theme, back          key.Binding

	navigate, topBottom, scroll, write key.Binding
}

var keys = boardKeys{
	left:   key.NewBinding(key.WithKeys("h", "left")),
	right:  key.NewBinding(key.WithKeys("l", "right")),
	up:     key.NewBinding(key.WithKeys("k", "up")),
	down:   key.NewBinding(key.WithKeys("j", "down")),
	top:    key.NewBinding(key.WithKeys("g")),
	bottom: key.NewBinding(key.WithKeys("G")),

	view:     key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "view")),
	comments: key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "comments")),
	edit:     key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
	move:     key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "move")),
	assign:   key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "assign")),
	yank:     key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "yank URL")),
	open:     key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open")),
	filter:   key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "filter")),
	theme:    key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "theme")),
	refresh:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	projects: key.NewBinding(key.WithKeys("ctrl+p"), key.WithHelp("ctrl+p", "projects")),
	quit:     key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),

	// close dismisses a popup.
	close:     key.NewBinding(key.WithKeys("esc", "q"), key.WithHelp("esc", "close")),
	forceQuit: key.NewBinding(key.WithKeys("ctrl+d")),
	interrupt: key.NewBinding(key.WithKeys("ctrl+c")),
	confirm:   key.NewBinding(key.WithKeys("y", "Y")),
	deny:      key.NewBinding(key.WithKeys("n", "N", "esc")),
	accept:    key.NewBinding(key.WithKeys("enter")),
	send:      key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "send")),
	blur:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "stop writing")),
	toggle:    key.NewBinding(key.WithKeys("space")),
	undo:      key.NewBinding(key.WithKeys("u")),
	back:      key.NewBinding(key.WithKeys("esc", "/", "shift+tab"), key.WithHelp("esc", "edit search")),
	nextTab:   key.NewBinding(key.WithKeys("tab")),
	prevTab:   key.NewBinding(key.WithKeys("shift+tab")),

	// help hides bindings without keys, so the help-only ones list theirs.
	navigate:  key.NewBinding(key.WithKeys("h", "j", "k", "l"), key.WithHelp("hjkl", "navigate")),
	topBottom: key.NewBinding(key.WithKeys("g", "G"), key.WithHelp("g/G", "top/bottom")),
	scroll:    key.NewBinding(key.WithKeys("up", "down", "left", "right"), key.WithHelp("↑↓←→", "scroll")),
	write:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "write")),
}

var resultsNavigateHelp = key.NewBinding(key.WithKeys("j", "k", "up", "down"), key.WithHelp("j/k", "navigate"))

// ShortHelp implements help.KeyMap.
func (k boardKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.navigate, k.topBottom, k.view, k.comments, k.move, k.assign, k.edit, k.yank, k.open, k.filter, k.theme, k.refresh, k.projects, k.quit}
}

// FullHelp implements help.KeyMap.
func (k boardKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}

// helpKeys is a help footer made of a fixed list of bindings.
type helpKeys []key.Binding

// ShortHelp implements help.KeyMap.
func (k helpKeys) ShortHelp() []key.Binding { return k }

// FullHelp implements help.KeyMap.
func (k helpKeys) FullHelp() [][]key.Binding { return [][]key.Binding{k} }

var (
	viewHelpKeys      = helpKeys{keys.scroll, keys.topBottom, keys.close}
	commentsHelpKeys  = helpKeys{keys.scroll, keys.topBottom, keys.write, keys.close}
	searchFormHelp    = helpKeys{formMoveHelp, formChooseHelp, formSearchHelp, formPickHelp, modeKey}
	searchJQLHelp     = helpKeys{formSearchHelp, formPickHelp, modeKey}
	searchResultsHelp = helpKeys{resultsNavigateHelp, keys.topBottom, keys.view, keys.comments, keys.move, keys.assign, keys.edit, keys.yank, keys.open, keys.refresh, keys.theme, keys.back, keys.quit}
	writingHelpKeys   = helpKeys{keys.send, keys.blur}
)
