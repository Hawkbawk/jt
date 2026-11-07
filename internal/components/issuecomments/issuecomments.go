// Package issuecomments is a tea.Model that shows an issue's comments above a
// box for writing a new one. It knows nothing about where it's shown: it asks
// to be dismissed with Closed and reports a posted comment with Sent.
package issuecomments

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/hawkbawk/jt/internal/keys"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/popup/issueview"
	"github.com/hawkbawk/jt/internal/tui/popup/pane"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
)

const (
	inputHeight = 5
	inputLines  = inputHeight + 2
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

// keyMap is the popup's key bindings. Its help depends on whether the input
// has focus.
type keyMap struct {
	scroll, topBottom, write, close key.Binding
	send, blur                      key.Binding

	form    []key.Binding
	writing bool
}

func newKeyMap() keyMap {
	k := keys.Get()
	return keyMap{
		scroll:    k.Scroll,
		topBottom: k.TopBottom,
		write:     keys.Describe(k.Accept, "write"),
		close:     k.Close,
		send:      k.Send,
		blur:      keys.Describe(k.Dismiss, "stop writing"),
	}
}

// inWriting returns k with the help for when the input has focus.
func (k keyMap) inWriting(form []key.Binding) keyMap {
	k.writing = true
	k.form = form
	return k
}

// ShortHelp implements help.KeyMap.
func (k keyMap) ShortHelp() []key.Binding {
	if k.writing {
		return append(append([]key.Binding{k.send}, k.form...), k.blur)
	}
	return []key.Binding{k.scroll, k.topBottom, k.write, k.close}
}

// FullHelp implements help.KeyMap.
func (k keyMap) FullHelp() [][]key.Binding { return [][]key.Binding{k.ShortHelp()} }

func helpView(km keyMap, width int) string {
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
	keys     keyMap
	id       int64
	backend  Backend
	issueKey string
	issue    *issueview.Model
	body     string
	input    *huh.Text
	form     *huh.Form

	sending bool
	sendErr error
}

// New loads the comments of the issue with issueKey, rendering them with md.
func New(b Backend, issueKey string, md issueview.Markdown) *Model {
	return &Model{
		id: pane.NewID(), backend: b, issueKey: issueKey, keys: newKeyMap(),
		issue: issueview.Comments(b, issueKey, md),
	}
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd { return m.issue.Init() }

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if m.form != nil {
			m.form.WithWidth(msg.Width).WithHeight(inputLines)
		}
		msg.Height -= inputLines + hintLines
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
		m.body = ""
		m.closeInput()
		return m, tea.Batch(m.issue.Reload(), func() tea.Msg { return Sent{Key: msg.key} })
	case tea.KeyPressMsg:
		if m.form != nil {
			switch {
			case key.Matches(msg, m.keys.blur):
				m.closeInput()
				return m, nil
			case key.Matches(msg, m.keys.send):
				return m, m.send()
			}
			if m.sending {
				return m, nil
			}
			_, cmd := m.form.Update(msg)
			return m, cmd
		}
		if key.Matches(msg, m.keys.close) {
			return m, func() tea.Msg { return Closed{} }
		}
		if key.Matches(msg, m.keys.write) && !m.issue.Loading() {
			return m, m.openInput()
		}
	}
	if m.form != nil {
		_, cmd := m.form.Update(msg)
		return m, cmd
	}
	_, cmd := m.issue.Update(msg)
	return m, cmd
}

func (m *Model) openInput() tea.Cmd {
	m.sendErr = nil
	m.input = huh.NewText().Placeholder("Write a comment…").
		CharLimit(0).ShowLineNumbers(false).Lines(inputHeight).ExternalEditor(true).Value(&m.body)
	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(key.WithDisabled())
	km.Text.Next = key.NewBinding(key.WithDisabled())
	km.Text.Prev = key.NewBinding(key.WithDisabled())
	km.Text.Submit = key.NewBinding(key.WithDisabled())
	m.form = pane.NewForm(m.input).WithKeyMap(km)
	return m.form.Init()
}

func (m *Model) closeInput() {
	m.form, m.input = nil, nil
}

func (m *Model) send() tea.Cmd {
	if m.sending || strings.TrimSpace(m.body) == "" {
		return nil
	}
	m.sending, m.sendErr = true, nil
	b, id, issueKey, body := m.backend, m.id, m.issueKey, m.body
	return tea.Batch(m.issue.SetBusy(true), func() tea.Msg {
		return sentMsg{id: id, key: issueKey, err: b.AddComment(issueKey, body)}
	})
}

// View implements tea.Model.
func (m *Model) View() tea.View {
	hint := helpView(m.keys, m.issue.Width())
	input := styles.Input().Width(m.issue.Width()).Height(inputHeight).
		Render(styles.Dim().Render("Write a comment…"))
	if m.form != nil {
		m.form.WithWidth(m.issue.Width()).WithHeight(inputLines)
		input = m.form.View()
		hint = helpView(m.keys.inWriting(m.form.KeyBinds()), m.issue.Width())
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
		hint = prefix + helpView(m.keys, rest)
		if m.form != nil {
			hint = prefix + helpView(m.keys.inWriting(m.form.KeyBinds()), rest)
		}
	}
	if !m.issue.Ready() {
		hint = ""
	}
	return tea.NewView(strings.Join([]string{m.issue.View().Content, input, hint}, "\n"))
}
