// Package issueview is a popup model that loads an issue and shows its details
// or its comments as scrollable text under a title. It leaves everything below
// the text to its owner: the owner sizes it for the room its own footer leaves,
// draws the footer under View, and decides when to hide it.
//
// Build one with Details or Comments, start it with Init, and forward every
// message to Update:
//
//	func (p *detail) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
//		_, cmd := p.issue.Update(msg)
//		return p, cmd
//	}
package issueview

import (
	"strings"
	"sync/atomic"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/styles"
)

var (
	top    = key.NewBinding(key.WithKeys("g"))
	bottom = key.NewBinding(key.WithKeys("G"))
)

// Backend loads issues.
type Backend interface {
	Issue(key string) (*jira.IssueDetail, error)
}

// Markdown renders markdown wrapped to width.
type Markdown = func(md string, width int) string

type loadedMsg struct {
	id    int64
	toEnd bool
	issue *jira.IssueDetail
	err   error
}

var lastID atomic.Int64

// Model shows part of an issue as scrollable text under a title. Build one with
// Details or Comments.
type Model struct {
	// id tags the model's loads, so a late response that reaches a newer model
	// for the same issue is ignored.
	id      int64
	backend Backend
	key     string
	title   string
	render  func(d *jira.IssueDetail, width int, md Markdown) string
	md      Markdown
	issue   *jira.IssueDetail

	vp   viewport.Model
	spin spinner.Model

	loading bool
	busy    bool
	err     error
}

// Details shows an issue's summary, fields and description.
func Details(b Backend, issueKey string, md Markdown) *Model {
	return newModel(b, "Details", issueKey, renderDetail, md)
}

// Comments shows an issue's comments.
func Comments(b Backend, issueKey string, md Markdown) *Model {
	return newModel(b, "Comments", issueKey, renderComments, md)
}

func newModel(b Backend, title, issueKey string, render func(*jira.IssueDetail, int, Markdown) string, md Markdown) *Model {
	vp := viewport.New()
	vp.SetHorizontalStep(4)
	sp := spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(styles.Title()))
	return &Model{
		id: lastID.Add(1), backend: b, key: issueKey, title: title + " · " + issueKey,
		render: render, md: md, vp: vp, spin: sp, loading: true,
	}
}

// Init loads the issue and starts the spinner.
func (m *Model) Init() tea.Cmd { return tea.Batch(m.Reload(false), m.spin.Tick) }

// Reload fetches the issue again and swaps in the new text when it arrives,
// keeping the old text meanwhile. With toEnd the view then scrolls to the
// bottom, which is where a new comment shows up.
func (m *Model) Reload(toEnd bool) tea.Cmd {
	b, id, issueKey := m.backend, m.id, m.key
	return func() tea.Msg {
		d, err := b.Issue(issueKey)
		return loadedMsg{id: id, toEnd: toEnd, issue: d, err: err}
	}
}

// SetBusy keeps the spinner running while the owner waits on something that
// doesn't replace the text, like sending a comment. The returned command
// restarts the spinner, which stops when nothing is busy.
func (m *Model) SetBusy(on bool) tea.Cmd {
	was := m.spinning()
	m.busy = on
	if !was && m.spinning() {
		return m.spin.Tick
	}
	return nil
}

func (m *Model) spinning() bool { return m.loading || m.busy }

// Loading reports whether the model is waiting for its first load.
func (m *Model) Loading() bool { return m.loading }

// Ready reports whether the text is showing, so neither loading nor failed.
func (m *Model) Ready() bool { return !m.loading && m.err == nil }

// Spinner returns the current spinner frame, for the owner's own busy hints.
func (m *Model) Spinner() string { return m.spin.View() }

// Width is the width of the text area.
func (m *Model) Width() int { return m.vp.Width() }

// ScrollPercent is how far the text is scrolled, from 0 to 1.
func (m *Model) ScrollPercent() float64 { return m.vp.ScrollPercent() }

func (m *Model) refresh() {
	if m.issue != nil {
		m.vp.SetContent(m.render(m.issue, m.vp.Width(), m.md))
	}
}

// Update shows the issue once it loads and handles sizing, the spinner and
// scroll keys. It ignores everything else, including the keys that close the
// popup. The title takes one line of the height it's sized for.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case loadedMsg:
		switch {
		case msg.id != m.id:
		case msg.err != nil:
			m.err, m.loading = msg.err, false
		case msg.issue != nil:
			m.issue, m.err, m.loading = msg.issue, nil, false
			m.refresh()
			if msg.toEnd {
				m.vp.GotoBottom()
			}
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.vp.SetWidth(msg.Width)
		m.vp.SetHeight(max(1, msg.Height-1))
		m.refresh()
		return m, nil
	case spinner.TickMsg:
		if !m.spinning() {
			return m, nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, top):
			m.vp.GotoTop()
			return m, nil
		case key.Matches(msg, bottom):
			m.vp.GotoBottom()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

// View draws the title and the text, or the error or loading state in its place.
func (m *Model) View() tea.View {
	var body string
	switch {
	case m.err != nil:
		body = styles.Error().Render("Error: " + m.err.Error())
	case m.loading:
		body = m.spin.View() + " Loading…"
	default:
		body = m.vp.View()
	}
	h := m.vp.Height()
	return tea.NewView(strings.Join([]string{
		styles.Title().Render(m.title),
		lipgloss.NewStyle().Height(h).MaxHeight(h).Render(body),
	}, "\n"))
}
