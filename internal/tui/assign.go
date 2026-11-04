package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/hawkbawk/jt/internal/components/usersearch"
	"github.com/hawkbawk/jt/internal/components/userselect"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/popup"
	"github.com/hawkbawk/jt/internal/tui/popup/pane"
)

type assignDoneMsg struct {
	id       int64
	issueKey string
	to       string
	// cleared is set when the issue was unassigned.
	cleared bool
	err     error
}

// assignPopup searches for the person to assign an issue to.
type assignPopup struct {
	id       int64
	issueKey string
	backend  Backend
	assignee jira.Option
	picker   *userselect.Picker
	form     *huh.Form
	spin     spinner.Model

	width, height int
	applying      bool
	err           error
}

// newAssignPopup starts on the issue's current assignee.
func newAssignPopup(b Backend, iss jira.Issue) *assignPopup {
	p := &assignPopup{id: pane.NewID(), issueKey: iss.Key, backend: b, assignee: iss.Assignee, spin: pane.NewSpinner()}
	p.picker = userselect.New("Assignee", searchUsers(b, iss.Key)).Value(&p.assignee)
	p.form = pane.NewForm(p.picker.Field())
	return p
}

// Init implements tea.Model.
func (p *assignPopup) Init() tea.Cmd { return tea.Batch(p.form.Init(), p.spin.Tick) }

func (p *assignPopup) loading() bool { return p.applying }

// Update implements tea.Model.
func (p *assignPopup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	was := p.loading()
	cmd := p.update(msg)
	return p, pane.Restart(p.spin, was, p.loading(), cmd)
}

func (p *assignPopup) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width, p.height = msg.Width, msg.Height
		return nil
	case spinner.TickMsg:
		return pane.Tick(&p.spin, msg, p.loading())
	case assignDoneMsg:
		if msg.id != p.id {
			return nil
		}
		p.applying, p.err = false, msg.err
		if msg.err == nil {
			status := "Assigned " + msg.issueKey + " to " + msg.to
			if msg.cleared {
				status = "Unassigned " + msg.issueKey
			}
			return closeEdited(msg.issueKey, status)
		}
		// The form finished to send the assignment, so a retry needs a new one.
		p.form = pane.NewForm(p.picker.Field())
		return p.form.Init()
	case tea.KeyPressMsg:
		// Typed text always goes to the form, so only esc closes.
		if key.Matches(msg, keys.blur) {
			return popup.Close()
		}
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
	b, id, issueKey := p.backend, p.id, p.issueKey
	return func() tea.Msg {
		return assignDoneMsg{id: id, issueKey: issueKey, to: who.Name, cleared: who.ID == "", err: b.Assign(issueKey, who.ID)}
	}
}

// View implements tea.Model.
func (p *assignPopup) View() tea.View {
	p.form.WithWidth(p.width).WithHeight(max(1, p.height-pane.ChromeLines-2))
	lines := strings.Split(p.form.View(), "\n")
	hint := styles.Dim().Render("enter next · shift+tab back to search · esc close")
	lines = append(lines, "", pane.StatusLine(p.spin.View(), p.width, p.err, p.applying, "Assigning…", hint))
	return tea.NewView(popup.Stack(p.height, pane.Header("Assign", p.issueKey), lines, helpView(pane.HelpKeys, p.width)))
}

// searchUsers looks up people who can be put on the issue.
func searchUsers(b Backend, issueKey string) usersearch.Func {
	return func(query string) ([]jira.Option, error) { return b.SearchUsers(issueKey, query) }
}
