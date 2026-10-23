package tui

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/hawkbawk/jt/internal/jira"
)

// pane is one tab of the edit popup.
type pane interface {
	update(tea.Msg) tea.Cmd
	view(spin string, innerW int, hint string) []string
	loading() bool
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
}

func newEditPopup(b Backend, iss jira.Issue, assignees []jira.Filter) *editPopup {
	move := newMovePane(b, iss.Key)
	assign := newAssignPane(b, iss, assignees)
	return &editPopup{
		help:     newHelp(),
		issueKey: iss.Key,
		tabs: []editTab{
			{name: "Move", pane: &move},
			{name: "Assignee", pane: &assign},
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
	switch {
	case key.Matches(km, keys.close):
		return nil, nil
	case key.Matches(km, keys.nextTab, keys.right):
		p.currentTab = (p.currentTab + 1) % len(p.tabs)
	case key.Matches(km, keys.prevTab, keys.left):
		p.currentTab = (p.currentTab + len(p.tabs) - 1) % len(p.tabs)
	default:
		return p, p.tabs[p.currentTab].pane.update(km)
	}
	return p, nil
}

func (p *editPopup) view(spin string) string {
	innerW, _ := popupInner(p.width, p.height)
	names := make([]string, len(p.tabs))
	for i, t := range p.tabs {
		names[i] = t.name
	}
	p.help.Width = innerW
	header := []string{titleStyle.Render("Edit · " + p.issueKey), renderTabs(names, p.currentTab), ""}
	body := p.tabs[p.currentTab].pane.view(spin, innerW, "")
	return renderPopup(p.width, p.height, header, body, p.help.View(editHelpKeys))
}
