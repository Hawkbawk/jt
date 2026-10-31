package tui

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/styles"
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

func loadIssue(b Backend, issueKey string) tea.Cmd {
	return func() tea.Msg {
		d, err := b.Issue(issueKey)
		return issueLoadedMsg{key: issueKey, detail: d, err: err}
	}
}

// markdownRenderer renders markdown wrapped to width.
type markdownRenderer func(md string, width int) string

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

	// reserved is the number of lines an embedding popup draws below the
	// viewport.
	reserved int
}

func newTextPopup(title, issueKey string, render issueRenderer, md markdownRenderer) *textPopup {
	vp := viewport.New()
	vp.SetHorizontalStep(4)
	return &textPopup{title: title, key: issueKey, render: render, md: md, vp: vp}
}

func (p *textPopup) setSize(width, height int) {
	p.width, p.height = width, height
	p.vp.SetWidth(width - styles.Popup().GetHorizontalFrameSize())
	// Title and hint lines sit above and below the viewport.
	p.vp.SetHeight(max(1, height-styles.Popup().GetVerticalFrameSize()-2-p.reserved))
	p.refresh()
}

func (p *textPopup) refresh() {
	if p.detail != nil {
		p.vp.SetContent(p.render(p.detail, p.vp.Width(), p.md))
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
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, keys.close):
			return nil, nil
		case key.Matches(msg, keys.top):
			p.vp.GotoTop()
			return p, nil
		case key.Matches(msg, keys.bottom):
			p.vp.GotoBottom()
			return p, nil
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
	return p.compose(spin, "", styles.Dim().Render(fmt.Sprintf("%3.f%% · ↑↓←→ scroll · g/G top/bottom · esc close", p.vp.ScrollPercent()*100)))
}

// compose lays out the title, content, optional below block and hint. The hint
// only shows once the content has loaded.
func (p *textPopup) compose(spin, below, hint string) string {
	innerW := p.width - styles.Popup().GetHorizontalFrameSize()
	title := styles.Title().Render(p.title + " · " + p.key)

	var body string
	switch {
	case p.err != nil:
		body = styles.Error().Render("Error: " + p.err.Error())
		hint = ""
	case p.loading():
		body = spin + " Loading…"
		hint = ""
	default:
		body = p.vp.View()
	}
	parts := []string{title, lipgloss.NewStyle().Height(p.vp.Height()).MaxHeight(p.vp.Height()).Render(body)}
	if below != "" {
		parts = append(parts, below)
	}
	parts = append(parts, hint)
	return styles.Popup().Width(innerW + styles.Popup().GetHorizontalFrameSize()).Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
}
