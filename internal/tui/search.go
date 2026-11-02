package tui

import (
	"context"
	"errors"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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

// SearchModel is the search screen, with one huh form and the board's issue popups.
type SearchModel struct {
	layout
	backend Backend
	form    *searchForm
	actions issueActions
	results resultList

	searched  bool
	searching bool
	searchErr error
	// Cancellation saves work; the ID also rejects replies queued before cancellation.
	searchID     int
	lastJQL      string
	cancelSearch context.CancelFunc
}

// NewSearch builds the search screen with the backend's default project selected.
func NewSearch(backend Backend, settings Settings) SearchModel {
	return SearchModel{
		layout:  newLayout(settings),
		backend: backend,
		form:    newSearchForm(backend.DefaultProject()),
		actions: newIssueActions(backend),
	}
}

// WithWarning shows text in the status line until the next action replaces it.
func (m SearchModel) WithWarning(text string) SearchModel {
	m.status = styles.Warn().Render(text)
	return m
}

// Init implements tea.Model.
func (m SearchModel) Init() tea.Cmd {
	return tea.Batch(loadIssueTypes(m.backend), loadProjects(m.backend), m.form.form.Init())
}

func (m SearchModel) busy() bool { return m.popupLoading() || m.searching }

func (m *SearchModel) restyle() tea.Cmd {
	m.layout.restyle()
	m.form.restyle()
	return m.form.update(nil)
}

// Update implements tea.Model.
func (m SearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case transitionDoneMsg, assignDoneMsg, fieldsSavedMsg:
		return m.finishEdit(msg, editResult(msg))

	case tea.WindowSizeMsg:
		m.resize(msg)
		m.results.resize(m.width-2, m.bodyHeight())
		m.form.resize(m.width-2, m.bodyHeight())
		cmd := m.form.update(msg)
		return m, cmd

	case ThemeChangedMsg:
		cmd := m.restyle()
		return m, cmd

	case themeSelectedMsg:
		cmd := m.selectTheme(msg)
		cmd = tea.Batch(cmd, m.restyle())
		return m, cmd

	case themeSavedMsg:
		m.onThemeSaved(msg)
		return m, nil

	case spinner.TickMsg:
		cmd := m.tick(msg, m.busy())
		return m, cmd

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case issueTypesLoadedMsg:
		if msg.err != nil {
			m.status = styles.Error().Render("loading issue types: " + msg.err.Error())
			return m, nil
		}
		m.form.setIssueTypes(msg.types)
		cmd := m.form.update(nil)
		return m, cmd

	case projectsLoadedMsg:
		if msg.err != nil {
			m.status = styles.Error().Render("loading projects: " + msg.err.Error())
			return m, nil
		}
		m.form.setProjects(msg.projects)
		cmd := m.form.update(nil)
		return m, cmd

	case searchLoadedMsg:
		cmd := m.applyResults(msg)
		return m, cmd

	case actionDoneMsg:
		m.status = msg.display()
		return m, nil
	}
	cmd := m.forwardMsg(msg)
	return m, cmd
}

func (m *SearchModel) forwardMsg(msg tea.Msg) tea.Cmd {
	if m.popup != nil {
		return m.forwardToPopup(msg, m.searching)
	}
	return m.form.update(msg)
}

func (m SearchModel) currentJQL() (string, error) {
	if m.form.advanced {
		jql := strings.TrimSpace(m.form.jql)
		if jql == "" {
			return "", errors.New("write some JQL to search with")
		}
		return jql, nil
	}
	q, err := m.form.query(m.form.now())
	if err != nil {
		return "", err
	}
	return q.String(), nil
}

func (m *SearchModel) submit() tea.Cmd {
	jql, err := m.currentJQL()
	if err != nil {
		m.status = styles.Error().Render(err.Error())
		return nil
	}
	m.status = ""
	return m.search(jql, "")
}

func (m *SearchModel) search(jql, focusKey string) tea.Cmd {
	m.stopSearch()
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelSearch = cancel
	m.searchID++
	m.searching, m.searchErr, m.lastJQL = true, nil, jql
	return m.runWithSpinner(runSearch(ctx, m.backend, m.searchID, jql, focusKey))
}

func (m *SearchModel) stopSearch() {
	if m.cancelSearch != nil {
		m.cancelSearch()
		m.cancelSearch = nil
	}
}

func (m *SearchModel) applyResults(msg searchLoadedMsg) tea.Cmd {
	if msg.id != m.searchID {
		return nil
	}
	m.stopSearch()
	m.searching, m.searched, m.searchErr = false, true, msg.err
	if msg.err != nil {
		m.status = styles.Error().Render("searching: " + msg.err.Error())
		m.results.set(nil, "")
	} else {
		m.results.set(msg.issues, msg.focusKey)
	}
	focus := m.firstField()
	if len(m.results.issues) > 0 {
		focus = fieldResults
	}
	return m.form.build(&m.results, m.resultsMessage(), focus)
}

func (m SearchModel) resultsMessage() string {
	switch {
	case m.searchErr != nil:
		return "Error: " + m.searchErr.Error()
	case m.searched && len(m.results.issues) == 0:
		return "No issues found."
	}
	return ""
}

func (m SearchModel) firstField() string {
	if m.form.advanced {
		return fieldJQL
	}
	return fieldProject
}

func (m SearchModel) resultsFocused() bool { return m.form.focused() == fieldResults }

func (m SearchModel) finishEdit(msg tea.Msg, e issueEdit) (tea.Model, tea.Cmd) {
	if e.err != nil {
		cmd := m.forwardToPopup(msg, m.searching)
		return m, cmd
	}
	m.popup = nil
	m.status = e.status
	cmd := m.search(m.lastJQL, e.key)
	return m, cmd
}

func (m *SearchModel) toggleMode() tea.Cmd {
	m.form.advanced = !m.form.advanced
	if m.form.advanced && strings.TrimSpace(m.form.jql) == "" {
		if q, err := m.form.query(m.form.now()); err == nil {
			m.form.jql = q.String()
		}
	}
	return m.form.build(&m.results, m.resultsMessage(), m.firstField())
}

func (m SearchModel) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if consumed, quit := m.quit.handle(msg); consumed {
		if quit {
			m.stopSearch()
			return m, tea.Quit
		}
		return m, nil
	}
	if m.popup != nil {
		cmd := m.forwardToPopup(msg, m.searching)
		return m, cmd
	}
	switch {
	case key.Matches(msg, modeKey):
		cmd := m.toggleMode()
		return m, cmd
	case key.Matches(msg, keys.send):
		cmd := m.submit()
		return m, cmd
	case m.resultsFocused():
		return m.handleResultsKey(msg)
	case key.Matches(msg, keys.accept) && (m.form.advanced || m.form.focused() == fieldUpdatedTo):
		cmd := m.submit()
		return m, cmd
	case key.Matches(msg, keys.nextTab) && len(m.results.issues) == 0 &&
		(m.form.advanced || m.form.focused() == fieldUpdatedTo):
		cmd := m.form.focus(m.firstField())
		return m, cmd
	}
	cmd := m.form.update(msg)
	return m, cmd
}

func (m SearchModel) handleResultsKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.quit):
		m.stopSearch()
		return m, tea.Quit
	case key.Matches(msg, keys.blur, keys.nextTab) || msg.String() == "/":
		cmd := m.form.focus(m.firstField())
		return m, cmd
	case key.Matches(msg, keys.refresh) && m.lastJQL != "" && !m.searching:
		focusKey := ""
		if sel := m.results.selected(); sel != nil {
			focusKey = sel.Key
		}
		m.status = ""
		cmd := m.search(m.lastJQL, focusKey)
		return m, cmd
	case key.Matches(msg, keys.theme):
		cmd := m.openPopup(newThemePopup(), nil)
		return m, cmd
	}
	if sel := m.results.selected(); sel != nil {
		if key.Matches(msg, keys.accept) {
			msg = tea.KeyPressMsg{Code: 'v', Text: "v"}
		}
		p, cmd := m.actions.handle(msg, *sel)
		if p != nil {
			cmd = m.openPopup(p, cmd)
			return m, cmd
		}
		if cmd != nil {
			return m, cmd
		}
	}
	cmd := m.form.update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m SearchModel) View() tea.View { return m.withView(m.render()) }

func (m SearchModel) render() string {
	if m.width == 0 {
		return ""
	}
	help := helpKeys(append(m.form.form.KeyBinds(), formSearchHelp, modeKey))
	if m.form.advanced {
		help = append(help, jqlSearchHelp)
	}
	if m.resultsFocused() {
		help = searchResultsHelp
	}
	body := lipgloss.NewStyle().Padding(0, 1).Width(m.width).MaxWidth(m.width).
		Height(m.bodyHeight()).MaxHeight(m.bodyHeight()).Render(m.form.form.View())
	if m.popup != nil {
		body = overlay(body, m.popup.view(m.spinner.View()), m.width)
	}
	mode := "basic"
	if m.form.advanced {
		mode = "advanced"
	}
	parts := []string{"search", styles.Dim().Render(mode)}
	if m.searching {
		parts = append(parts, m.spinner.View()+" Searching…")
	}
	content := lipgloss.JoinVertical(lipgloss.Left, m.headerView(parts), body, m.footerView(help))
	return lipgloss.NewStyle().MaxWidth(m.width).MaxHeight(m.height).Render(content)
}
