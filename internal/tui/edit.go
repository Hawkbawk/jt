package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/hawkbawk/jt/internal/jira"
)

// pane is one tab of the edit popup.
type pane interface {
	update(tea.Msg) tea.Cmd
	view(spin string, innerW, height int, hint string) []string
	loading() bool
}

// capturer is a pane that can take over every key, such as while text is
// being typed into it.
type capturer interface {
	capturing() bool
}

// stager is a pane that holds changes until they're saved with ctrl+s.
type stager interface {
	unsaved() int
}

type editTab struct {
	name string
	pane pane
}

// editKeys is the help footer for the edit popup. The bindings only carry help
// text; the matching happens against the shared keys.
type editKeys struct {
	tabs, choose, apply, cancel key.Binding
}

var editHelpKeys = editKeys{
	tabs:   key.NewBinding(key.WithHelp("h/l", "switch tab")),
	choose: key.NewBinding(key.WithHelp("j/k", "select")),
	apply:  key.NewBinding(key.WithHelp("enter", "apply")),
	cancel: key.NewBinding(key.WithHelp("esc", "cancel")),
}

// ShortHelp implements help.KeyMap.
func (k editKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.tabs, k.choose, k.apply, k.cancel}
}

// FullHelp implements help.KeyMap.
func (k editKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}

// editPopup edits an issue, with a tab for each kind of change.
type editPopup struct {
	issueKey   string
	tabs       []editTab
	currentTab int
	width      int
	height     int
	help       help.Model
	// confirmClose is set while asking whether to drop unsaved changes.
	confirmClose bool
}

func newEditPopup(b Backend, iss jira.Issue, assignees []jira.Filter) *editPopup {
	move := newMovePane(b, iss.Key)
	assign := newAssignPane(b, iss, assignees)
	fields := newFieldsPane(b, iss.Key)
	return &editPopup{
		help:     newHelp(),
		issueKey: iss.Key,
		tabs: []editTab{
			{name: "Move", pane: &move},
			{name: "Assignee", pane: &assign},
			{name: "Fields", pane: &fields},
		},
	}
}

func (p *editPopup) setSize(width, height int) {
	p.width, p.height = width, height
}

func (p *editPopup) loading() bool {
	for _, t := range p.tabs {
		if t.pane.loading() {
			return true
		}
	}
	return false
}

func (p *editPopup) update(msg tea.Msg) (popup, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		cmds := make([]tea.Cmd, len(p.tabs))
		for i, t := range p.tabs {
			cmds[i] = t.pane.update(msg)
		}
		return p, tea.Batch(cmds...)
	}
	if p.confirmClose {
		return p.updateConfirm(km)
	}
	current := p.tabs[p.currentTab].pane
	if c, ok := current.(capturer); ok && c.capturing() {
		return p, current.update(km)
	}
	switch {
	case key.Matches(km, keys.close):
		if p.unsaved() > 0 {
			p.confirmClose = true
			return p, nil
		}
		return nil, nil
	case key.Matches(km, keys.send):
		return p, p.save(km)
	case key.Matches(km, keys.nextTab, keys.right):
		p.currentTab = (p.currentTab + 1) % len(p.tabs)
	case key.Matches(km, keys.prevTab, keys.left):
		p.currentTab = (p.currentTab + len(p.tabs) - 1) % len(p.tabs)
	default:
		return p, current.update(km)
	}
	return p, nil
}

func (p *editPopup) updateConfirm(km tea.KeyMsg) (popup, tea.Cmd) {
	switch {
	case key.Matches(km, keys.confirm):
		return nil, nil
	case key.Matches(km, keys.deny):
		p.confirmClose = false
	case key.Matches(km, keys.send):
		p.confirmClose = false
		return p, p.save(km)
	}
	return p, nil
}

// save sends ctrl+s to every pane with unsaved changes, whichever tab is
// showing.
func (p *editPopup) save(km tea.KeyMsg) tea.Cmd {
	var cmds []tea.Cmd
	for _, t := range p.tabs {
		if s, ok := t.pane.(stager); ok && s.unsaved() > 0 {
			cmds = append(cmds, t.pane.update(km))
		}
	}
	return tea.Batch(cmds...)
}

func (p *editPopup) unsaved() int {
	n := 0
	for _, t := range p.tabs {
		if s, ok := t.pane.(stager); ok {
			n += s.unsaved()
		}
	}
	return n
}

func (p *editPopup) view(spin string) string {
	innerW, innerH := popupInner(p.width, p.height)
	names := make([]string, len(p.tabs))
	for i, t := range p.tabs {
		names[i] = t.name
	}
	p.help.Width = innerW
	title := titleStyle.Render("Edit · " + p.issueKey)
	footer := p.help.View(editHelpKeys)
	if n := p.unsaved(); n > 0 {
		changes := fmt.Sprintf("%d unsaved %s", n, plural(n, "change"))
		title += "  " + warnStyle.Render("● "+changes+" · ctrl+s to save")
		if p.confirmClose {
			footer = warnStyle.Render("Discard " + changes + "? y discard · n keep editing · ctrl+s save")
		}
	}
	header := []string{title, renderTabs(names, p.currentTab), ""}
	// The footer takes the last line.
	body := p.tabs[p.currentTab].pane.view(spin, innerW, innerH-len(header)-1, "")
	return renderPopup(p.width, p.height, header, body, footer)
}
