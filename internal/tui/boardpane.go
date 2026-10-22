package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/hawkbawk/jt/internal/jira"
)

type boardLoadedMsg struct {
	project  string
	board    *jira.Board
	focusKey string
	err      error
}

// boardPane owns the current project's board: loading it, tracking focus and
// rendering the body of the screen.
type boardPane struct {
	backend   Backend
	project   string
	nav       *boardNav
	filter    *jira.Filter
	quick     []jira.Filter
	assignees []jira.Filter
	loading   bool
	err       error
}

func newBoardPane(b Backend) boardPane {
	project := b.DefaultProject()
	return boardPane{backend: b, project: project, loading: project != ""}
}

// fetch loads project's board without touching the pane's state.
func (b boardPane) fetch(project, focusKey string) tea.Cmd {
	backend := b.backend
	filterJQL := ""
	if b.filter != nil {
		filterJQL = b.filter.JQL
	}
	return func() tea.Msg {
		board, err := backend.Board(project, filterJQL)
		return boardLoadedMsg{project: project, board: board, focusKey: focusKey, err: err}
	}
}

// open switches to project, dropping the current board.
func (b *boardPane) open(project string) tea.Cmd {
	b.project = project
	b.nav = nil
	b.filter, b.quick, b.assignees = nil, nil, nil
	return b.reload("")
}

// setFilter narrows the board to f, or shows everything when f is nil.
func (b *boardPane) setFilter(f *jira.Filter) tea.Cmd {
	b.filter = f
	return b.reload("")
}

// reload refreshes the current board and refocuses focusKey when it's found.
func (b *boardPane) reload(focusKey string) tea.Cmd {
	b.loading = true
	b.err = nil
	return b.fetch(b.project, focusKey)
}

// update applies a loaded board, ignoring results for a project that's no
// longer current.
func (b *boardPane) update(msg boardLoadedMsg) {
	if msg.project != b.project {
		return
	}
	b.loading = false
	if msg.err != nil {
		b.err = msg.err
		return
	}
	// A filtered board only knows its own assignees, so the options come from
	// the unfiltered load.
	if b.filter == nil {
		b.quick, b.assignees = msg.board.QuickFilters, msg.board.Assignees
	}
	b.nav = newBoardNav(msg.board)
	if msg.focusKey != "" {
		b.nav.focus(msg.focusKey)
	}
}

// move applies a navigation key and reports whether it was one.
func (b *boardPane) move(msg tea.KeyMsg) bool {
	if b.nav == nil {
		return false
	}
	switch {
	case key.Matches(msg, keys.left):
		b.nav.left()
	case key.Matches(msg, keys.right):
		b.nav.right()
	case key.Matches(msg, keys.up):
		b.nav.up()
	case key.Matches(msg, keys.down):
		b.nav.down()
	case key.Matches(msg, keys.top):
		b.nav.top()
	case key.Matches(msg, keys.bottom):
		b.nav.bottom()
	default:
		return false
	}
	return true
}

// selected returns the focused issue, or nil when there is none.
func (b *boardPane) selected() *jira.Issue {
	if b.nav == nil {
		return nil
	}
	return b.nav.selected()
}

// headerParts lists what the pane contributes to the header line.
func (b boardPane) headerParts(spin string) []string {
	var parts []string
	if b.project != "" {
		parts = append(parts, b.project)
	}
	if b.nav != nil {
		parts = append(parts, dimStyle.Render(b.nav.board.Name))
	}
	if b.filter != nil {
		parts = append(parts, "filter: "+b.filter.Name)
	}
	if b.loading {
		parts = append(parts, spin)
	}
	return parts
}

func (b boardPane) view(width, height int, spin string) string {
	var body string
	switch {
	case b.err != nil:
		body = errorStyle.Render(fmt.Sprintf("Error: %v", b.err))
	case b.nav != nil:
		return renderBoard(b.nav, width, height)
	case b.loading:
		body = spin + " Loading board…"
	default:
		body = dimStyle.Render("No project selected.")
	}
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, body)
}
