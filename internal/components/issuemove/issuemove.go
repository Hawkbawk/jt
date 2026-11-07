// Package issuemove is the popup that moves an issue to another status. It
// lists the transitions the issue allows in a select and applies the chosen
// one. When the move succeeds it closes with a Moved message.
package issuemove

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/keys"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/layout"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
	"github.com/hawkbawk/jt/internal/tui/widget"
)

// Backend loads and applies an issue's transitions.
type Backend interface {
	Transitions(iss jira.Issue) ([]jira.Transition, error)
	Transition(key, transitionID string) error
}

// Moved is what the component ends with after a move. It's a tuimsg.Finished
// and a tuimsg.Final.
type Moved struct {
	Key string
	To  string
}

var _ tuimsg.Finished = Moved{}

// Final implements tuimsg.Final.
func (Moved) Final() {}

// IssueKey implements tuimsg.Finished.
func (m Moved) IssueKey() string { return m.Key }

// Status implements tuimsg.Finished.
func (m Moved) Status() string { return "Moved " + m.Key + " to " + m.To }

func newKeyMap() layout.KeyMap {
	return layout.NewKeyMap(keys.Describe(keys.Get().Close, "cancel"))
}

type transitionsLoadedMsg struct {
	key         string
	transitions []jira.Transition
	err         error
}

type transitionDoneMsg struct {
	key string
	to  string
	err error
}

func loadTransitions(b Backend, iss jira.Issue) tea.Cmd {
	return func() tea.Msg {
		ts, err := b.Transitions(iss)
		return transitionsLoadedMsg{key: iss.Key, transitions: ts, err: err}
	}
}

// Model lists the transitions available for an issue.
type Model struct {
	tuimsg.Requests
	keys    layout.KeyMap
	issue   jira.Issue
	backend Backend
	spin    widget.Spinner

	width, height int
	transitions   []jira.Transition
	choice        int
	selectField   *huh.Select[int]
	form          *huh.Form
	loaded        bool
	applying      bool
	err           error
}

// New returns a popup that moves iss.
func New(b Backend, iss jira.Issue) *Model {
	return &Model{issue: iss, backend: b, spin: widget.NewSpinner(), keys: newKeyMap()}
}

// Init implements tea.Model.
func (p *Model) Init() tea.Cmd {
	return tea.Batch(p.Request(loadTransitions(p.backend, p.issue)), p.spin.Sync(p.loading()))
}

func (p *Model) loading() bool {
	return (!p.loaded && p.err == nil) || p.applying
}

func (p *Model) buildForm() {
	opts := make([]huh.Option[int], len(p.transitions))
	for i, t := range p.transitions {
		opts[i] = huh.NewOption(t.Label(), i)
	}
	p.selectField = huh.NewSelect[int]().Title("Select a transition").Options(opts...).Value(&p.choice)
	p.form = widget.NewForm(p.selectField)
	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(key.WithDisabled())
	km.Select.Filter.SetHelp("/", "search")
	km.Select.SetFilter.SetHelp("esc", "stop search")
	km.Select.ClearFilter.SetHelp("esc", "clear search")
	km.Select.Submit.SetHelp("enter", "apply")
	p.form.WithKeyMap(km)
}

// Update implements tea.Model.
func (p *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := p.update(msg)
	return p, tea.Batch(cmd, p.spin.Sync(p.loading()))
}

func (p *Model) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width, p.height = msg.Width, msg.Height
		return nil
	case spinner.TickMsg:
		return p.spin.Update(msg, p.loading())
	case transitionsLoadedMsg:
		p.transitions, p.err, p.loaded = msg.transitions, msg.err, true
		if len(p.transitions) == 0 {
			return nil
		}
		p.buildForm()
		return p.form.Init()
	case transitionDoneMsg:
		p.applying, p.err = false, msg.err
		if msg.err == nil {
			return tuimsg.End(Moved{Key: msg.key, To: msg.to})
		}
		// The form finished to send the move, so a retry needs a new one.
		p.buildForm()
		return p.form.Init()
	case tea.KeyPressMsg:
		if p.form == nil || (!p.consumesEsc() && key.Matches(msg, p.keys.Dismiss())) {
			if key.Matches(msg, p.keys.Dismiss()) {
				return tuimsg.Cancel()
			}
			return nil
		}
	}
	if p.form == nil || p.applying {
		return nil
	}
	_, cmd := p.form.Update(msg)
	if p.form.State != huh.StateCompleted {
		return cmd
	}
	p.applying, p.err = true, nil
	t := p.transitions[p.choice]
	b, issueKey := p.backend, p.issue.Key
	return p.Request(func() tea.Msg {
		return transitionDoneMsg{key: issueKey, to: t.To, err: b.Transition(issueKey, t.ID)}
	})
}

func (p *Model) consumesEsc() bool {
	return p.selectField != nil && key.Matches(tea.KeyPressMsg{Code: tea.KeyEsc}, p.selectField.KeyBinds()...)
}

// View implements tea.Model.
func (p *Model) View() tea.View {
	var body []string
	km := p.keys
	switch {
	case !p.loaded && p.err == nil:
		body = append(body, p.spin.View()+" Loading transitions…")
	case p.form == nil && p.err == nil:
		body = append(body, styles.Dim().Render("No transitions available."))
	case p.form != nil:
		p.form.WithWidth(p.width).WithHeight(max(1, p.height-layout.ChromeLines-2))
		body = strings.Split(p.form.View(), "\n")
		km = km.WithForm(p.selectField.KeyBinds(), p.consumesEsc())
	}
	body = append(body, "", layout.StatusLine(p.spin.View(), p.width, p.err, p.applying, "Moving…", ""))
	return tea.NewView(layout.Stack(p.height, layout.Header("Move", p.issue.Key), body, p.footer(km)))
}

func (p *Model) footer(km layout.KeyMap) string {
	h := styles.Help()
	h.SetWidth(max(0, p.width))
	return h.View(km)
}
