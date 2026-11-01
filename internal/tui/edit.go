package tui

import (
	"fmt"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/styles"
)

// pane is the content of a pane popup.
type pane interface {
	update(tea.Msg) tea.Cmd
	view(spin string, innerW, height int, hint string) []string
	loading() bool
}

// capturer is a pane that takes typed text, so only esc closes it.
type capturer interface {
	capturing() bool
}

// escaper is a pane that uses esc to step back before it closes.
type escaper interface {
	consumesEsc() bool
}

// holder is a pane that can't be saved while one of its fields is open.
type holder interface{ holding() bool }

// stager is a pane that holds changes until they're saved with ctrl+s.
type stager interface {
	unsaved() int
}

// paneKeys is the help footer for pane popups. The bindings only carry help
// text; the matching happens against the shared keys.
type paneKeys struct {
	choose, apply, cancel key.Binding
}

var paneHelpKeys = paneKeys{
	choose: key.NewBinding(key.WithHelp("j/k", "select")),
	apply:  key.NewBinding(key.WithHelp("enter", "apply")),
	cancel: key.NewBinding(key.WithHelp("esc", "cancel")),
}

// ShortHelp implements help.KeyMap.
func (k paneKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.choose, k.apply, k.cancel}
}

// FullHelp implements help.KeyMap.
func (k paneKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}

// panePopup shows one pane for an issue: moving it, assigning it or editing
// its fields.
type panePopup struct {
	title    string
	issueKey string
	pane     pane
	width    int
	height   int
	help     help.Model
	// confirmClose is set while asking whether to drop unsaved changes.
	confirmClose bool
}

func newPanePopup(title, issueKey string, p pane) *panePopup {
	return &panePopup{title: title, issueKey: issueKey, pane: p, help: styles.Help()}
}

func newMovePopup(b Backend, iss jira.Issue) *panePopup {
	p := newMovePane(b, iss.Key)
	return newPanePopup("Move", iss.Key, &p)
}

func newAssignPopup(p *assignPane) *panePopup {
	return newPanePopup("Assign", p.issueKey, p)
}

func newEditPopup(b Backend, iss jira.Issue) *panePopup {
	p := newFieldsPane(b, iss.Key)
	return newPanePopup("Edit", iss.Key, &p)
}

func (p *panePopup) setSize(width, height int) {
	p.width, p.height = width, height
}

func (p *panePopup) loading() bool {
	return p.pane.loading()
}

func (p *panePopup) update(msg tea.Msg) (popup, tea.Cmd) {
	km, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return p, p.pane.update(msg)
	}
	if p.confirmClose {
		return p.updateConfirm(km)
	}
	closes := keys.close
	if c, ok := p.pane.(capturer); ok && c.capturing() {
		closes = keys.blur
	}
	if e, ok := p.pane.(escaper); ok && e.consumesEsc() && key.Matches(km, keys.blur) {
		return p, p.pane.update(km)
	}
	if key.Matches(km, closes) {
		if p.unsaved() > 0 {
			p.confirmClose = true
			return p, nil
		}
		return nil, nil
	}
	return p, p.pane.update(km)
}

func (p *panePopup) updateConfirm(km tea.KeyPressMsg) (popup, tea.Cmd) {
	switch {
	case key.Matches(km, keys.confirm):
		return nil, nil
	case key.Matches(km, keys.deny):
		p.confirmClose = false
	case key.Matches(km, keys.send):
		p.confirmClose = false
		return p, p.pane.update(km)
	}
	return p, nil
}

func (p *panePopup) unsaved() int {
	if s, ok := p.pane.(stager); ok {
		return s.unsaved()
	}
	return 0
}

func (p *panePopup) view(spin string) string {
	innerW, innerH := popupInner(p.width, p.height)
	p.help.SetWidth(innerW)
	title := styles.Title().Render(p.title + " · " + p.issueKey)
	footer := p.help.View(paneHelpKeys)
	if n := p.unsaved(); n > 0 {
		changes := fmt.Sprintf("%d unsaved %s", n, plural(n, "change"))
		warn := "● " + changes
		if h, ok := p.pane.(holder); !ok || !h.holding() {
			warn += " · ctrl+s to save"
		}
		title += "  " + styles.Warn().Render(warn)
		if p.confirmClose {
			footer = styles.Warn().Render("Discard " + changes + "? y discard · n keep editing · ctrl+s save")
		}
	}
	header := []string{title, ""}
	// The footer takes the last line.
	body := p.pane.view(spin, innerW, innerH-len(header)-1, "")
	return renderPopup(p.width, p.height, header, body, footer)
}
