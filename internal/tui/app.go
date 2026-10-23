// Package tui implements jt's bubbletea interface.
package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/hawkbawk/jt/internal/jira"
)

// Backend is the Jira functionality the TUI depends on.
type Backend interface {
	DefaultProject() string
	Projects() ([]jira.Project, error)
	Board(project, filterJQL string) (*jira.Board, error)
	Issue(key string) (*jira.IssueDetail, error)
	Transitions(key string) ([]jira.Transition, error)
	Transition(key, transitionID string) error
	Assign(key, assigneeID string) error
	AddComment(key, body string) error
	EditFields(iss jira.Issue) ([]jira.EditField, error)
	Edit(key string, fields []jira.EditField) error
	IssueURL(key string) string
}

// Model is the root bubbletea model. It routes input and messages to the
// board, project picker and at most one popup, and controls high-level
// or broadly used state.
type Model struct {
	width, height int
	spinner       spinner.Model
	status        string
	help          help.Model

	board   boardPane
	picker  picker
	popup   popup
	actions issueActions
	quit    quitGuard
}

// New builds the root model. The backend's default project, if any, is
// opened on start.
func New(backend Backend) Model {
	m := Model{
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(titleStyle)),
		help:    newHelp(),
		board:   newBoardPane(backend),
		picker:  newPicker(backend),
		actions: newIssueActions(backend),
	}
	if m.board.project == "" {
		m.picker.show(false)
	}
	return m
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	if m.board.project == "" {
		return m.startBusy(m.picker.fetch())
	}
	return m.startBusy(m.board.fetch(m.board.project, ""))
}

// startBusy runs cmd alongside the spinner.
func (m Model) startBusy(cmd tea.Cmd) tea.Cmd {
	return tea.Batch(cmd, m.spinner.Tick)
}

// busy reports whether anything is waiting on the network.
func (m Model) busy() bool {
	if p, ok := m.popup.(interface{ loading() bool }); ok && p.loading() {
		return true
	}
	return m.board.loading || m.picker.loading()
}

func (m Model) bodyHeight() int {
	return max(0, m.height-2)
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.picker.setSize(m.width, m.bodyHeight())
		if m.popup != nil {
			m.popup.setSize(popupSize(m.width, m.bodyHeight()))
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case spinner.TickMsg:
		if !m.busy() {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case projectSelectedMsg:
		m.status = ""
		return m, m.startBusy(m.board.open(msg.key))

	case boardLoadedMsg:
		m.board.update(msg)
		return m, nil

	case issueLoadedMsg, transitionsLoadedMsg, fieldsLoadedMsg, commentSentMsg:
		return m.forwardToPopup(msg)

	case filterSelectedMsg:
		m.status = ""
		return m, m.startBusy(m.board.setFilter(msg.filter))

	case transitionDoneMsg:
		return m.finishEdit(msg, msg.err, msg.key, "Moved "+msg.key+" to "+msg.to)

	case assignDoneMsg:
		status := "Assigned " + msg.issueKey + " to " + msg.to
		if msg.cleared {
			status = "Unassigned " + msg.issueKey
		}
		return m.finishEdit(msg, msg.err, msg.issueKey, status)

	case fieldsSavedMsg:
		return m.finishEdit(msg, msg.err, msg.key, fmt.Sprintf("Updated %d %s on %s", msg.count, plural(msg.count, "field"), msg.key))

	case actionDoneMsg:
		m.status = msg.display()
		return m, nil
	}
	// The picker's list filters asynchronously and reports back with its own
	// messages.
	return m, m.picker.update(msg)
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if consumed, quit := m.quit.handle(msg); consumed {
		if quit {
			return m, tea.Quit
		}
		return m, nil
	}

	switch {
	case key.Matches(msg, keys.projects) && !m.picker.open:
		// A popup may hold unsaved work, so it has to be closed first.
		if m.popup != nil {
			return m, nil
		}
		return m, m.startBusy(m.picker.show(m.board.nav != nil))
	case m.picker.open:
		return m, m.picker.update(msg)
	case m.popup != nil:
		return m.forwardToPopup(msg)
	}
	return m.handleBoardKey(msg)
}

func (m Model) handleBoardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, keys.quit) {
		return m, tea.Quit
	}
	if m.board.move(msg) {
		return m, nil
	}
	sel := m.board.selected()
	if key.Matches(msg, keys.refresh) && m.board.project != "" && !m.board.loading {
		focusKey := ""
		if sel != nil {
			focusKey = sel.Key
		}
		m.status = ""
		return m, m.startBusy(m.board.reload(focusKey))
	}
	if key.Matches(msg, keys.filter) && m.board.nav != nil {
		return m, m.openPopup(newFilterPopup(m.board.quick, m.board.assignees, m.board.filter), nil)
	}
	if sel == nil {
		return m, nil
	}
	if key.Matches(msg, keys.edit) {
		b := m.actions.backend
		return m, m.openPopup(newEditPopup(b, *sel, m.board.assignees), tea.Batch(loadTransitions(b, sel.Key), loadFields(b, *sel)))
	}
	p, cmd := m.actions.handle(msg, sel.Key)
	if p != nil {
		return m, m.openPopup(p, cmd)
	}
	return m, cmd
}

// finishEdit closes the popup and reloads the board after a successful change.
// A failure goes back to the popup to show.
func (m Model) finishEdit(msg tea.Msg, err error, issueKey, status string) (tea.Model, tea.Cmd) {
	if err != nil {
		return m.forwardToPopup(msg)
	}
	m.popup = nil
	m.status = status
	return m, m.startBusy(m.board.reload(issueKey))
}

func (m Model) forwardToPopup(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.popup == nil {
		return m, nil
	}
	wasBusy := m.busy()
	var cmd tea.Cmd
	m.popup, cmd = m.popup.update(msg)
	// Popups start their own loads (sending or reloading a comment), so the
	// spinner has to be started here once they become busy.
	if !wasBusy && m.busy() {
		cmd = m.startBusy(cmd)
	}
	return m, cmd
}

// openPopup shows p and runs load, the command that fills it.
func (m *Model) openPopup(p popup, load tea.Cmd) tea.Cmd {
	p.setSize(popupSize(m.width, m.bodyHeight()))
	m.popup = p
	return m.startBusy(load)
}
