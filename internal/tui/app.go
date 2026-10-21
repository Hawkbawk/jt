// Package tui implements jt's bubbletea interface.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ryanhawkins/jt/internal/config"
	"github.com/ryanhawkins/jt/internal/jira"
)

// Backend is the Jira functionality the TUI depends on.
type Backend interface {
	Projects() ([]jira.Project, error)
	Board(project string, boardID int) (*jira.Board, error)
	Issue(key string) (*jira.IssueDetail, error)
	Transitions(key string) ([]jira.Transition, error)
	Transition(key, transitionID string) error
}

type boardLoadedMsg struct {
	project  string
	board    *jira.Board
	focusKey string
	err      error
}

// Model is the root bubbletea model.
type Model struct {
	backend Backend
	cfg     *config.Config
	cfgPath string

	width, height int
	spinner       spinner.Model

	project     string
	loading     bool
	nav         boardNav
	hasBoard    bool
	status      string
	err         error
	confirmQuit bool
}

// New builds the root model. The configured default project, if any, is
// opened on start.
func New(backend Backend, cfg *config.Config, cfgPath string) Model {
	return Model{
		backend: backend,
		cfg:     cfg,
		cfgPath: cfgPath,
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(titleStyle)),
		project: cfg.DefaultProject,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	if m.project == "" {
		return nil
	}
	return m.startLoadBoard(m.project, "")
}

func (m *Model) startLoadBoard(project, focusKey string) tea.Cmd {
	m.loading = true
	m.err = nil
	boardID, _ := m.cfg.BoardFor(project)
	backend := m.backend
	load := func() tea.Msg {
		b, err := backend.Board(project, boardID)
		return boardLoadedMsg{project: project, board: b, focusKey: focusKey, err: err}
	}
	return tea.Batch(load, m.spinner.Tick)
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case spinner.TickMsg:
		if !m.loading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case boardLoadedMsg:
		if msg.project != m.project {
			return m, nil
		}
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.nav = newBoardNav(msg.board)
		m.hasBoard = true
		if msg.focusKey != "" {
			m.nav.focus(msg.focusKey)
		}
		return m, nil
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+d":
		return m, tea.Quit
	case "ctrl+c":
		if m.confirmQuit {
			return m, tea.Quit
		}
		m.confirmQuit = true
		return m, nil
	}

	if m.confirmQuit {
		switch msg.String() {
		case "y", "Y":
			return m, tea.Quit
		case "n", "N", "esc":
			m.confirmQuit = false
		}
		return m, nil
	}

	return m.handleBoardKey(msg)
}

func (m Model) handleBoardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if !m.hasBoard {
		return m, nil
	}
	switch msg.String() {
	case "h", "left":
		m.nav.left()
	case "l", "right":
		m.nav.right()
	case "k", "up":
		m.nav.up()
	case "j", "down":
		m.nav.down()
	}
	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	if m.width == 0 {
		return ""
	}
	header := m.headerView()
	footer := m.footerView()
	bodyHeight := m.height - lipgloss.Height(header) - lipgloss.Height(footer)
	return lipgloss.JoinVertical(lipgloss.Left, header, m.bodyView(bodyHeight), footer)
}

func (m Model) headerView() string {
	parts := []string{titleStyle.Render("jt")}
	if m.project != "" {
		parts = append(parts, m.project)
	}
	if m.hasBoard {
		parts = append(parts, dimStyle.Render(m.nav.board.Name))
	}
	if m.loading {
		parts = append(parts, m.spinner.View())
	}
	return headerStyle.Width(m.width).Render(strings.Join(parts, dimStyle.Render(" · ")))
}

func (m Model) bodyView(height int) string {
	var body string
	switch {
	case m.err != nil:
		body = errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	case m.hasBoard:
		return renderBoard(m.nav, m.width, height)
	case m.loading:
		body = m.spinner.View() + " Loading board…"
	default:
		body = dimStyle.Render("No project selected.")
	}
	return lipgloss.Place(m.width, height, lipgloss.Center, lipgloss.Center, body)
}

func (m Model) footerView() string {
	if m.confirmQuit {
		return errorStyle.Bold(true).Padding(0, 1).Render("Quit jt? (y/n, ctrl+c again to quit)")
	}
	help := "hjkl navigate · ctrl+d quit"
	if m.status != "" {
		help = m.status + dimStyle.Render(" · ") + help
	}
	return dimStyle.Padding(0, 1).Width(m.width).Render(help)
}
