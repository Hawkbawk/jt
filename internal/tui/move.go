package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

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

func loadTransitions(b Backend, key string) tea.Cmd {
	return func() tea.Msg {
		ts, err := b.Transitions(key)
		return transitionsLoadedMsg{key: key, transitions: ts, err: err}
	}
}

// movePopup lists the transitions available for an issue.
type movePopup struct {
	key     string
	backend Backend

	transitions []jira.Transition
	loaded      bool
	applying    bool
	err         error
	cursor      int
	width       int
}

func newMovePopup(b Backend, key string) *movePopup {
	return &movePopup{key: key, backend: b}
}

func (p *movePopup) setSize(width, _ int) {
	p.width = min(width, 64)
}

func (p *movePopup) loading() bool {
	return (!p.loaded && p.err == nil) || p.applying
}

func (p *movePopup) update(msg tea.Msg) (popup, tea.Cmd) {
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
		switch msg.String() {
		case "esc", "q":
			return nil, nil
		case "j", "down":
			if p.cursor < len(p.transitions)-1 {
				p.cursor++
			}
		case "k", "up":
			if p.cursor > 0 {
				p.cursor--
			}
		case "enter":
			if p.applying || p.cursor >= len(p.transitions) {
				break
			}
			p.applying, p.err = true, nil
			t := p.transitions[p.cursor]
			b, key := p.backend, p.key
			return p, func() tea.Msg {
				return transitionDoneMsg{key: key, to: t.To, err: b.Transition(key, t.ID)}
			}
		}
	}
	return p, nil
}

func (p *movePopup) view(spin string) string {
	innerW := p.width - popupStyle.GetHorizontalFrameSize()
	lines := []string{titleStyle.Render("Move · " + p.key), ""}

	switch {
	case !p.loaded && p.err == nil:
		lines = append(lines, spin+" Loading transitions…")
	case p.loaded && len(p.transitions) == 0 && p.err == nil:
		lines = append(lines, dimStyle.Render("No transitions available."))
	default:
		for i, t := range p.transitions {
			label := ansi.Truncate(t.Label(), innerW-2, "…")
			if i == p.cursor {
				lines = append(lines, lipgloss.NewStyle().Foreground(highlight).Bold(true).Render("› "+label))
			} else {
				lines = append(lines, "  "+label)
			}
		}
	}

	lines = append(lines, "")
	switch {
	case p.err != nil:
		lines = append(lines, errorStyle.Width(innerW).Render("Error: "+p.err.Error()))
	case p.applying:
		lines = append(lines, spin+" Moving…")
	default:
		lines = append(lines, dimStyle.Render("j/k select · enter move · esc cancel"))
	}
	return popupStyle.Width(innerW + popupStyle.GetHorizontalPadding()).Render(strings.Join(lines, "\n"))
}
