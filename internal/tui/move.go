package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/hawkbawk/jt/internal/jira"
)

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

func loadTransitions(b Backend, issueKey string) tea.Cmd {
	return func() tea.Msg {
		ts, err := b.Transitions(issueKey)
		return transitionsLoadedMsg{key: issueKey, transitions: ts, err: err}
	}
}

// movePane lists the transitions available for an issue. It's shared by the
// move popup and the edit popup.
type movePane struct {
	choiceList
	key     string
	backend Backend

	transitions []jira.Transition
	loaded      bool
	applying    bool
	err         error
}

func newMovePane(b Backend, issueKey string) movePane {
	return movePane{key: issueKey, backend: b}
}

func (p *movePane) loading() bool {
	return (!p.loaded && p.err == nil) || p.applying
}

func (p *movePane) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case transitionsLoadedMsg:
		if msg.key == p.key {
			p.transitions, p.err, p.loaded = msg.transitions, msg.err, true
		}
	case transitionDoneMsg:
		if msg.key == p.key {
			p.applying, p.err = false, msg.err
		}
	case tea.KeyMsg:
		if p.handle(msg, len(p.transitions)) {
			break
		}
		if !key.Matches(msg, keys.accept) || p.applying || p.cursor >= len(p.transitions) {
			break
		}
		p.applying, p.err = true, nil
		t := p.transitions[p.cursor]
		b, issueKey := p.backend, p.key
		return func() tea.Msg {
			return transitionDoneMsg{key: issueKey, to: t.To, err: b.Transition(issueKey, t.ID)}
		}
	}
	return nil
}

func (p *movePane) view(spin string, innerW int, hint string) []string {
	var lines []string
	switch {
	case !p.loaded && p.err == nil:
		lines = append(lines, spin+" Loading transitions…")
	case p.loaded && len(p.transitions) == 0 && p.err == nil:
		lines = append(lines, dimStyle.Render("No transitions available."))
	default:
		labels := make([]string, len(p.transitions))
		for i, t := range p.transitions {
			labels[i] = t.Label()
		}
		lines = append(lines, p.render(labels, innerW)...)
	}
	return append(lines, "", statusLine(spin, innerW, p.err, p.applying, "Moving…", hint))
}

// statusLine shows the error, the in-progress text or the hint, in that order.
func statusLine(spin string, innerW int, err error, applying bool, progress, hint string) string {
	switch {
	case err != nil:
		return errorStyle.Width(innerW).Render("Error: " + err.Error())
	case applying:
		return spin + " " + progress
	}
	return dimStyle.Render(hint)
}

// movePopup is the standalone move menu.
type movePopup struct {
	movePane
	width int
}

func newMovePopup(b Backend, issueKey string) *movePopup {
	return &movePopup{movePane: newMovePane(b, issueKey)}
}

func (p *movePopup) setSize(width, _ int) {
	p.width = min(width, 64)
}

func (p *movePopup) update(msg tea.Msg) (popup, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok && key.Matches(km, keys.close) {
		return nil, nil
	}
	return p, p.movePane.update(msg)
}

func (p *movePopup) view(spin string) string {
	innerW := p.width - popupStyle.GetHorizontalFrameSize()
	lines := append([]string{titleStyle.Render("Move · " + p.key), ""},
		p.movePane.view(spin, innerW, "j/k select · enter move · esc cancel")...)
	return renderPopup(innerW, lines)
}
