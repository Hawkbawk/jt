// Package issuecomments is the popup that shows an issue's comments above a
// box for writing a new one. It reports a posted comment with Sent and stays
// open.
package issuecomments

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/hawkbawk/jt/internal/components/issueview"
	"github.com/hawkbawk/jt/internal/components/pane"
	"github.com/hawkbawk/jt/internal/keys"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
)

const (
	inputHeight = 5
	inputLines  = inputHeight + 2
)

// Backend loads an issue's comments and posts a new one.
type Backend interface {
	issueview.Backend
	AddComment(key, body string) error
}

// Sent reports that a comment was posted. It's a tuimsg.Finished.
type Sent struct {
	Key string
}

// ForParent marks Sent for the popup host.
func (Sent) ForParent() {}

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

type sentMsg struct {
	key string
	err error
}

// Model shows an issue's comments above a box for writing a new one.
type Model struct {
	keys     keyMap
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
		backend: b, issueKey: issueKey, keys: newKeyMap(),
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
		msg.Height -= inputLines
		_, cmd := m.issue.Update(msg)
		return m, cmd
	case sentMsg:
		m.sending, m.sendErr = false, msg.err
		m.issue.SetBusy(false)
		if msg.err != nil {
			return m, nil
		}
		m.body = ""
		m.closeInput()
		return m, tea.Batch(m.issue.Reload(), tuimsg.Send(Sent{Key: msg.key}))
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
			return m, tuimsg.Close()
		}
		if key.Matches(msg, m.keys.write) && !m.issue.Loading() {
			return m, m.openInput()
		}
	}
	if m.form != nil {
		_, formCmd := m.form.Update(msg)
		_, issueCmd := m.issue.Update(msg)
		return m, tea.Batch(formCmd, issueCmd)
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
	b, issueKey, body := m.backend, m.issueKey, m.body
	return tea.Batch(m.issue.SetBusy(true), func() tea.Msg {
		return sentMsg{key: issueKey, err: b.AddComment(issueKey, body)}
	})
}

// Title implements popup.Titled.
func (m *Model) Title() string { return m.issue.Title() }

// Footer implements popup.Footed.
func (m *Model) Footer() (string, help.KeyMap) {
	switch {
	case !m.issue.Ready():
		return "", nil
	case m.sending:
		return styles.Dim().Render(m.issue.Spinner() + " Sending…"), nil
	case m.sendErr != nil:
		return styles.Error().Render("Error: " + m.sendErr.Error()), nil
	}
	var keys help.KeyMap = m.keys
	if m.form != nil {
		keys = m.keys.inWriting(m.form.KeyBinds())
	}
	var status string
	if l := m.issue.CommentList(); l != nil && l.Total > len(l.Items) {
		status = styles.Dim().Render(fmt.Sprintf("Newest %d of %d · ", len(l.Items), l.Total))
	}
	return status, keys
}

// View implements tea.Model.
func (m *Model) View() tea.View {
	input := styles.Input().Width(m.issue.Width()).Height(inputHeight).
		Render(styles.Dim().Render("Write a comment…"))
	if m.form != nil {
		m.form.WithWidth(m.issue.Width()).WithHeight(inputLines)
		input = m.form.View()
	} else if m.body != "" {
		input = styles.Input().Width(m.issue.Width()).Height(inputHeight).
			MaxHeight(inputHeight).Render(m.body)
	}
	return tea.NewView(m.issue.View().Content + "\n" + input)
}
