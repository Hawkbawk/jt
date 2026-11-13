// Package issuemove is the modal that moves an issue to another status. It
// lists the transitions the issue allows in a select and applies the chosen
// one. When the move succeeds it closes with a Moved message.
package issuemove

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/components/pane"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/keys"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
)

// Backend loads and applies an issue's transitions.
type Backend interface {
	Transitions(iss jira.Issue) ([]jira.Transition, error)
	Transition(key, transitionID string) error
}

// Moved is what the modal closes with after a move. It's a tuimsg.Finished.
type Moved struct {
	Key string
	To  string
}

// ForParent marks Moved for the modal host.
func (Moved) ForParent() {}

// ClosesModal marks Moved for the modal host.
func (Moved) ClosesModal() {}

var _ tuimsg.Finished = Moved{}

// IssueKey implements tuimsg.Finished.
func (m Moved) IssueKey() string { return m.Key }

// Status implements tuimsg.Finished.
func (m Moved) Status() string { return "Moved " + m.Key + " to " + m.To }

// keyMap is the modal's key bindings.
type keyMap struct {
	pane.KeyMap
	navigate, apply, search, stop key.Binding
}

func newKeyMap() keyMap {
	k := keys.Get()
	return keyMap{
		KeyMap:   pane.NewKeyMap(keys.Describe(k.Close, "cancel")),
		navigate: k.ListNavigate, apply: keys.Describe(k.Accept, "apply"),
		search: keys.Describe(k.ListFilter, "search"), stop: keys.Describe(k.Dismiss, "cancel search"),
	}
}

type item struct {
	transition jira.Transition
}

func (i item) Title() string       { return i.transition.Label() }
func (i item) Description() string { return "" }
func (i item) FilterValue() string { return i.transition.Label() }

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
	keys    keyMap
	issue   jira.Issue
	backend Backend
	spin    spinner.Model

	width, height int
	transitions   []jira.Transition
	list          list.Model
	loaded        bool
	applying      bool
	err           error
}

// New returns a modal that moves iss.
func New(b Backend, iss jira.Issue) *Model {
	return &Model{issue: iss, backend: b, spin: pane.NewSpinner(), keys: newKeyMap()}
}

// Init implements tea.Model.
func (p *Model) Init() tea.Cmd {
	return tea.Batch(loadTransitions(p.backend, p.issue), p.spin.Tick)
}

func (p *Model) loading() bool {
	return (!p.loaded && p.err == nil) || p.applying
}

func (p *Model) buildList() {
	items := make([]list.Item, len(p.transitions))
	for i, t := range p.transitions {
		items[i] = item{transition: t}
	}
	d := styles.ListDelegate()
	d.ShowDescription = false
	d.SetSpacing(0)
	l := list.New(items, d, p.width, p.listHeight())
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.KeyMap.Quit = key.NewBinding()
	l.KeyMap.ForceQuit = key.NewBinding()
	p.list = l
}

func (p *Model) listHeight() int { return max(1, p.height-pane.BodyLines) }

func (p *Model) hasList() bool { return len(p.transitions) > 0 }

func (p *Model) filtering() bool { return p.hasList() && p.list.FilterState() == list.Filtering }

// Update implements tea.Model.
func (p *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	was := p.loading()
	cmd := p.update(msg)
	return p, pane.Restart(p.spin, was, p.loading(), cmd)
}

func (p *Model) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width, p.height = msg.Width, msg.Height
		if p.hasList() {
			p.list.SetSize(p.width, p.listHeight())
		}
		return nil
	case spinner.TickMsg:
		return pane.Tick(&p.spin, msg, p.loading())
	case transitionsLoadedMsg:
		p.transitions, p.err, p.loaded = msg.transitions, msg.err, true
		if p.hasList() {
			p.buildList()
		}
		return nil
	case transitionDoneMsg:
		p.applying, p.err = false, msg.err
		if msg.err == nil {
			return tuimsg.Send(Moved{Key: msg.key, To: msg.to})
		}
		return nil
	case tea.KeyPressMsg:
		if p.filtering() && !key.Matches(msg, p.keys.apply) {
			return p.forward(msg)
		}
		switch {
		case key.Matches(msg, p.keys.Close()) && (!p.hasList() || !p.list.IsFiltered()):
			return tuimsg.Close()
		case key.Matches(msg, p.keys.apply) && p.hasList() && !p.applying:
			sel, ok := p.list.SelectedItem().(item)
			if !ok {
				return nil
			}
			return p.apply(sel.transition)
		}
	}
	return p.forward(msg)
}

func (p *Model) forward(msg tea.Msg) tea.Cmd {
	if !p.hasList() || p.applying {
		return nil
	}
	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return cmd
}

func (p *Model) apply(t jira.Transition) tea.Cmd {
	p.applying, p.err = true, nil
	b, issueKey := p.backend, p.issue.Key
	return func() tea.Msg {
		return transitionDoneMsg{key: issueKey, to: t.To, err: b.Transition(issueKey, t.ID)}
	}
}

// Title implements modal.Titled.
func (p *Model) Title() string { return pane.Title("Move", p.issue.Key) }

// Footer implements modal.Footed.
func (p *Model) Footer() (string, help.KeyMap) {
	switch {
	case p.filtering():
		return "", p.keys.WithLead(p.keys.stop, p.keys.apply).WithCloseShown(false)
	case p.hasList():
		return "", p.keys.WithLead(p.keys.navigate, p.keys.apply, p.keys.search)
	}
	return "", p.keys.KeyMap
}

// View implements tea.Model.
func (p *Model) View() tea.View {
	body := []string{""}
	switch {
	case !p.loaded && p.err == nil:
		body = append(body, p.spin.View()+" Loading transitions…")
	case !p.hasList() && p.err == nil:
		body = append(body, styles.Dim().Render("No transitions available."))
	case p.hasList():
		body = append(body, p.list.View())
	}
	body = append(body, "", pane.StatusLine(p.spin.View(), p.width, p.err, p.applying, "Moving…", ""))
	return tea.NewView(strings.Join(body, "\n"))
}
