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

	project  string
	projects []jira.Project

	picker     picker
	pickerOpen bool

	popup popup
	md    markdownRenderer

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
	m := Model{
		backend: backend,
		cfg:     cfg,
		cfgPath: cfgPath,
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(titleStyle)),
		project: cfg.DefaultProject,
		md:      glamourRenderer(glamourStyle()),
	}
	if m.project == "" {
		m.picker = newPicker(false)
		m.pickerOpen = true
	}
	return m
}

func glamourStyle() string {
	if lipgloss.HasDarkBackground() {
		return "dark"
	}
	return "light"
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	if m.project == "" {
		return tea.Batch(m.loadProjects(), m.spinner.Tick)
	}
	return m.startLoadBoard(m.project, "")
}

func (m *Model) openPicker() tea.Cmd {
	m.picker = newPicker(m.hasBoard)
	m.pickerOpen = true
	m.resizePicker()
	if m.projects != nil {
		m.picker.setProjects(m.projects, m.cfg.DefaultProject)
		return nil
	}
	return tea.Batch(m.loadProjects(), m.spinner.Tick)
}

func (m Model) loadProjects() tea.Cmd {
	backend := m.backend
	return func() tea.Msg {
		ps, err := backend.Projects()
		return projectsLoadedMsg{projects: ps, err: err}
	}
}

func (m *Model) resizePicker() {
	if !m.pickerOpen {
		return
	}
	m.picker.setSize(m.pickerSize())
}

func (m Model) pickerSize() (int, int) {
	w, h := popupSize(m.width, m.bodyHeight())
	return min(w, 80), h
}

// busy reports whether anything is waiting on the network.
func (m Model) busy() bool {
	if p, ok := m.popup.(interface{ loading() bool }); ok && p.loading() {
		return true
	}
	return m.loading || (m.pickerOpen && !m.picker.loaded && m.picker.err == nil)
}

func (m Model) bodyHeight() int {
	return max(0, m.height-2)
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
		m.resizePicker()
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

	case projectsLoadedMsg:
		if msg.err != nil {
			m.picker.err = msg.err
			return m, nil
		}
		m.projects = msg.projects
		m.picker.setProjects(msg.projects, m.cfg.DefaultProject)
		return m, nil

	case issueLoadedMsg, transitionsLoadedMsg:
		return m.forwardToPopup(msg)

	case transitionDoneMsg:
		if msg.err != nil {
			return m.forwardToPopup(msg)
		}
		m.popup = nil
		m.status = "Moved " + msg.key + " to " + msg.to
		return m, m.startLoadBoard(m.project, msg.key)

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

	if msg.String() == "ctrl+p" && !m.pickerOpen {
		m.popup = nil
		return m, m.openPicker()
	}
	if m.pickerOpen {
		return m.handlePickerKey(msg)
	}
	if m.popup != nil {
		return m.forwardToPopup(msg)
	}
	return m.handleBoardKey(msg)
}

func (m Model) forwardToPopup(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.popup == nil {
		return m, nil
	}
	var cmd tea.Cmd
	m.popup, cmd = m.popup.update(msg)
	return m, cmd
}

func (m *Model) openPopup(p popup, load tea.Cmd) tea.Cmd {
	p.setSize(popupSize(m.width, m.bodyHeight()))
	m.popup = p
	return tea.Batch(load, m.spinner.Tick)
}

func (m Model) handlePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.picker.filtering() {
		return m, m.picker.update(msg)
	}
	switch msg.String() {
	case "esc":
		if m.picker.list.IsFiltered() {
			break
		}
		if m.picker.cancellable {
			m.pickerOpen = false
		}
		return m, nil
	case "enter":
		p, ok := m.picker.selected()
		if !ok {
			return m, nil
		}
		m.pickerOpen = false
		m.project = p.Key
		m.hasBoard = false
		m.status = ""
		return m, m.startLoadBoard(p.Key, "")
	case " ":
		p, ok := m.picker.selected()
		if !ok {
			return m, nil
		}
		m.setDefaultProject(p.Key)
		return m, nil
	}
	return m, m.picker.update(msg)
}

func (m *Model) setDefaultProject(key string) {
	prev := m.cfg.DefaultProject
	m.cfg.DefaultProject = key
	if err := m.cfg.Save(m.cfgPath); err != nil {
		m.cfg.DefaultProject = prev
		m.status = errorStyle.Render("Saving config: " + err.Error())
		return
	}
	m.picker.markDefault(key)
	m.status = "Default project set to " + key
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
	case "v":
		if sel := m.nav.selected(); sel != nil {
			return m, m.openPopup(newTextPopup("Details", sel.Key, renderDetail, m.md), loadIssue(m.backend, sel.Key))
		}
	case "c":
		if sel := m.nav.selected(); sel != nil {
			return m, m.openPopup(newTextPopup("Comments", sel.Key, renderComments, m.md), loadIssue(m.backend, sel.Key))
		}
	case "m":
		if sel := m.nav.selected(); sel != nil {
			return m, m.openPopup(newMovePopup(m.backend, sel.Key), loadTransitions(m.backend, sel.Key))
		}
	}
	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	if m.width == 0 {
		return ""
	}
	body := m.bodyView(m.bodyHeight())
	if m.pickerOpen {
		w, h := m.pickerSize()
		body = overlay(body, m.picker.view(w, h, m.spinner.View()), m.width)
	} else if m.popup != nil {
		body = overlay(body, m.popup.view(m.spinner.View()), m.width)
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.headerView(), body, m.footerView())
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
	help := "hjkl navigate · v view · c comments · m move · ctrl+p projects · ctrl+d quit"
	if m.status != "" {
		help = m.status + dimStyle.Render(" · ") + help
	}
	return dimStyle.Padding(0, 1).Width(m.width).Render(help)
}
