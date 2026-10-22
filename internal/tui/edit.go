package tui

import (
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

// editPopup edits an issue, with a tab for each kind of change.
type editPopup struct {
	issueKey   string
	tabs       []editTab
	currentTab int
	width      int
}

func newEditPopup(b Backend, iss jira.Issue, assignees []jira.Filter) *editPopup {
	move := newMovePane(b, iss.Key)
	assign := newAssignPane(b, iss, assignees)
	return &editPopup{
		issueKey: iss.Key,
		tabs: []editTab{
			{name: "Move", pane: &move},
			{name: "Assignee", pane: &assign},
		},
	}
}

func (p *editPopup) setSize(width, _ int) {
	p.width = min(width, 64)
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
	innerW := p.width - popupStyle.GetHorizontalFrameSize()
	names := make([]string, len(p.tabs))
	for i, t := range p.tabs {
		names[i] = t.name
	}
	lines := []string{titleStyle.Render("Edit · " + p.issueKey), renderTabs(names, p.currentTab), ""}
	lines = append(lines, p.tabs[p.currentTab].pane.view(spin, innerW, "tab switch · j/k select · enter apply · esc cancel")...)
	return renderPopup(innerW, lines)
}
