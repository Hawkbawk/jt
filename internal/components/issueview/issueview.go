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
	"github.com/hawkbawk/jt/internal/keys"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/termimage"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
	"github.com/hawkbawk/jt/internal/tui/widget"
)

// Backend loads issues, their comments and the images in them.
type Backend interface {
	Issue(key string) (*jira.IssueDetail, error)
	Comments(key string) (*jira.CommentList, error)
	Attachment(id string) ([]byte, error)
}

// Markdown renders markdown wrapped to width.
type Markdown = func(md string, width int) string

// loadedMsg carries what a model loaded: the issue for Details, the comments
// for Comments.
type loadedMsg struct {
	issue    *jira.IssueDetail
	comments *jira.CommentList
	err      error
}

var lastID atomic.Int64

// Model shows part of an issue as scrollable text under a title. Build one with
// Details or Comments.
type Model struct {
	tuimsg.Requests
	top, bottom key.Binding
	// id places the model's images apart from another model's.
	id      int64
	backend Backend
	key     string
	title   string
	fetch   func(b Backend, key string) loadedMsg
	render  func(m *Model, width int, md Markdown) string
	md      Markdown
	issue   *jira.IssueDetail
	list    *jira.CommentList
	// images is the state of each image asked for, by attachment ID. placed is
	// the size each is placed at. imageRows is where the images not asked for
	// yet are in the text. pending is what the terminal still has to be sent
	// for them.
	images    map[string]*imageState
	placed    map[string]termimage.Placement
	imageRows []imageRow
	pending   []string

	vp   viewport.Model
	spin widget.Spinner

	loading bool
	busy    bool
	err     error
}

// Details shows an issue's summary, fields and description.
func Details(b Backend, issueKey string, md Markdown) *Model {
	return newModel(b, "Details", issueKey, md,
		func(b Backend, key string) loadedMsg {
			d, err := b.Issue(key)
			return loadedMsg{issue: d, err: err}
		},
		func(m *Model, width int, md Markdown) string { return renderDetail(m.issue, width, md) })
}

// Comments shows an issue's comments, which load only when it opens.
func Comments(b Backend, issueKey string, md Markdown) *Model {
	return newModel(b, "Comments", issueKey, md,
		func(b Backend, key string) loadedMsg {
			l, err := b.Comments(key)
			return loadedMsg{comments: l, err: err}
		},
		func(m *Model, width int, md Markdown) string { return renderComments(m.list, width, md) })
}

func newModel(b Backend, title, issueKey string, md Markdown, fetch func(Backend, string) loadedMsg, render func(*Model, int, Markdown) string) *Model {
	vp := viewport.New()
	vp.SetHorizontalStep(4)
	return &Model{
		top: keys.Get().Top, bottom: keys.Get().Bottom,
		id: lastID.Add(1), backend: b, key: issueKey, title: title + " · " + issueKey,
		fetch: fetch, render: render, md: md, vp: vp, spin: widget.NewSpinner(), loading: true,
		images: map[string]*imageState{}, placed: map[string]termimage.Placement{},
	}
}

// Init loads the issue and starts the spinner.
func (m *Model) Init() tea.Cmd { return tea.Batch(m.Reload(), m.spin.Sync(m.spinning())) }

// Reload fetches the issue again and swaps in the new text when it arrives,
// keeping the old text meanwhile.
func (m *Model) Reload() tea.Cmd {
	b, issueKey, fetch := m.backend, m.key, m.fetch
	return m.Request(func() tea.Msg { return fetch(b, issueKey) })
}

// SetBusy keeps the spinner running while the owner waits on something that
// doesn't replace the text, like sending a comment. The returned command
// restarts the spinner, which stops when nothing is busy.
func (m *Model) SetBusy(on bool) tea.Cmd {
	m.busy = on
	return m.spin.Sync(m.spinning())
}

func (m *Model) spinning() bool { return m.loading || m.busy }

// Loading reports whether the model is waiting for its first load.
func (m *Model) Loading() bool { return m.loading }

// Ready reports whether the text is showing, so neither loading nor failed.
func (m *Model) Ready() bool { return !m.loading && m.err == nil }

// Spinner returns the current spinner frame, for the owner's own busy hints.
func (m *Model) Spinner() string { return m.spin.View() }

// Detail is the loaded issue, or nil before the first load or for a model that
// shows comments.
func (m *Model) Detail() *jira.IssueDetail { return m.issue }

// CommentList is the loaded comments, or nil before the first load or for a
// model that shows details.
func (m *Model) CommentList() *jira.CommentList { return m.list }

// Width is the width of the text area.
func (m *Model) Width() int { return m.vp.Width() }

// ScrollPercent is how far the text is scrolled, from 0 to 1.
func (m *Model) ScrollPercent() float64 { return m.vp.ScrollPercent() }

func (m *Model) refresh() {
	if m.issue != nil || m.list != nil {
		content, rows := extractMarks(m.render(m, m.vp.Width(), m.withImages(m.md)))
		m.imageRows = rows
		m.vp.SetContent(content)
	}
}

// Update shows the issue once it loads and handles sizing, the spinner and
// scroll keys. It ignores everything else, including the keys that close the
// popup. The title takes one line of the height it's sized for. Images in the
// text load after it, and replace the text that stood in for them.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := m.update(msg)
	return m, tea.Batch(cmd, m.loadVisible(), m.flush())
}

func (m *Model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case loadedMsg:
		switch {
		case msg.err != nil:
			m.err, m.loading = msg.err, false
		case msg.issue != nil || msg.comments != nil:
			m.issue, m.list, m.err, m.loading = msg.issue, msg.comments, nil, false
			m.refresh()
		}
		return m, nil
	case imageMsg:
		m.imageLoaded(msg)
		return m, nil
	case tea.WindowSizeMsg:
		m.vp.SetWidth(msg.Width)
		m.vp.SetHeight(max(1, msg.Height-1))
		m.refresh()
		return m, nil
	case spinner.TickMsg:
		return m, m.spin.Update(msg, m.spinning())
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.top):
			m.vp.GotoTop()
			return m, nil
		case key.Matches(msg, m.bottom):
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
	h := m.vp.Height()
	switch {
	case m.err != nil:
		body = styles.Error().Render("Error: " + m.err.Error())
	case m.loading:
		body = m.spin.View() + " Loading…"
	default:
		// The viewport pads itself to its size, and measuring a screen of
		// image placeholders a second time is slow.
		return tea.NewView(styles.Title().Render(m.title) + "\n" + m.vp.View())
	}
	return tea.NewView(strings.Join([]string{
		styles.Title().Render(m.title),
		lipgloss.NewStyle().Height(h).MaxHeight(h).Render(body),
	}, "\n"))
}
