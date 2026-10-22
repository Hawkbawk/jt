package tui

import (
	"fmt"

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
	backend Backend
	project string
	nav     *boardNav
	loading bool
	err     error
}

func newBoardPane(b Backend) boardPane {
	project := b.DefaultProject()
	return boardPane{backend: b, project: project, loading: project != ""}
}

// fetch loads project's board without touching the pane's state.
func (b boardPane) fetch(project, focusKey string) tea.Cmd {
	backend := b.backend
	return func() tea.Msg {
		board, err := backend.Board(project)
		return boardLoadedMsg{project: project, board: board, focusKey: focusKey, err: err}
	}
}

// open switches to project, dropping the current board.
func (b *boardPane) open(project string) tea.Cmd {
	b.project = project
	b.nav = nil
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
	b.nav = newBoardNav(msg.board)
	if msg.focusKey != "" {
		b.nav.focus(msg.focusKey)
	}
}

// move applies a navigation key and reports whether it was one.
func (b *boardPane) move(key string) bool {
	if b.nav == nil {
		return false
	}
	switch key {
	case "h", "left":
		b.nav.left()
	case "l", "right":
		b.nav.right()
	case "k", "up":
		b.nav.up()
	case "j", "down":
		b.nav.down()
	case "g":
		b.nav.top()
	case "G":
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
