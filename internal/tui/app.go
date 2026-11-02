// Package tui implements jt's bubbletea interface.
package tui

import (
	"context"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/styles"
)

// Backend is the Jira functionality the TUI depends on.
type Backend interface {
	DefaultProject() string
	Projects() ([]jira.Project, error)
	IssueTypes() ([]string, error)
	Search(ctx context.Context, jql string) ([]jira.Issue, error)
	Board(project, filterJQL string) (*jira.Board, error)
	Issue(key string) (*jira.IssueDetail, error)
	Transitions(iss jira.Issue) ([]jira.Transition, error)
	Transition(key, transitionID string) error
	Assign(key, assigneeID string) error
	AddComment(key, body string) error
	EditFields(iss jira.Issue) ([]jira.EditField, error)
	Edit(key string, fields []jira.EditField) error
	SearchUsers(issueKey, query string) ([]jira.Option, error)
	IssueURL(key string) string
}

// BoardModel is the board's root bubbletea model. It routes input and messages to the
// board, project picker and at most one popup, and controls high-level
// or broadly used state.
type BoardModel struct {
	layout

	board   boardPane
	picker  picker
	actions issueActions
}

// New builds the root model. The backend's default project, if any, is
// opened on start.
func New(backend Backend, settings Settings) BoardModel {
	m := BoardModel{
		layout:  newLayout(settings),
		board:   newBoardPane(backend),
		picker:  newPicker(backend),
		actions: newIssueActions(backend),
	}
	if m.board.project == "" {
		m.picker.show(false)
	}
	return m
}

// WithWarning shows text in the status line until the next action replaces it.
func (m BoardModel) WithWarning(text string) BoardModel {
	m.status = styles.Warn().Render(text)
	return m
}

// Init implements tea.Model.
func (m BoardModel) Init() tea.Cmd {
	if m.board.project == "" {
		return m.runWithSpinner(m.picker.fetch())
	}
	return m.runWithSpinner(m.board.fetch(m.board.project, ""))
}

// busy reports whether anything is waiting on the network.
func (m BoardModel) busy() bool {
	return m.popupLoading() || m.board.loading || m.picker.loading()
}

// Update implements tea.Model.
func (m BoardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case transitionDoneMsg, assignDoneMsg, fieldsSavedMsg:
		return m.finishEdit(msg, editResult(msg))

	case tea.WindowSizeMsg:
		m.resize(msg)
		m.picker.setSize(m.width, m.bodyHeight())
		return m, nil

	case ThemeChangedMsg:
		m.restyle()
		return m, nil

	case themeSelectedMsg:
		cmd := m.selectTheme(msg)
		m.picker.restyle()
		return m, cmd

	case themeSavedMsg:
		m.onThemeSaved(msg)
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case spinner.TickMsg:
		return m, m.tick(msg, m.busy())

	case projectSelectedMsg:
		m.status = ""
		return m, m.runWithSpinner(m.board.open(msg.key))

	case boardLoadedMsg:
		m.board.update(msg)
		return m, nil

	case issueLoadedMsg, transitionsLoadedMsg, fieldsLoadedMsg, commentSentMsg:
		return m.forwardToPopup(msg)

	case filterSelectedMsg:
		m.status = ""
		return m, m.runWithSpinner(m.board.setFilter(msg.filter))

	case actionDoneMsg:
		m.status = msg.display()
		return m, nil
	}
	// The picker's list filters asynchronously and a popup's form runs on its
	// own messages, so both get what's left.
	pm, popupCmd := m.forwardToPopup(msg)
	m = pm.(BoardModel)
	return m, tea.Batch(popupCmd, m.picker.update(msg))
}

// restyle rebuilds what the model copied from the styles when it was built.
func (m *BoardModel) restyle() {
	m.layout.restyle()
	m.picker.restyle()
}

func (m BoardModel) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
		return m, m.runWithSpinner(m.picker.show(m.board.nav != nil))
	case m.picker.open:
		return m, m.picker.update(msg)
	case m.popup != nil:
		return m.forwardToPopup(msg)
	}
	return m.handleBoardKey(msg)
}

func (m BoardModel) handleBoardKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
		return m, m.runWithSpinner(m.board.reload(focusKey))
	}
	if key.Matches(msg, keys.theme) {
		return m, m.openPopup(newThemePopup(), nil)
	}
	if key.Matches(msg, keys.filter) && m.board.nav != nil {
		return m, m.openPopup(newFilterPopup(m.board.quick, m.board.assignees, m.board.filter), nil)
	}
	if sel == nil {
		return m, nil
	}
	p, cmd := m.actions.handle(msg, *sel)
	if p != nil {
		return m, m.openPopup(p, cmd)
	}
	return m, cmd
}

// finishEdit closes the popup and reloads the board after a successful change.
// A failure goes back to the popup to show.
func (m BoardModel) finishEdit(msg tea.Msg, e issueEdit) (tea.Model, tea.Cmd) {
	if e.err != nil {
		return m.forwardToPopup(msg)
	}
	m.popup = nil
	m.status = e.status
	return m, m.runWithSpinner(m.board.reload(e.key))
}

func (m BoardModel) forwardToPopup(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, m.layout.forwardToPopup(msg, m.board.loading || m.picker.loading())
}
