// Package tui implements jt's bubbletea interface.
package tui

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/components/boardpicker"
	"github.com/hawkbawk/jt/internal/components/projectpicker"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/popup"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
)

// Backend is the Jira functionality the TUI depends on.
type Backend interface {
	DefaultProject() string
	SetDefaultProject(project string)
	SetDefaultBoard(id int)
	Projects() ([]jira.Project, error)
	IssueTypes() ([]string, error)
	Search(jql string) ([]jira.Issue, error)
	Boards(project string) ([]jira.BoardSummary, error)
	Board(project string, boardID int, filterJQL string) (*jira.Board, error)
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
// board and at most one popup, and controls high-level or broadly used state.
type BoardModel struct {
	layout

	board   boardPane
	actions issueActions

	// projects is the list the project popup loaded, kept for the session.
	projects []jira.Project
}

// New builds the root model. The backend's default project, if any, is
// opened on start.
func New(backend Backend, settings Settings) BoardModel {
	m := BoardModel{
		layout:  newLayout(settings),
		board:   newBoardPane(backend),
		actions: newIssueActions(backend),
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
		return popup.Open(m.newProjectPopup(false))
	}
	return m.runWithSpinner(m.board.fetch(m.board.project, ""))
}

// busy reports whether anything is waiting on the network.
func (m BoardModel) busy() bool {
	return m.board.loading
}

// Update implements tea.Model.
func (m BoardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case issueEditedMsg:
		m.status = msg.status
		return m, m.runWithSpinner(m.board.reload(msg.key))

	case tea.WindowSizeMsg:
		return m, m.resize(msg)

	case tuimsg.ThemeChanged:
		m.restyle()
		return m, m.updatePopup(msg)

	case themeSelectedMsg:
		return m, m.selectTheme(msg)

	case themeSavedMsg:
		m.onThemeSaved(msg)
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case spinner.TickMsg:
		return m, tea.Batch(m.tick(msg, m.busy()), m.updatePopup(msg))

	case popup.OpenMsg, popup.CloseMsg:
		return m, m.updatePopup(msg)

	case projectpicker.SelectedMsg:
		m.status = ""
		return m, m.runWithSpinner(m.board.open(msg.Key))

	case projectpicker.LoadedMsg:
		if msg.Err == nil {
			m.projects = msg.Projects
		}
		return m, m.updatePopup(msg)

	case boardpicker.SelectedMsg:
		return m.selectBoard(msg)

	case boardSavedMsg:
		if msg.err != nil {
			m.status = styles.Error().Render("saving board: " + msg.err.Error())
		} else {
			m.board.backend.SetDefaultBoard(msg.id)
			m.status = styles.Dim().Render("Saved as the default board")
		}
		return m, nil

	case projectpicker.FavoriteMsg:
		return m, saveProject(m.settings, msg.Key)

	case projectSavedMsg:
		if msg.err != nil {
			m.status = styles.Error().Render("saving project: " + msg.err.Error())
			return m, nil
		}
		m.board.backend.SetDefaultProject(msg.key)
		m.status = styles.Dim().Render("Saved " + msg.key + " as the default project")
		if p, ok := m.popup.Content().(*projectpicker.Picker); ok {
			return m, p.Refresh()
		}
		return m, nil

	case boardLoadedMsg:
		m.board.update(msg)
		return m, nil

	case filterSelectedMsg:
		m.status = ""
		return m, m.runWithSpinner(m.board.setFilter(msg.filter))

	case actionDoneMsg:
		m.status = msg.display()
		return m, nil
	}
	// A popup's responses and form run on their own messages, so it gets
	// what's left.
	return m, m.updatePopup(msg)
}

// selectBoard opens the picked board.
func (m BoardModel) selectBoard(msg boardpicker.SelectedMsg) (tea.Model, tea.Cmd) {
	if msg.Project != m.board.project {
		return m, nil
	}
	m.status = ""
	return m, m.runWithSpinner(m.board.openBoard(msg.Board.ID))
}

func (m BoardModel) newProjectPopup(cancellable bool) *projectpicker.Picker {
	return projectpicker.New(m.board.backend, m.projects, cancellable)
}

func (m BoardModel) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if consumed, quit := m.quit.handle(msg); consumed {
		if quit {
			return m, tea.Quit
		}
		return m, nil
	}

	switch {
	case key.Matches(msg, keys.projects):
		// A popup may hold unsaved work, so it has to be closed first.
		if m.popup.IsOpen() {
			return m, nil
		}
		return m, popup.Open(m.newProjectPopup(m.board.nav != nil))
	case m.popup.Captures(msg):
		return m, m.updatePopup(msg)
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
		return m, popup.Open(newThemePopup())
	}
	if key.Matches(msg, keys.boards) && m.board.nav != nil {
		return m, popup.Open(boardpicker.New(m.board.backend, m.board.project, m.board.nav.board.ID))
	}
	if key.Matches(msg, keys.favorite) && m.board.nav != nil {
		if m.board.project != m.board.backend.DefaultProject() {
			m.status = styles.Warn().Render("Only the default project's board can be the default")
			return m, nil
		}
		return m, saveBoard(m.settings, m.board.nav.board.ID)
	}
	if key.Matches(msg, keys.filter) && m.board.nav != nil {
		return m, popup.Open(newFilterPopup(m.board.quick, m.board.assignees, m.board.filter))
	}
	if sel == nil {
		return m, nil
	}
	return m, m.actions.handle(msg, *sel)
}
