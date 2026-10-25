package tui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
)

// boardKeys holds the key bindings jt matches against. navigate and topBottom
// only appear in the help footer; the bindings next to them do the matching.
type boardKeys struct {
	left, right, up, down, top, bottom key.Binding
	view, comments, edit, yank, open   key.Binding
	move, assign                       key.Binding
	refresh, projects, quit, close     key.Binding
	filter, nextTab, prevTab           key.Binding
	forceQuit, interrupt, confirm      key.Binding
	deny, accept, send, blur           key.Binding
	toggle, undo                       key.Binding

	navigate, topBottom key.Binding
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
	refresh:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	projects: key.NewBinding(key.WithKeys("ctrl+p"), key.WithHelp("ctrl+p", "projects")),
	quit:     key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),

	// close dismisses a popup.
	close:     key.NewBinding(key.WithKeys("esc", "q")),
	forceQuit: key.NewBinding(key.WithKeys("ctrl+d")),
	interrupt: key.NewBinding(key.WithKeys("ctrl+c")),
	confirm:   key.NewBinding(key.WithKeys("y", "Y")),
	deny:      key.NewBinding(key.WithKeys("n", "N", "esc")),
	accept:    key.NewBinding(key.WithKeys("enter")),
	send:      key.NewBinding(key.WithKeys("ctrl+s")),
	blur:      key.NewBinding(key.WithKeys("esc")),
	toggle:    key.NewBinding(key.WithKeys(" ")),
	undo:      key.NewBinding(key.WithKeys("u")),
	nextTab:   key.NewBinding(key.WithKeys("tab")),
	prevTab:   key.NewBinding(key.WithKeys("shift+tab")),

	navigate:  key.NewBinding(key.WithHelp("hjkl", "navigate")),
	topBottom: key.NewBinding(key.WithHelp("g/G", "top/bottom")),
}

// ShortHelp implements help.KeyMap.
func (k boardKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.navigate, k.topBottom, k.view, k.comments, k.move, k.assign, k.edit, k.yank, k.open, k.filter, k.refresh, k.projects, k.quit}
}

// FullHelp implements help.KeyMap.
func (k boardKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}

func newHelp() help.Model {
	h := help.New()
	h.Styles.ShortKey = dimStyle
	h.Styles.ShortDesc = dimStyle
	h.Styles.ShortSeparator = dimStyle
	h.Styles.Ellipsis = dimStyle
	return h
}
