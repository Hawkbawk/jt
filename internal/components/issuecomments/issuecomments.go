// Package issuecomments is a tea.Model that shows an issue's comments above a
// box for writing a new one. It knows nothing about where it's shown: it asks
// to be dismissed with Closed and reports a posted comment with Sent.
package issuecomments

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/popup/issueview"
	"github.com/hawkbawk/jt/internal/tui/popup/pane"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
)

const (
	inputHeight = 5
	// hintLines is the height of the key hint under the input.
	hintLines = 1
)

// Backend loads an issue's comments and posts a new one.
type Backend interface {
	issueview.Backend
	AddComment(key, body string) error
}

// Closed asks the host to dismiss the comments.
type Closed struct{}

// Sent reports that a comment was posted. It's a tuimsg.Finished.
type Sent struct {
	Key string
}

var _ tuimsg.Finished = Sent{}

// IssueKey implements tuimsg.Finished.
func (s Sent) IssueKey() string { return s.Key }

// Status implements tuimsg.Finished.
func (s Sent) Status() string { return "Commented on " + s.Key }

var (
	keyClose  = key.NewBinding(key.WithKeys("esc", "q"), key.WithHelp("esc", "close"))
	keyWrite  = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "write"))
	keySend   = key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "send"))
	keyBlur   = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "stop writing"))
	keyScroll = key.NewBinding(key.WithKeys("up", "down", "left", "right"), key.WithHelp("↑↓←→", "scroll"))
	keyTop    = key.NewBinding(key.WithKeys("g", "G"), key.WithHelp("g/G", "top/bottom"))

	readingHelp = helpKeys{keyScroll, keyTop, keyWrite, keyClose}
	writingHelp = helpKeys{keySend, keyBlur}
)

// helpKeys is a help footer made of a fixed list of bindings.
type helpKeys []key.Binding

// ShortHelp implements help.KeyMap.
func (k helpKeys) ShortHelp() []key.Binding { return k }

// FullHelp implements help.KeyMap.
func (k helpKeys) FullHelp() [][]key.Binding { return [][]key.Binding{k} }

func helpView(km helpKeys, width int) string {
	h := styles.Help()
	h.SetWidth(max(0, width))
	return h.View(km)
}

type sentMsg struct {
	id  int64
	key string
	err error
}

// Model shows an issue's comments above a box for writing a new one.
type Model struct {
	id       int64
	backend  Backend
	issueKey string
	issue    *issueview.Model
	input    textarea.Model

	sending bool
	sendErr error
}

// New loads the comments of the issue with issueKey, rendering them with md.
func New(b Backend, issueKey string, md issueview.Markdown) *Model {
	ta := textarea.New()
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.Placeholder = "Write a comment…"
	ta.SetStyles(styles.TextArea())
	ta.SetHeight(inputHeight)

	return &Model{
		id: pane.NewID(), backend: b, issueKey: issueKey,
		issue: issueview.Comments(b, issueKey, md), input: ta,
	}
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd { return m.issue.Init() }

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.input.SetWidth(msg.Width - styles.Input().GetHorizontalFrameSize())
		msg.Height -= inputHeight + styles.Input().GetVerticalFrameSize() + hintLines
		_, cmd := m.issue.Update(msg)
		return m, cmd
	case sentMsg:
		if msg.id != m.id {
			return m, nil
		}
		m.sending, m.sendErr = false, msg.err
		m.issue.SetBusy(false)
		if msg.err != nil {
			return m, nil
		}
		m.input.Reset()
		m.input.Blur()
		return m, tea.Batch(m.issue.Reload(), func() tea.Msg { return Sent{Key: msg.key} })
	case tea.KeyPressMsg:
		if m.input.Focused() {
			return m, m.updateInput(msg)
		}
		if key.Matches(msg, keyClose) {
			return m, func() tea.Msg { return Closed{} }
		}
		if key.Matches(msg, keyWrite) && !m.issue.Loading() {
			m.sendErr = nil
			return m, m.input.Focus()
		}
	}
	_, cmd := m.issue.Update(msg)
	return m, cmd
}

func (m *Model) updateInput(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(msg, keyBlur):
		m.input.Blur()
		return nil
	case key.Matches(msg, keySend):
		body := m.input.Value()
		if m.sending || strings.TrimSpace(body) == "" {
			return nil
		}
		m.sending, m.sendErr = true, nil
		b, id, issueKey := m.backend, m.id, m.issueKey
		return tea.Batch(m.issue.SetBusy(true), func() tea.Msg {
			return sentMsg{id: id, key: issueKey, err: b.AddComment(issueKey, body)}
		})
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return cmd
}

// View implements tea.Model.
func (m *Model) View() tea.View {
	border := styles.Input()
	hint := helpView(readingHelp, m.issue.Width())
	if m.input.Focused() {
		border = border.BorderForeground(styles.Highlight())
		hint = helpView(writingHelp, m.issue.Width())
	}
	switch {
	case m.sending:
		hint = styles.Dim().Render(m.issue.Spinner() + " Sending…")
	case m.sendErr != nil:
		hint = styles.Error().Render("Error: " + m.sendErr.Error())
	}
	if l := m.issue.CommentList(); l != nil && l.Total > len(l.Items) && !m.sending && m.sendErr == nil {
		prefix := styles.Dim().Render(fmt.Sprintf("Newest %d of %d · ", len(l.Items), l.Total))
		rest := m.issue.Width() - lipgloss.Width(prefix)
		hint = prefix + helpView(readingHelp, rest)
		if m.input.Focused() {
			hint = prefix + helpView(writingHelp, rest)
		}
	}
	if !m.issue.Ready() {
		hint = ""
	}
	return tea.NewView(strings.Join([]string{m.issue.View().Content, border.Render(m.input.View()), hint}, "\n"))
}
