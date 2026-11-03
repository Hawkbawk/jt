package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/popup"
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

// SearchModel is the search screen: a huh form, the results table and the
// board's issue popups. Only one search runs at a time.
type SearchModel struct {
	layout
	backend Backend
	form    *searchForm
	actions issueActions
	results resultList

	// inResults is whether the table rather than the form has focus.
	inResults bool
	searched  bool
	searching bool
	searchErr error
	lastJQL   string
	// refocus is where to put the results cursor after the next search, set
	// when an edit reruns it.
	refocus *resultCursor
}

// NewSearch builds the search screen with the backend's default project selected.
func NewSearch(backend Backend, settings Settings) SearchModel {
	m := SearchModel{
		layout:  newLayout(settings),
		backend: backend,
		form:    newSearchForm(backend.DefaultProject()),
		actions: newIssueActions(backend),
	}
	m.results.set(nil)
	return m
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
	m.results.restyle()
	return m.form.update(nil)
}

func (m SearchModel) resultsFocused() bool { return m.inResults && m.searched }

// mainHeight is the height left for the form and the results table with its
// title, after the message under them.
func (m SearchModel) mainHeight() int {
	h := m.bodyHeight()
	if m.resultsMessage() != "" {
		h--
	}
	return max(0, h)
}

// formHeight is all of the body until there are results to share it with.
func (m SearchModel) formHeight() int {
	if !m.searched {
		return m.mainHeight()
	}
	return min(maxFormHeight, m.mainHeight()/2)
}

// fit sizes the form and the table to the body.
func (m *SearchModel) fit() {
	w, formH := max(0, m.width-2), m.formHeight()
	m.form.resize(w, formH)
	m.results.resize(w, m.mainHeight()-formH-1)
}

// Update implements tea.Model.
func (m SearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case transitionDoneMsg, assignDoneMsg, fieldsSavedMsg:
		return m.finishEdit(msg, editResult(msg))

	case tea.WindowSizeMsg:
		popupCmd := m.resize(msg)
		m.fit()
		return m, tea.Batch(popupCmd, m.form.update(msg))

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
		return m, tea.Batch(m.tick(msg, m.busy()), m.updatePopup(msg))

	case popup.OpenMsg, popup.CloseMsg:
		return m, m.updatePopup(msg)

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
		m.applyResults(msg)
		return m, nil

	case actionDoneMsg:
		m.status = msg.display()
		return m, nil
	}
	cmd := m.forwardMsg(msg)
	return m, cmd
}

func (m *SearchModel) forwardMsg(msg tea.Msg) tea.Cmd {
	if m.popupOpen() {
		return m.forwardToPopup(msg, m.searching)
	}
	return m.updateForm(msg)
}

func (m SearchModel) currentJQL() (string, error) {
	if m.form.advanced {
		jql := strings.TrimSpace(m.form.jql)
		return jql, requireJQL(jql)
	}
	q, err := m.form.query(m.form.now())
	if err != nil {
		return "", err
	}
	return q.String(), nil
}

// sendKey searches from anywhere in the form. Pressing enter first makes huh
// validate the focused field, which then shows its own error and blocks the search.
func (m *SearchModel) sendKey() tea.Cmd {
	if m.searching {
		return nil
	}
	if m.resultsFocused() {
		return m.submit()
	}
	cmd := m.form.update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.form.form.GetFocusedField().Error() != nil {
		return cmd
	}
	return tea.Batch(cmd, m.submit())
}

// submit searches unless a search is already running, or the entered values
// don't make a query.
func (m *SearchModel) submit() tea.Cmd {
	if m.searching {
		return nil
	}
	jql, err := m.currentJQL()
	if err != nil {
		m.status = styles.Error().Render(err.Error())
		return nil
	}
	m.status = ""
	return m.search(jql)
}

func (m *SearchModel) search(jql string) tea.Cmd {
	m.searching, m.searchErr, m.lastJQL = true, nil, jql
	return m.runWithSpinner(runSearch(m.backend, jql))
}

func (m *SearchModel) applyResults(msg searchLoadedMsg) {
	m.searching, m.searched, m.searchErr = false, true, msg.err
	if msg.err != nil {
		m.results.set(nil)
	} else {
		m.results.set(msg.issues)
		if m.refocus != nil {
			m.results.restore(m.refocus.key, m.refocus.index)
		}
	}
	m.refocus = nil
	m.focusResults()
	m.fit()
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

func (m *SearchModel) focusResults() {
	m.inResults = true
	m.form.form.GetFocusedField().Blur()
}

func (m *SearchModel) focusForm() tea.Cmd {
	m.inResults = false
	return m.form.form.GetFocusedField().Focus()
}

func (m SearchModel) finishEdit(msg tea.Msg, e issueEdit) (tea.Model, tea.Cmd) {
	if e.err != nil {
		cmd := m.forwardToPopup(msg, m.searching)
		return m, cmd
	}
	m.legacy = nil
	m.status = e.status
	m.refocus = &resultCursor{key: e.key, index: m.results.table.GetHighlightedRowIndex()}
	cmd := m.search(m.lastJQL)
	return m, cmd
}

func (m *SearchModel) toggleMode() tea.Cmd {
	m.form.advanced = !m.form.advanced
	if m.form.advanced && strings.TrimSpace(m.form.jql) == "" {
		if q, err := m.form.query(m.form.now()); err == nil {
			m.form.jql = q.String()
		}
	}
	m.inResults = false
	cmd := m.form.build()
	m.fit()
	return cmd
}

func (m SearchModel) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if consumed, quit := m.quit.handle(msg); consumed {
		if quit {
			return m, tea.Quit
		}
		return m, nil
	}
	if m.popupOpen() {
		cmd := m.forwardToPopup(msg, m.searching)
		return m, cmd
	}
	switch {
	case key.Matches(msg, modeKey):
		cmd := m.toggleMode()
		return m, cmd
	case m.searched && !m.inResults && key.Matches(msg, keys.nextTab) && m.form.onLastField():
		m.focusResults()
		return m, nil
	case key.Matches(msg, focusKey) && m.searched:
		if m.inResults {
			cmd := m.focusForm()
			return m, cmd
		}
		m.focusResults()
		return m, nil
	case key.Matches(msg, keys.send):
		cmd := m.sendKey()
		return m, cmd
	case m.resultsFocused():
		return m.handleResultsKey(msg)
	}
	cmd := m.updateForm(msg)
	return m, cmd
}

// updateForm lets huh handle msg. Once huh finishes the form, it searches and
// starts over.
func (m *SearchModel) updateForm(msg tea.Msg) tea.Cmd {
	cmd := m.form.update(msg)
	if m.form.form.State == huh.StateNormal {
		return cmd
	}
	cmd = tea.Batch(cmd, m.form.build())
	m.fit()
	cmd = tea.Batch(cmd, m.submit())
	return cmd
}

func (m SearchModel) handleResultsKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.quit):
		return m, tea.Quit
	case key.Matches(msg, keys.back, keys.nextTab):
		cmd := m.focusForm()
		return m, cmd
	case key.Matches(msg, keys.top, keys.bottom):
		m.results.move(key.Matches(msg, keys.bottom))
		return m, nil
	case key.Matches(msg, keys.refresh):
		if m.lastJQL == "" || m.searching {
			return m, nil
		}
		m.status = ""
		cmd := m.search(m.lastJQL)
		return m, cmd
	case key.Matches(msg, keys.theme):
		return m, popup.Open(newThemePopup())
	}
	// Edits search again, which would race a search that's still running.
	if sel := m.results.selected(); sel != nil && !m.searching {
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
	cmd := m.results.update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m SearchModel) View() tea.View { return m.withView(m.render()) }

func (m SearchModel) bodyView() string {
	h := m.formHeight()
	box := lipgloss.NewStyle().Height(h).MaxHeight(h)
	parts := []string{box.Render(m.form.form.View())}
	if m.searched {
		title := styles.Title().Render(fmt.Sprintf("Results (%d)", len(m.results.issues)))
		parts = append(parts, title, m.results.view())
	}
	if msg := m.resultsMessage(); msg != "" {
		parts = append(parts, styles.Dim().Render(msg))
	}
	return lipgloss.NewStyle().Padding(0, 1).Width(m.width).MaxWidth(m.width).
		Height(m.bodyHeight()).MaxHeight(m.bodyHeight()).Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
}

func (m SearchModel) helpView() helpKeys {
	if m.resultsFocused() {
		return searchResultsHelp
	}
	help := helpKeys(m.form.form.KeyBinds())
	help = append(help, formSearchHelp, modeKey)
	if m.searched {
		help = append(help, focusKey)
	}
	if m.form.advanced {
		help = append(help, jqlSearchHelp)
	}
	return help
}

func (m SearchModel) render() string {
	if m.width == 0 {
		return ""
	}
	body := m.bodyView()
	switch {
	case m.legacy != nil:
		body = overlay(body, m.legacy.view(m.spinner.View()), m.width)
	default:
		body = m.popup.Overlay(body)
	}
	mode := "basic"
	if m.form.advanced {
		mode = "advanced"
	}
	parts := []string{"search", styles.Dim().Render(mode)}
	if m.searching {
		parts = append(parts, m.spinner.View()+" Searching…")
	}
	content := lipgloss.JoinVertical(lipgloss.Left, m.headerView(parts), body, m.footerView(m.helpView()))
	return lipgloss.NewStyle().MaxWidth(m.width).MaxHeight(m.height).Render(content)
}
