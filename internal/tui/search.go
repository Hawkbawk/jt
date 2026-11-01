package tui

import (
	"context"
	"errors"
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

// SearchModel is the root model of the search screen. It shares its layout and
// project picker with the board.
type SearchModel struct {
	layout
	backend Backend
	picker  picker
	form    *searchForm
	jql     *jqlEditor
	// advanced is set while writing JQL rather than filling in the form.
	advanced bool
	now      func() time.Time

	actions issueActions
	results resultList
	// resultsFocused is set while the issue list has the keys, not the form.
	resultsFocused bool
	searched       bool
	searching      bool
	searchErr      error
	// searchID tells results apart so a slow, older search can't replace a
	// newer one.
	searchID     int
	lastJQL      string
	cancelSearch context.CancelFunc
}

// NewSearch builds the search screen. The backend's default project is
// selected to start with.
func NewSearch(backend Backend, settings Settings) SearchModel {
	m := SearchModel{
		layout:  newLayout(settings),
		backend: backend,
		picker:  newPicker(backend),
		form:    newSearchForm(backend.DefaultProject()),
		jql:     newJQLEditor(),
		actions: newIssueActions(backend),
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
	return m.popupLoading() || m.otherBusy()
}

// otherBusy reports whether anything but the popup is waiting on the network.
func (m SearchModel) otherBusy() bool {
	return m.picker.loading() || m.searching
}

func (m SearchModel) restyle() SearchModel {
	m.layout.restyle()
	m.picker.restyle()
	m.form.restyle()
	m.jql.restyle()
	return m
}

// Update implements tea.Model.
func (m SearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case transitionDoneMsg, assignDoneMsg, fieldsSavedMsg:
		return m.finishEdit(msg, editResult(msg))

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
		m.onThemeSaved(msg)
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

	case submitSearchMsg:
		cmd := m.submit()
		return m, cmd

	case searchLoadedMsg:
		m.applyResults(msg)
		return m, nil

	case actionDoneMsg:
		m.status = msg.display()
		return m, nil

	case openProjectsMsg:
		cmd := m.runWithSpinner(m.picker.show(true))
		return m, cmd

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
		return m.forwardToPopup(msg, m.otherBusy())
	}
	if m.picker.open {
		return nil
	}
	return m.focused(msg)
}

// focused gives msg to whichever mode is showing.
func (m *SearchModel) focused(msg tea.Msg) tea.Cmd {
	switch {
	case m.resultsFocused:
		return nil
	case m.advanced:
		return m.jql.update(msg)
	}
	return m.form.update(msg)
}

// currentJQL is the JQL the form or the editor stands for.
func (m SearchModel) currentJQL() (string, error) {
	if m.advanced {
		jql := strings.TrimSpace(m.jql.value())
		if jql == "" {
			return "", errors.New("write some JQL to search with")
		}
		return jql, nil
	}
	q, err := m.form.query(m.now())
	if err != nil {
		return "", err
	}
	return q.String(), nil
}

// submit runs the search the form or the editor describes.
func (m *SearchModel) submit() tea.Cmd {
	jql, err := m.currentJQL()
	if err != nil {
		m.status = styles.Error().Render(err.Error())
		return nil
	}
	m.status = ""
	return m.search(jql, "")
}

// search runs jql and keeps the cursor on focusKey when that issue is found.
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

// applyResults shows a finished search and moves to the issues it found.
func (m *SearchModel) applyResults(msg searchLoadedMsg) {
	if msg.id != m.searchID {
		return
	}
	m.stopSearch()
	m.searching, m.searched = false, true
	if msg.err != nil {
		m.searchErr = msg.err
		m.results.set(nil, "")
		return
	}
	m.results.set(msg.issues, msg.focusKey)
	if len(msg.issues) > 0 {
		m.focusResults()
	}
}

func (m *SearchModel) focusResults() {
	m.resultsFocused = true
	m.form.blur()
	m.jql.blur()
}

// focusInput moves the keys back to the form or editor.
func (m *SearchModel) focusInput() tea.Cmd {
	m.resultsFocused = false
	if m.advanced {
		return m.jql.focus()
	}
	return m.form.focusCmd()
}

// finishEdit closes the popup and searches again after a successful change, so
// the list shows it. A failure goes back to the popup to show.
func (m SearchModel) finishEdit(msg tea.Msg, e issueEdit) (tea.Model, tea.Cmd) {
	if e.err != nil {
		cmd := m.forwardToPopup(msg, m.otherBusy())
		return m, cmd
	}
	m.popup = nil
	m.status = e.status
	cmd := m.search(m.lastJQL, e.key)
	return m, cmd
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
			m.jql.setValue(q.String())
		}
	}
	return m.jql.focus()
}

func (m SearchModel) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if consumed, quit := m.quit.handle(msg); consumed {
		if quit {
			m.stopSearch()
			return m, tea.Quit
		}
		return m, nil
	}
	switch {
	case m.picker.open:
		return m, m.picker.update(msg)
	case m.popup != nil:
		cmd := m.forwardToPopup(msg, m.otherBusy())
		return m, cmd
	}
	if key.Matches(msg, modeKey) {
		m.resultsFocused = false
		cmd := m.toggleMode()
		return m, cmd
	}
	if m.resultsFocused {
		return m.handleResultsKey(msg)
	}
	switch {
	case key.Matches(msg, keys.accept, keys.send) && m.advanced:
		cmd := m.submit()
		return m, cmd
	case key.Matches(msg, formNextKey) && len(m.results.issues) > 0 && (m.advanced || m.form.atEnd()):
		m.focusResults()
		return m, nil
	}
	return m, m.focused(msg)
}

func (m SearchModel) handleResultsKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	sel := m.results.selected()
	switch {
	case key.Matches(msg, keys.quit):
		m.stopSearch()
		return m, tea.Quit
	case m.results.move(msg):
		return m, nil
	case key.Matches(msg, keys.back):
		cmd := m.focusInput()
		return m, cmd
	case key.Matches(msg, keys.refresh) && m.lastJQL != "" && !m.searching:
		focusKey := ""
		if sel != nil {
			focusKey = sel.Key
		}
		m.status = ""
		cmd := m.search(m.lastJQL, focusKey)
		return m, cmd
	case key.Matches(msg, keys.theme):
		cmd := m.openPopup(newThemePopup(), nil)
		return m, cmd
	case sel == nil:
		return m, nil
	}
	p, cmd := m.actions.handle(msg, *sel)
	if p != nil {
		cmd = m.openPopup(p, cmd)
		return m, cmd
	}
	return m, cmd
}

// View implements tea.Model.
func (m SearchModel) View() tea.View {
	return m.withView(m.render())
}

func (m SearchModel) render() string {
	if m.width == 0 {
		return ""
	}
	top := m.formView()
	help := searchFormHelp
	switch {
	case m.resultsFocused:
		help = searchResultsHelp
	case m.advanced:
		help = searchJQLHelp
	}
	if m.advanced {
		top = m.jql.view(m.width - 2)
	}
	top = lipgloss.NewStyle().Padding(1, 1, 0).Render(top)
	rest := max(0, m.bodyHeight()-lipgloss.Height(top))
	body := lipgloss.NewStyle().Width(m.width).Height(m.bodyHeight()).MaxHeight(m.bodyHeight()).
		Render(lipgloss.JoinVertical(lipgloss.Left, top, m.resultsView(m.width-2, rest)))
	switch {
	case m.picker.open:
		body = overlay(body, m.picker.view(m.spinner.View()), m.width)
	case m.popup != nil:
		body = overlay(body, m.popup.view(m.spinner.View()), m.width)
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		m.headerView([]string{"search", styles.Dim().Render(m.modeName())}), body, m.footerView(help))
}

// resultsView draws the issues found in the height left under the form.
func (m SearchModel) resultsView(width, height int) string {
	var body string
	switch {
	case m.searching:
		body = m.spinner.View() + " Searching…"
	case m.searchErr != nil:
		body = styles.Error().Render(ansi.Truncate("Error: "+m.searchErr.Error(), width, "…"))
	case !m.searched:
		return ""
	case len(m.results.issues) == 0:
		body = styles.Dim().Render("No issues found.")
	default:
		return lipgloss.NewStyle().Padding(0, 1).Render(m.results.view(width, height, m.resultsFocused))
	}
	return lipgloss.NewStyle().Padding(0, 1).Render(body)
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
		preview = styles.Dim().Render(ansi.Truncate("JQL: "+q.String(), inner, "…"))
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.form.view(inner), "", preview)
}
