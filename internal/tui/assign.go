package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/hawkbawk/jt/internal/jira"
)

type assignDoneMsg struct {
	issueKey string
	to       string
	// cleared is set when the issue was unassigned.
	cleared bool
	err     error
}

// assignPane lists the board's assignees, including unassigned.
type assignPane struct {
	choiceList
	issueKey string
	backend  Backend
	people   []jira.Filter

	applying bool
	err      error
}

// newAssignPane starts the cursor on the issue's current assignee.
func newAssignPane(b Backend, iss jira.Issue, people []jira.Filter) assignPane {
	p := assignPane{issueKey: iss.Key, backend: b, people: people}
	for i, f := range people {
		if f.AssigneeID == iss.AssigneeID {
			p.cursor = i
			break
		}
	}
	return p
}

func (p *assignPane) loading() bool { return p.applying }

func (p *assignPane) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case assignDoneMsg:
		if msg.issueKey == p.issueKey {
			p.applying, p.err = false, msg.err
		}
	case tea.KeyMsg:
		if p.handle(msg, len(p.people)) {
			break
		}
		if !key.Matches(msg, keys.accept) || p.applying || p.cursor >= len(p.people) {
			break
		}
		p.applying, p.err = true, nil
		who := p.people[p.cursor]
		b, issueKey := p.backend, p.issueKey
		return func() tea.Msg {
			return assignDoneMsg{issueKey: issueKey, to: who.Name, cleared: who.AssigneeID == "", err: b.Assign(issueKey, who.AssigneeID)}
		}
	}
	return nil
}

func (p *assignPane) view(spin string, innerW int, hint string) []string {
	var lines []string
	if len(p.people) == 0 {
		lines = append(lines, dimStyle.Render("No assignees on this board."))
	} else {
		lines = append(lines, p.render(filterNames(p.people), innerW)...)
	}
	return append(lines, "", statusLine(spin, innerW, p.err, p.applying, "Assigning…", hint))
}
