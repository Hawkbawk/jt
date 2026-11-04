package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/popup"
	"github.com/hawkbawk/jt/internal/tui/popup/issueview"
	"github.com/hawkbawk/jt/internal/tui/popup/pane"
)

const commentInputHeight = 5

type commentSentMsg struct {
	id  int64
	key string
	err error
}

// commentsPopup shows an issue's comments above a box for writing a new one.
type commentsPopup struct {
	id       int64
	backend  Backend
	issueKey string
	issue    *issueview.Model
	input    textarea.Model

	sending bool
	sendErr error
}

func newCommentsPopup(b Backend, issueKey string, md markdownRenderer) *commentsPopup {
	ta := textarea.New()
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.Placeholder = "Write a comment…"
	ta.SetStyles(styles.TextArea())
	ta.SetHeight(commentInputHeight)

	return &commentsPopup{
		id: pane.NewID(), backend: b, issueKey: issueKey,
		issue: issueview.Comments(b, issueKey, md), input: ta,
	}
}

// Init implements tea.Model.
func (p *commentsPopup) Init() tea.Cmd { return p.issue.Init() }

// Update implements tea.Model.
func (p *commentsPopup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.input.SetWidth(msg.Width - styles.Input().GetHorizontalFrameSize())
		msg.Height -= commentInputHeight + styles.Input().GetVerticalFrameSize() + hintLines
		_, cmd := p.issue.Update(msg)
		return p, cmd
	case commentSentMsg:
		if msg.id != p.id {
			return p, nil
		}
		p.sending, p.sendErr = false, msg.err
		p.issue.SetBusy(false)
		if msg.err != nil {
			return p, nil
		}
		p.input.Reset()
		p.input.Blur()
		return p, p.issue.Reload()
	case tea.KeyPressMsg:
		if p.input.Focused() {
			return p, p.updateInput(msg)
		}
		if key.Matches(msg, keys.close) {
			return p, popup.Close()
		}
		if key.Matches(msg, keys.accept) && !p.issue.Loading() {
			p.sendErr = nil
			return p, p.input.Focus()
		}
	}
	_, cmd := p.issue.Update(msg)
	return p, cmd
}

func (p *commentsPopup) updateInput(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(msg, keys.blur):
		p.input.Blur()
		return nil
	case key.Matches(msg, keys.send):
		body := p.input.Value()
		if p.sending || strings.TrimSpace(body) == "" {
			return nil
		}
		p.sending, p.sendErr = true, nil
		b, id, issueKey := p.backend, p.id, p.issueKey
		return tea.Batch(p.issue.SetBusy(true), func() tea.Msg {
			return commentSentMsg{id: id, key: issueKey, err: b.AddComment(issueKey, body)}
		})
	}
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return cmd
}

// View implements tea.Model.
func (p *commentsPopup) View() tea.View {
	border := styles.Input()
	hint := helpView(commentsHelpKeys, p.issue.Width())
	if p.input.Focused() {
		border = border.BorderForeground(styles.Highlight())
		hint = helpView(writingHelpKeys, p.issue.Width())
	}
	switch {
	case p.sending:
		hint = styles.Dim().Render(p.issue.Spinner() + " Sending…")
	case p.sendErr != nil:
		hint = styles.Error().Render("Error: " + p.sendErr.Error())
	}
	if l := p.issue.CommentList(); l != nil && l.Total > len(l.Items) && !p.sending && p.sendErr == nil {
		prefix := styles.Dim().Render(fmt.Sprintf("Newest %d of %d · ", len(l.Items), l.Total))
		hint = prefix + helpView(commentsHelpKeys, p.issue.Width()-lipgloss.Width(prefix))
		if p.input.Focused() {
			hint = prefix + helpView(writingHelpKeys, p.issue.Width()-lipgloss.Width(prefix))
		}
	}
	if !p.issue.Ready() {
		hint = ""
	}
	return tea.NewView(strings.Join([]string{p.issue.View().Content, border.Render(p.input.View()), hint}, "\n"))
}
