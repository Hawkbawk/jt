// Package issueassign is the popup that assigns an issue to someone. It
// searches for people and closes with an Assigned message once applied.
package issueassign

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/hawkbawk/jt/internal/components/usersearch"
	"github.com/hawkbawk/jt/internal/components/userselect"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/keys"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/layout"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
	"github.com/hawkbawk/jt/internal/tui/widget"
)

// Backend searches for people and assigns the issue to one.
type Backend interface {
	SearchUsers(issueKey, query string) ([]jira.Option, error)
	Assign(key, assigneeID string) error
}

// Assigned is what the component ends with after an assignment. It's a
// tuimsg.Finished.
type Assigned struct {
	Key string
	To  string
	// Cleared is set when the issue was unassigned.
	Cleared bool
}

var _ tuimsg.Finished = Assigned{}

// Final implements tuimsg.Final.
func (Assigned) Final() {}

// IssueKey implements tuimsg.Finished.
func (a Assigned) IssueKey() string { return a.Key }

// Status implements tuimsg.Finished.
func (a Assigned) Status() string {
	if a.Cleared {
		return "Unassigned " + a.Key
	}
	return "Assigned " + a.Key + " to " + a.To
}

func newKeyMap() layout.KeyMap {
	return layout.NewKeyMap(keys.Describe(keys.Get().Dismiss, "cancel"))
}

type assignDoneMsg struct {
	issueKey string
	to       string
	// cleared is set when the issue was unassigned.
	cleared bool
	err     error
}

// Model searches for the person to assign an issue to.
type Model struct {
	tuimsg.Requests
	keys     layout.KeyMap
	issueKey string
	backend  Backend
	assignee jira.Option
	picker   *userselect.Picker
	form     *huh.Form
	spin     widget.Spinner

	width, height int
	applying      bool
	err           error
}

// New starts on the issue's current assignee.
func New(b Backend, iss jira.Issue) *Model {
	p := &Model{issueKey: iss.Key, backend: b, assignee: iss.Assignee, spin: widget.NewSpinner(), keys: newKeyMap()}
	p.picker = userselect.New("Assignee", searchUsers(b, iss.Key)).Value(&p.assignee)
	p.form = widget.NewForm(p.picker.Field())
	return p
}

// Init implements tea.Model.
func (p *Model) Init() tea.Cmd { return p.form.Init() }

// Update implements tea.Model.
func (p *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width, p.height = msg.Width, msg.Height
		return p, nil
	case spinner.TickMsg:
		return p, p.spin.Update(msg, p.applying)
	case assignDoneMsg:
		p.applying, p.err = false, msg.err
		if msg.err == nil {
			return p, tuimsg.End(Assigned{Key: msg.issueKey, To: msg.to, Cleared: msg.cleared})
		}
		// The form finished to send the assignment, so a retry needs a new one.
		p.form = widget.NewForm(p.picker.Field())
		return p, p.form.Init()
	case tea.KeyPressMsg:
		// Typed text always goes to the form, so only esc closes.
		if key.Matches(msg, p.keys.Dismiss()) {
			return p, tuimsg.Cancel()
		}
	}
	if p.applying {
		return p, nil
	}
	_, cmd := p.form.Update(msg)
	if p.form.State != huh.StateCompleted {
		return p, cmd
	}
	p.applying, p.err = true, nil
	who := p.assignee
	b, issueKey := p.backend, p.issueKey
	return p, tea.Batch(p.spin.Sync(true), p.Request(func() tea.Msg {
		return assignDoneMsg{issueKey: issueKey, to: who.Name, cleared: who.ID == "", err: b.Assign(issueKey, who.ID)}
	}))
}

// View implements tea.Model.
func (p *Model) View() tea.View {
	p.form.WithWidth(p.width).WithHeight(max(1, p.height-layout.ChromeLines-2))
	lines := strings.Split(p.form.View(), "\n")
	lines = append(lines, "", layout.StatusLine(p.spin.View(), p.width, p.err, p.applying, "Assigning…", ""))
	return tea.NewView(layout.Stack(p.height, layout.Header("Assign", p.issueKey), lines, p.footer()))
}

func (p *Model) footer() string {
	h := styles.Help()
	h.SetWidth(max(0, p.width))
	km := p.keys.WithForm(p.form.KeyBinds(), false)
	return h.View(km)
}

// searchUsers looks up people who can be put on the issue.
func searchUsers(b Backend, issueKey string) usersearch.Func {
	return func(query string) ([]jira.Option, error) { return b.SearchUsers(issueKey, query) }
}
