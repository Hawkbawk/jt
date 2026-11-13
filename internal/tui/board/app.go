// Package board is the board screen: the kanban view of a project's board, its
// pickers and the issue modals opened from it.
package board

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/components/boardpicker"
	"github.com/hawkbawk/jt/internal/components/commandpicker"
	"github.com/hawkbawk/jt/internal/components/filterpicker"
	"github.com/hawkbawk/jt/internal/components/issuecreate"
	"github.com/hawkbawk/jt/internal/components/projectpicker"
	"github.com/hawkbawk/jt/internal/components/themepicker"
	"github.com/hawkbawk/jt/internal/config"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/keys"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/modal"
	"github.com/hawkbawk/jt/internal/tui/screen"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
)

// Backend is the Jira functionality the board depends on.
type Backend interface {
	screen.IssueBackend
	DefaultProject() string
	SetDefaultProject(project string)
	SetDefaultBoard(id int)
	Projects() ([]jira.Project, error)
	Boards(project string) ([]jira.BoardSummary, error)
	Board(project string, boardID int, filterJQL string) (*jira.Board, error)
}

// Model is the board's root bubbletea model. It routes input and messages to the
// board and at most one modal, and controls high-level or broadly used state.
type Model struct {
	screen.Layout

	// settings is the full settings, which also saves the board and project.
	settings Settings
	board    boardPane
	actions  screen.IssueActions
	commands []config.Command
	keys     boardKeyMap

	// projects is the list the project modal loaded, kept for the session.
	projects []jira.Project
}

// New builds the root model. The backend's default project, if any, is
// opened on start.
func New(backend Backend, settings Settings, commands ...config.Command) Model {
	m := Model{
		Layout:   screen.NewLayout(settings),
		settings: settings,
		board:    newBoardPane(backend),
		actions:  screen.NewIssueActions(backend),
		commands: commands,
		keys:     newBoardKeys(),
	}
	return m
}

// WithWarning shows text in the status line until the next action replaces it.
func (m Model) WithWarning(text string) Model {
	m.Status = styles.Warn().Render(text)
	return m
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	if m.board.project == "" {
		return modal.Open(m.newProjectModal(false))
	}
	return m.RunWithSpinner(m.board.fetch(m.board.project, ""))
}

// busy reports whether anything is waiting on the network.
func (m Model) busy() bool {
	return m.board.loading
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if cmd, status, handled := commandpicker.Handle(m.board.backend, msg); handled {
		m.Status = status
		return m, cmd
	}
	if k, ok := msg.(tea.KeyPressMsg); ok {
		return m.handleKey(k)
	}
	if m.Modal.Routes(msg) {
		return m, m.UpdateModal(msg)
	}
	switch msg := msg.(type) {
	case tuimsg.Finished:
		m.Status = msg.Status()
		return m, m.RunWithSpinner(m.board.reload(msg.IssueKey()))

	case tea.WindowSizeMsg:
		return m, m.Resize(msg, m.keys)

	case tuimsg.ThemeChanged:
		m.Restyle()
		return m, m.UpdateModal(msg)

	case themepicker.SelectedMsg:
		return m, m.SelectTheme(msg)

	case screen.ThemeSavedMsg:
		m.OnThemeSaved(msg)
		return m, nil

	case spinner.TickMsg:
		return m, m.Tick(msg, m.busy())

	case projectpicker.SelectedMsg:
		m.Status = ""
		return m, m.RunWithSpinner(m.board.open(msg.Key))

	case projectpicker.LoadedMsg:
		if msg.Err == nil {
			m.projects = msg.Projects
		}
		return m, nil

	case boardpicker.SelectedMsg:
		return m.selectBoard(msg)

	case boardpicker.FavoriteMsg:
		if m.board.project != m.board.backend.DefaultProject() {
			m.Status = styles.Warn().Render("Only the default project's board can be the default")
			return m, nil
		}
		return m, saveBoard(m.settings, msg.ID)

	case boardSavedMsg:
		if msg.err != nil {
			m.Status = styles.Error().Render("saving board: " + msg.err.Error())
		} else {
			m.board.backend.SetDefaultBoard(msg.id)
			m.Status = styles.Dim().Render("Saved as the default board")
		}
		return m, nil

	case projectpicker.FavoriteMsg:
		return m, saveProject(m.settings, msg.Key)

	case projectSavedMsg:
		if msg.err != nil {
			m.Status = styles.Error().Render("saving project: " + msg.err.Error())
			return m, nil
		}
		m.board.backend.SetDefaultProject(msg.key)
		m.Status = styles.Dim().Render("Saved " + msg.key + " as the default project")
		return m, m.UpdateModal(projectpicker.DefaultChanged{})

	case boardLoadedMsg:
		m.board.update(msg)
		return m, nil

	case filterpicker.SelectedMsg:
		m.Status = ""
		return m, m.RunWithSpinner(m.board.setFilter(msg.Filter))

	case screen.ActionDoneMsg:
		m.Status = msg.Display()
		return m, nil
	}
	// What's left may be a result the program sends straight to the screen, such
	// as the editor a form ran, so the open modal gets to see it.
	return m, m.UpdateModal(msg)
}

// selectBoard opens the picked board.
func (m Model) selectBoard(msg boardpicker.SelectedMsg) (tea.Model, tea.Cmd) {
	if msg.Project != m.board.project {
		return m, nil
	}
	m.Status = ""
	return m, m.RunWithSpinner(m.board.openBoard(msg.Board.ID))
}

func (m Model) newProjectModal(cancellable bool) *projectpicker.Picker {
	return projectpicker.New(m.board.backend, m.projects, cancellable)
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if consumed, quit := m.Quit.Handle(msg); consumed {
		if quit {
			return m, tea.Quit
		}
		return m, nil
	}

	switch {
	case key.Matches(msg, keys.Get().Projects):
		// A modal may hold unsaved work, so it has to be closed first.
		if m.Modal.IsOpen() {
			return m, nil
		}
		return m, modal.Open(m.newProjectModal(m.board.nav != nil))
	case m.Modal.Captures(msg):
		return m, m.UpdateModal(msg)
	}
	return m.handleBoardKey(msg)
}

func (m Model) handleBoardKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, keys.Get().Quit) {
		return m, tea.Quit
	}
	if key.Matches(msg, keys.Get().Help) {
		m.ToggleHelp()
		return m, nil
	}
	if m.board.move(msg) {
		return m, nil
	}
	sel := m.board.selected()
	if key.Matches(msg, keys.Get().Commands) {
		if sel == nil {
			return m, nil
		}
		return m, modal.Open(commandpicker.New(m.commands, *sel, m.board.nav.board.Project, m.board.nav.board.Name))
	}
	if key.Matches(msg, keys.Get().Refresh) && m.board.project != "" && !m.board.loading {
		focusKey := ""
		if sel != nil {
			focusKey = sel.Key
		}
		m.Status = ""
		return m, m.RunWithSpinner(m.board.reload(focusKey))
	}
	if key.Matches(msg, keys.Get().Theme) {
		return m, modal.Open(themepicker.New())
	}
	if key.Matches(msg, keys.Get().Create) {
		project := m.board.project
		if project == "" {
			project = m.board.backend.DefaultProject()
		}
		return m, modal.Open(issuecreate.New(m.board.backend, project))
	}
	if key.Matches(msg, keys.Get().Boards) && m.board.nav != nil {
		return m, modal.Open(boardpicker.New(m.board.backend, m.board.project, m.board.nav.board.ID))
	}
	if key.Matches(msg, keys.Get().Filter) && m.board.nav != nil {
		return m, modal.Open(filterpicker.New(m.board.quick, m.board.assignees, m.board.filter))
	}
	if sel == nil {
		return m, nil
	}
	return m, m.actions.Handle(msg, *sel)
}
