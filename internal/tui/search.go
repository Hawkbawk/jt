package tui

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
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
	jql     *jqlEditor
	// advanced is set while writing JQL rather than filling in the form.
	advanced bool
	now      func() time.Time
}

// NewSearch builds the search screen. The backend's default project is
// selected to start with.
func NewSearch(backend Backend, settings Settings) SearchModel {
	m := SearchModel{
		chrome:  newChrome(settings),
		backend: backend,
		picker:  newPicker(backend),
		form:    newSearchForm(backend.DefaultProject()),
		jql:     newJQLEditor(),
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
	m.jql.restyle()
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
	return m.focused(msg)
}

// focused gives msg to whichever mode is showing.
func (m *SearchModel) focused(msg tea.Msg) tea.Cmd {
	if m.advanced {
		return m.jql.update(msg)
	}
	return m.form.update(msg)
}

// toggleMode switches between the form and the JQL editor. Entering the editor
// with nothing written starts from the JQL the form stands for.
func (m *SearchModel) toggleMode() tea.Cmd {
	m.advanced = !m.advanced
	if !m.advanced {
		m.jql.blur()
		return m.form.focusCmd()
	}
	if strings.TrimSpace(m.jql.value()) == "" {
		if q, err := m.form.query(m.now()); err == nil {
			m.jql.setValue(q.JQL())
		}
	}
	return m.jql.focus()
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
	if key.Matches(msg, modeKey) {
		return m, m.toggleMode()
	}
	return m, m.focused(msg)
}

// View implements tea.Model.
func (m SearchModel) View() tea.View {
	return m.newView(m.render())
}

func (m SearchModel) render() string {
	if m.width == 0 {
		return ""
	}
	top := m.formView()
	help := searchFormHelp
	if m.advanced {
		top, help = m.jql.view(m.width-2), searchJQLHelp
	}
	body := lipgloss.NewStyle().Width(m.width).Height(m.bodyHeight()).MaxHeight(m.bodyHeight()).
		Padding(1, 1, 0).Render(top)
	switch {
	case m.picker.open:
		body = overlay(body, m.picker.view(m.spinner.View()), m.width)
	case m.popup != nil:
		body = overlay(body, m.popup.view(m.spinner.View()), m.width)
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		m.headerView([]string{"search", styles.Dim().Render(m.modeName())}), body, m.footerView(help))
}

func (m SearchModel) modeName() string {
	if m.advanced {
		return "advanced"
	}
	return "basic"
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
