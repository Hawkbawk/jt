// Package board is the board screen: the kanban view of a project's board, its
// pickers and the issue popups opened from it.
package board

import (
	"os/exec"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/command"
	"github.com/hawkbawk/jt/internal/components/boardpicker"
	"github.com/hawkbawk/jt/internal/components/commandpicker"
	"github.com/hawkbawk/jt/internal/components/filterpicker"
	"github.com/hawkbawk/jt/internal/components/issuecomments"
	"github.com/hawkbawk/jt/internal/components/projectpicker"
	"github.com/hawkbawk/jt/internal/components/themepicker"
	"github.com/hawkbawk/jt/internal/config"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/keys"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/popup"
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
// board and at most one popup, and controls high-level or broadly used state.
type Model struct {
	screen.Layout

	// settings is the full settings, which also saves the board and project.
	settings Settings
	board    boardPane
	actions  screen.IssueActions
	commands []config.Command

	// projects is the list the project popup loaded, kept for the session.
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
		return popup.Open(m.newProjectPopup(false))
	}
	return m.RunWithSpinner(m.board.fetch(m.board.project, ""))
}

// busy reports whether anything is waiting on the network.
func (m Model) busy() bool {
	return m.board.loading
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tuimsg.Finished:
		m.Status = msg.Status()
		return m, m.RunWithSpinner(m.board.reload(msg.IssueKey()))

	case tea.WindowSizeMsg:
		return m, m.Resize(msg)

	case tuimsg.ThemeChanged:
		m.Restyle()
		return m, m.UpdatePopup(msg)

	case themepicker.SelectedMsg:
		return m, m.SelectTheme(msg)

	case screen.ThemeSavedMsg:
		m.OnThemeSaved(msg)
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case spinner.TickMsg:
		return m, tea.Batch(m.Tick(msg, m.busy()), m.UpdatePopup(msg))

	case issuecomments.Closed:
		return m, popup.Close()

	case commandpicker.SelectedMsg:
		m.Status = styles.Dim().Render("Preparing " + msg.Command.DisplayName() + "…")
		return m, m.loadCommand(msg.Command)

	case commandLoadedMsg:
		if msg.err != nil {
			m.Status = styles.Error().Render("preparing " + msg.command.DisplayName() + ": " + msg.err.Error())
			return m, nil
		}
		args, err := command.RenderArgs(msg.command.Args, command.Data{
			Key: msg.issue.Key, IssueKey: msg.issue.Key, Project: msg.project,
			Board: msg.board, URL: msg.url, Title: msg.title,
			Description: msg.description, Comments: msg.comments,
		})
		if err != nil {
			m.Status = styles.Error().Render("preparing command: " + err.Error())
			return m, nil
		}
		return m, tea.ExecProcess(exec.Command(msg.command.Program(), args...), func(err error) tea.Msg {
			return commandFinishedMsg{command: msg.command, err: err}
		})

	case commandFinishedMsg:
		if msg.err != nil {
			m.Status = styles.Error().Render("running " + msg.command.DisplayName() + ": " + msg.err.Error())
		} else {
			m.Status = styles.Dim().Render("Ran " + msg.command.DisplayName())
		}
		return m, nil

	case popup.OpenMsg, popup.CloseMsg:
		return m, m.UpdatePopup(msg)

	case projectpicker.SelectedMsg:
		m.Status = ""
		return m, m.RunWithSpinner(m.board.open(msg.Key))

	case projectpicker.LoadedMsg:
		if msg.Err == nil {
			m.projects = msg.Projects
		}
		return m, m.UpdatePopup(msg)

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
		if p, ok := m.Popup.Content().(*projectpicker.Picker); ok {
			return m, p.Refresh()
		}
		return m, nil

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
	// A popup's responses and form run on their own messages, so it gets
	// what's left.
	return m, m.UpdatePopup(msg)
}

// selectBoard opens the picked board.
func (m Model) selectBoard(msg boardpicker.SelectedMsg) (tea.Model, tea.Cmd) {
	if msg.Project != m.board.project {
		return m, nil
	}
	m.Status = ""
	return m, m.RunWithSpinner(m.board.openBoard(msg.Board.ID))
}

func (m Model) newProjectPopup(cancellable bool) *projectpicker.Picker {
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
		// A popup may hold unsaved work, so it has to be closed first.
		if m.Popup.IsOpen() {
			return m, nil
		}
		return m, popup.Open(m.newProjectPopup(m.board.nav != nil))
	case m.Popup.Captures(msg):
		return m, m.UpdatePopup(msg)
	}
	return m.handleBoardKey(msg)
}

func (m Model) handleBoardKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, keys.Get().Quit) {
		return m, tea.Quit
	}
	if m.board.move(msg) {
		return m, nil
	}
	sel := m.board.selected()
	if key.Matches(msg, keys.Get().Commands) {
		if sel == nil {
			return m, nil
		}
		return m, popup.Open(commandpicker.New(m.commands))
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
		return m, popup.Open(themepicker.New())
	}
	if key.Matches(msg, keys.Get().Boards) && m.board.nav != nil {
		return m, popup.Open(boardpicker.New(m.board.backend, m.board.project, m.board.nav.board.ID))
	}
	if key.Matches(msg, keys.Get().Filter) && m.board.nav != nil {
		return m, popup.Open(filterpicker.New(m.board.quick, m.board.assignees, m.board.filter))
	}
	if sel == nil {
		return m, nil
	}
	return m, m.actions.Handle(msg, *sel)
}

type commandLoadedMsg struct {
	command             config.Command
	issue               jira.Issue
	project, board, url string
	title, description  string
	comments            []jira.Comment
	err                 error
}

type commandFinishedMsg struct {
	command config.Command
	err     error
}

func (m Model) loadCommand(c config.Command) tea.Cmd {
	selected := m.board.selected()
	if selected == nil {
		return nil
	}
	issue := *selected
	project, board := m.board.nav.board.Project, m.board.nav.board.Name
	backend := m.board.backend
	return func() tea.Msg {
		detail, err := backend.Issue(issue.Key)
		if err != nil {
			return commandLoadedMsg{command: c, err: err}
		}
		comments, err := backend.Comments(issue.Key)
		if err != nil {
			return commandLoadedMsg{command: c, err: err}
		}
		title, description := issue.Summary, ""
		if detail != nil {
			if detail.Summary != "" {
				title = detail.Summary
			}
			description = detail.Description
		}
		var commentItems []jira.Comment
		if comments != nil {
			commentItems = comments.Items
		}
		return commandLoadedMsg{
			command: c, issue: issue, project: project, board: board,
			url: backend.IssueURL(issue.Key), title: title, description: description,
			comments: commentItems,
		}
	}
}
