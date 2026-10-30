package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/hawkbawk/jt/internal/components/userselect"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/styles"
)

type assignDoneMsg struct {
	issueKey string
	to       string
	// cleared is set when the issue was unassigned.
	cleared bool
	err     error
}

// assignPane searches for the person to assign an issue to.
type assignPane struct {
	issueKey string
	backend  Backend
	assignee jira.Option
	picker   *userselect.Picker
	form     *huh.Form

	applying bool
	err      error
}

// newAssignPane starts on the issue's current assignee.
func newAssignPane(b Backend, iss jira.Issue) *assignPane {
	p := &assignPane{issueKey: iss.Key, backend: b, assignee: iss.Assignee}
	p.picker = userselect.New("Assignee", searchUsers(b, iss.Key)).Value(&p.assignee)
	p.form = newForm(p.picker.Field())
	return p
}

// init starts the form. The popup is opened with it as its load.
func (p *assignPane) init() tea.Cmd { return p.form.Init() }

func (p *assignPane) loading() bool { return p.applying }

// capturing is true since typed text always goes to the form.
func (p *assignPane) capturing() bool { return true }

func (p *assignPane) update(msg tea.Msg) tea.Cmd {
	if msg, ok := msg.(assignDoneMsg); ok {
		if msg.issueKey == p.issueKey {
			p.applying, p.err = false, msg.err
			// The form finished to send the assignment, so a retry needs a new one.
			p.form = newForm(p.picker.Field())
			return p.form.Init()
		}
		return nil
	}
	if p.applying {
		return nil
	}
	_, cmd := p.form.Update(msg)
	if p.form.State != huh.StateCompleted {
		return cmd
	}
	p.applying, p.err = true, nil
	who := p.assignee
	b, issueKey := p.backend, p.issueKey
	return func() tea.Msg {
		return assignDoneMsg{issueKey: issueKey, to: who.Name, cleared: who.ID == "", err: b.Assign(issueKey, who.ID)}
	}
}

func (p *assignPane) view(spin string, innerW, height int, _ string) []string {
	p.form.WithWidth(innerW).WithHeight(max(1, height-2))
	lines := strings.Split(p.form.View(), "\n")
	hint := styles.Dim().Render("enter next · shift+tab back to search · esc close")
	return append(lines, "", statusLine(spin, innerW, p.err, p.applying, "Assigning…", hint))
}
