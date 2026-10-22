package tui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
)

// boardKeys holds the key bindings jt matches against. navigate and topBottom
// only appear in the help footer; the bindings next to them do the matching.
type boardKeys struct {
	left, right, up, down, top, bottom key.Binding
	view, comments, move, yank, open   key.Binding
	projects, quit, close, forceQuit   key.Binding
	interrupt, confirm, deny, accept   key.Binding

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
	move:     key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "move")),
	yank:     key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "yank URL")),
	open:     key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open")),
	projects: key.NewBinding(key.WithKeys("ctrl+p"), key.WithHelp("ctrl+p", "projects")),
	quit:     key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),

	// close dismisses a popup.
	close:     key.NewBinding(key.WithKeys("esc", "q")),
	forceQuit: key.NewBinding(key.WithKeys("ctrl+d")),
	interrupt: key.NewBinding(key.WithKeys("ctrl+c")),
	confirm:   key.NewBinding(key.WithKeys("y", "Y")),
	deny:      key.NewBinding(key.WithKeys("n", "N", "esc")),
	accept:    key.NewBinding(key.WithKeys("enter")),

	navigate:  key.NewBinding(key.WithHelp("hjkl", "navigate")),
	topBottom: key.NewBinding(key.WithHelp("g/G", "top/bottom")),
}

// ShortHelp implements help.KeyMap.
func (k boardKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.navigate, k.topBottom, k.view, k.comments, k.move, k.yank, k.open, k.projects, k.quit}
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
