package tui

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/popup"
	"github.com/hawkbawk/jt/internal/tui/popup/pane"
)

type transitionsLoadedMsg struct {
	id          int64
	key         string
	transitions []jira.Transition
	err         error
}

type transitionDoneMsg struct {
	id  int64
	key string
	to  string
	err error
}

func loadTransitions(b Backend, id int64, iss jira.Issue) tea.Cmd {
	return func() tea.Msg {
		ts, err := b.Transitions(iss)
		return transitionsLoadedMsg{id: id, key: iss.Key, transitions: ts, err: err}
	}
}

// movePopup lists the transitions available for an issue.
type movePopup struct {
	choiceList
	id      int64
	issue   jira.Issue
	backend Backend
	spin    spinner.Model

	width, height int
	transitions   []jira.Transition
	loaded        bool
	applying      bool
	err           error
}

func newMovePopup(b Backend, iss jira.Issue) *movePopup {
	return &movePopup{id: pane.NewID(), issue: iss, backend: b, spin: pane.NewSpinner()}
}

// Init implements tea.Model.
func (p *movePopup) Init() tea.Cmd {
	return tea.Batch(loadTransitions(p.backend, p.id, p.issue), p.spin.Tick)
}

func (p *movePopup) loading() bool {
	return (!p.loaded && p.err == nil) || p.applying
}

// Update implements tea.Model.
func (p *movePopup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	was := p.loading()
	cmd := p.update(msg)
	return p, pane.Restart(p.spin, was, p.loading(), cmd)
}

func (p *movePopup) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width, p.height = msg.Width, msg.Height
	case spinner.TickMsg:
		return pane.Tick(&p.spin, msg, p.loading())
	case transitionsLoadedMsg:
		if msg.id == p.id {
			p.transitions, p.err, p.loaded = msg.transitions, msg.err, true
		}
	case transitionDoneMsg:
		if msg.id != p.id {
			break
		}
		p.applying, p.err = false, msg.err
		if msg.err == nil {
			return closeEdited(msg.key, "Moved "+msg.key+" to "+msg.to)
		}
	case tea.KeyPressMsg:
		if key.Matches(msg, keys.close) {
			return popup.Close()
		}
		if p.handle(msg, len(p.transitions)) {
			break
		}
		if !key.Matches(msg, keys.accept) || p.applying || p.cursor >= len(p.transitions) {
			break
		}
		p.applying, p.err = true, nil
		t := p.transitions[p.cursor]
		b, id, issueKey := p.backend, p.id, p.issue.Key
		return func() tea.Msg {
			return transitionDoneMsg{id: id, key: issueKey, to: t.To, err: b.Transition(issueKey, t.ID)}
		}
	}
	return nil
}

// View implements tea.Model.
func (p *movePopup) View() tea.View {
	var lines []string
	switch {
	case !p.loaded && p.err == nil:
		lines = append(lines, p.spin.View()+" Loading transitions…")
	case p.loaded && len(p.transitions) == 0 && p.err == nil:
		lines = append(lines, styles.Dim().Render("No transitions available."))
	default:
		labels := make([]string, len(p.transitions))
		for i, t := range p.transitions {
			labels[i] = t.Label()
		}
		lines = append(lines, p.render(labels, p.width)...)
	}
	lines = append(lines, "", pane.StatusLine(p.spin.View(), p.width, p.err, p.applying, "Moving…", ""))
	return tea.NewView(popup.Stack(p.height, pane.Header("Move", p.issue.Key), lines, helpView(pane.HelpKeys, p.width)))
}
