package tui

import (
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/hawkbawk/jt/internal/styles"
)

type issueTypesLoadedMsg struct {
	types []string
	err   error
}

func loadIssueTypes(b Backend) tea.Cmd {
	return func() tea.Msg {
		ts, err := b.IssueTypes()
		return issueTypesLoadedMsg{types: ts, err: err}
	}
}

// SearchModel is the root model of the search screen. It shares its chrome and
// project picker with the board.
type SearchModel struct {
	chrome
	backend Backend
	picker  picker
	form    *searchForm
	now     func() time.Time
}

// NewSearch builds the search screen. The backend's default project is
// selected to start with.
func NewSearch(backend Backend, settings Settings) SearchModel {
	m := SearchModel{
		chrome:  newChrome(settings),
		backend: backend,
		picker:  newPicker(backend),
		form:    newSearchForm(backend.DefaultProject()),
		now:     time.Now,
	}
	return m
}

// WithWarning shows text in the status line until the next action replaces it.
func (m SearchModel) WithWarning(text string) SearchModel {
	m.status = styles.Warn().Render(text)
	return m
}

// Init implements tea.Model.
func (m SearchModel) Init() tea.Cmd {
	return tea.Batch(loadIssueTypes(m.backend), m.form.focusCmd())
}

func (m SearchModel) busy() bool {
	return m.popupLoading() || m.picker.loading()
}

func (m SearchModel) restyle() SearchModel {
	m.chrome.restyle()
	m.picker.restyle()
	m.form.restyle()
	return m
}

// Update implements tea.Model.
func (m SearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.resize(msg)
		m.picker.setSize(m.width, m.bodyHeight())
		return m, nil

	case ThemeChangedMsg:
		return m.restyle(), nil

	case themeSelectedMsg:
		cmd := m.selectTheme(msg)
		return m.restyle(), cmd

	case themeSavedMsg:
		m.themeSaved(msg)
		return m, nil

	case spinner.TickMsg:
		return m, m.tick(msg, m.busy())

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case issueTypesLoadedMsg:
		if msg.err != nil {
			m.status = styles.Error().Render("loading issue types: " + msg.err.Error())
			return m, nil
		}
		m.form.setIssueTypes(msg.types)
		return m, nil

	case openProjectsMsg:
		return m, m.startBusy(m.picker.show(true))

	case projectSelectedMsg:
		m.form.setProject(msg.key)
		return m, nil
	}
	cmd := tea.Batch(m.picker.update(msg), m.forwardMsg(msg))
	return m, cmd
}

// forwardMsg gives messages nothing above claimed, such as cursor blinks, to
// the popup or the form.
func (m *SearchModel) forwardMsg(msg tea.Msg) tea.Cmd {
	if m.popup != nil {
		return m.forwardToPopup(msg, m.picker.loading())
	}
	if m.picker.open {
		return nil
	}
	return m.form.update(msg)
}

func (m SearchModel) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if consumed, quit := m.quit.handle(msg); consumed {
		if quit {
			return m, tea.Quit
		}
		return m, nil
	}
	switch {
	case m.picker.open:
		return m, m.picker.update(msg)
	case m.popup != nil:
		return m, m.forwardToPopup(msg, m.picker.loading())
	}
	return m, m.form.update(msg)
}

// View implements tea.Model.
func (m SearchModel) View() tea.View {
	return m.newView(m.render())
}

func (m SearchModel) render() string {
	if m.width == 0 {
		return ""
	}
	body := lipgloss.NewStyle().Width(m.width).Height(m.bodyHeight()).MaxHeight(m.bodyHeight()).
		Padding(1, 1, 0).Render(m.formView())
	switch {
	case m.picker.open:
		body = overlay(body, m.picker.view(m.spinner.View()), m.width)
	case m.popup != nil:
		body = overlay(body, m.popup.view(m.spinner.View()), m.width)
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		m.headerView([]string{"search"}), body, m.footerView(searchFormHelp))
}

// formView draws the form above a preview of the JQL it stands for.
func (m SearchModel) formView() string {
	inner := m.width - 2
	var preview string
	if q, err := m.form.query(m.now()); err != nil {
		preview = styles.Error().Render(ansi.Truncate("Error: "+err.Error(), inner, "…"))
	} else {
		preview = styles.Dim().Render(ansi.Truncate("JQL: "+q.JQL(), inner, "…"))
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.form.view(inner), "", preview)
}
