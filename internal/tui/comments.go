package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

const commentInputHeight = 5

type commentSentMsg struct {
	key string
	err error
}

// commentsPopup shows an issue's comments above a box for writing a new one.
type commentsPopup struct {
	*textPopup
	backend Backend
	input   textarea.Model

	sending   bool
	sendErr   error
	jumpToEnd bool
}

func newCommentsPopup(b Backend, issueKey string, md markdownRenderer) *commentsPopup {
	ta := textarea.New()
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.Placeholder = "Write a comment…"
	ta.FocusedStyle.Placeholder = ta.FocusedStyle.Placeholder.Foreground(muted)
	ta.BlurredStyle.Placeholder = ta.BlurredStyle.Placeholder.Foreground(muted)
	ta.BlurredStyle.Text = ta.BlurredStyle.Text.Foreground(muted)
	ta.Cursor.SetMode(cursor.CursorStatic)
	ta.SetHeight(commentInputHeight)

	tp := newTextPopup("Comments", issueKey, renderComments, md)
	tp.reserved = commentInputHeight + inputBorder.GetVerticalFrameSize()
	return &commentsPopup{textPopup: tp, backend: b, input: ta}
}

func (p *commentsPopup) setSize(width, height int) {
	p.input.SetWidth(width - popupStyle.GetHorizontalFrameSize() - inputBorder.GetHorizontalFrameSize())
	p.textPopup.setSize(width, height)
}

func (p *commentsPopup) loading() bool {
	return p.textPopup.loading() || p.sending
}

func (p *commentsPopup) update(msg tea.Msg) (popup, tea.Cmd) {
	switch msg := msg.(type) {
	case issueLoadedMsg:
		_, cmd := p.textPopup.update(msg)
		if p.jumpToEnd && msg.key == p.key {
			p.jumpToEnd = false
			p.vp.GotoBottom()
		}
		return p, cmd
	case commentSentMsg:
		if msg.key != p.key {
			return p, nil
		}
		p.sending, p.sendErr = false, msg.err
		if msg.err != nil {
			return p, nil
		}
		p.input.Reset()
		p.input.Blur()
		p.jumpToEnd = true
		return p, loadIssue(p.backend, p.key)
	case tea.KeyMsg:
		if p.input.Focused() {
			return p.updateInput(msg)
		}
		if key.Matches(msg, keys.accept) && !p.textPopup.loading() {
			p.sendErr = nil
			return p, p.input.Focus()
		}
	}
	inner, cmd := p.textPopup.update(msg)
	if inner == nil {
		return nil, cmd
	}
	return p, cmd
}

func (p *commentsPopup) updateInput(msg tea.KeyMsg) (popup, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.blur):
		p.input.Blur()
		return p, nil
	case key.Matches(msg, keys.send):
		body := p.input.Value()
		if p.sending || strings.TrimSpace(body) == "" {
			return p, nil
		}
		p.sending, p.sendErr = true, nil
		b, issueKey := p.backend, p.key
		return p, func() tea.Msg {
			return commentSentMsg{key: issueKey, err: b.AddComment(issueKey, body)}
		}
	}
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return p, cmd
}

func (p *commentsPopup) view(spin string) string {
	border := inputBorder
	hint := "↑↓←→ scroll · enter write · esc close"
	if p.input.Focused() {
		border = border.BorderForeground(highlight)
		hint = "ctrl+s send · esc stop writing"
	}
	switch {
	case p.sending:
		hint = spin + " Sending…"
	case p.sendErr != nil:
		hint = errorStyle.Render("Error: " + p.sendErr.Error())
	}
	below := border.Render(p.input.View())
	return p.compose(spin, below, dimStyle.Render(hint))
}
