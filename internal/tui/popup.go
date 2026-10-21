package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/ryanhawkins/jt/internal/jira"
)

// popup is a modal shown over the board. update returns nil to close it.
type popup interface {
	update(tea.Msg) (popup, tea.Cmd)
	view(spin string) string
	setSize(width, height int)
}

type issueLoadedMsg struct {
	key    string
	detail *jira.IssueDetail
	err    error
}

func loadIssue(b Backend, key string) tea.Cmd {
	return func() tea.Msg {
		d, err := b.Issue(key)
		return issueLoadedMsg{key: key, detail: d, err: err}
	}
}

// issueRenderer turns a loaded issue into popup content for a given width.
type issueRenderer func(d *jira.IssueDetail, width int, md markdownRenderer) string

// textPopup shows scrollable, issue-derived content.
type textPopup struct {
	title  string
	key    string
	render issueRenderer
	md     markdownRenderer

	detail *jira.IssueDetail
	err    error
	vp     viewport.Model
	width  int
	height int
}

func newTextPopup(title, key string, render issueRenderer, md markdownRenderer) *textPopup {
	vp := viewport.New(0, 0)
	vp.SetHorizontalStep(4)
	return &textPopup{title: title, key: key, render: render, md: md, vp: vp}
}

func (p *textPopup) setSize(width, height int) {
	p.width, p.height = width, height
	p.vp.Width = width - popupStyle.GetHorizontalFrameSize()
	// Title and hint lines sit above and below the viewport.
	p.vp.Height = max(1, height-popupStyle.GetVerticalFrameSize()-2)
	p.refresh()
}

func (p *textPopup) refresh() {
	if p.detail != nil {
		p.vp.SetContent(p.render(p.detail, p.vp.Width, p.md))
	}
}

func (p *textPopup) update(msg tea.Msg) (popup, tea.Cmd) {
	switch msg := msg.(type) {
	case issueLoadedMsg:
		if msg.key != p.key {
			return p, nil
		}
		p.detail, p.err = msg.detail, msg.err
		p.refresh()
		return p, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			return nil, nil
		}
	}
	var cmd tea.Cmd
	p.vp, cmd = p.vp.Update(msg)
	return p, cmd
}

func (p *textPopup) loading() bool {
	return p.detail == nil && p.err == nil
}

func (p *textPopup) view(spin string) string {
	innerW := p.width - popupStyle.GetHorizontalFrameSize()
	title := titleStyle.Render(p.title + " · " + p.key)

	var body, hint string
	switch {
	case p.err != nil:
		body = errorStyle.Render("Error: " + p.err.Error())
	case p.loading():
		body = spin + " Loading…"
	default:
		body = p.vp.View()
		hint = fmt.Sprintf("%3.f%% · ↑↓←→ scroll · esc close", p.vp.ScrollPercent()*100)
	}
	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		lipgloss.NewStyle().Height(p.vp.Height).MaxHeight(p.vp.Height).Render(body),
		dimStyle.Render(hint),
	)
	return popupStyle.Width(innerW + popupStyle.GetHorizontalPadding()).Render(content)
}

// markdownRenderer renders markdown wrapped to width.
type markdownRenderer func(md string, width int) string

func glamourRenderer(style string) markdownRenderer {
	return func(md string, width int) string {
		r, err := glamour.NewTermRenderer(
			glamour.WithStandardStyle(style),
			glamour.WithWordWrap(width),
		)
		if err != nil {
			return md
		}
		out, err := r.Render(md)
		if err != nil {
			return md
		}
		return out
	}
}
