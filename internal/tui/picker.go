package tui

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/popup"
)

type projectsLoadedMsg struct {
	id       int64
	projects []jira.Project
	err      error
}

type projectItem struct {
	jira.Project
	isDefault bool
}

func (i projectItem) Title() string {
	if i.isDefault {
		return i.Key + " ★"
	}
	return i.Key
}

func (i projectItem) Description() string { return i.Name }
func (i projectItem) FilterValue() string { return i.Key + " " + i.Name }

var (
	pickerOpenKey  = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open"))
	pickerCloseKey = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close"))
)

// projectSelectedMsg reports the project the user picked.
type projectSelectedMsg struct {
	key string
}

// projectPopup lists the projects and reports the one picked. The board keeps
// the projects it loaded, so later popups start with them.
type projectPopup struct {
	id          int64
	backend     Backend
	spin        spinner.Model
	cancellable bool
	list        list.Model
	projects    []jira.Project
	loaded      bool
	err         error
	width       int
}

// newProjectPopup builds the popup. It loads the projects unless it's given
// them. A popup that isn't cancellable can only be closed by picking a project.
func newProjectPopup(b Backend, projects []jira.Project, cancellable bool) *projectPopup {
	return &projectPopup{
		id:          newPopupID(),
		backend:     b,
		spin:        newSpinner(),
		cancellable: cancellable,
		list:        newProjectList(cancellable),
		projects:    projects,
	}
}

// themedDelegate returns a list delegate that marks the selected item in the
// highlight color.
func themedDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.Styles.SelectedTitle = d.Styles.SelectedTitle.Foreground(styles.Highlight()).BorderForeground(styles.Highlight())
	d.Styles.SelectedDesc = d.Styles.SelectedDesc.Foreground(styles.Highlight()).BorderForeground(styles.Highlight())
	return d
}

func newProjectList(cancellable bool) list.Model {
	l := list.New(nil, themedDelegate(), 0, 0)
	l.Title = "Select a project"
	l.Styles.Title = styles.Title()
	// The list re-enables its own bindings, so quitting is removed by giving
	// those bindings no keys; jt handles quitting itself.
	l.KeyMap.Quit = key.NewBinding()
	l.KeyMap.ForceQuit = key.NewBinding()
	l.AdditionalShortHelpKeys = func() []key.Binding {
		keys := []key.Binding{pickerOpenKey}
		if cancellable {
			keys = append(keys, pickerCloseKey)
		}
		return keys
	}
	return l
}

func loadProjects(backend Backend, id int64) tea.Cmd {
	return func() tea.Msg {
		ps, err := backend.Projects()
		return projectsLoadedMsg{id: id, projects: ps, err: err}
	}
}

// Init implements tea.Model.
func (p *projectPopup) Init() tea.Cmd {
	if p.projects != nil {
		return p.setProjects(p.projects)
	}
	return tea.Batch(loadProjects(p.backend, p.id), p.spin.Tick)
}

func (p *projectPopup) setProjects(projects []jira.Project) tea.Cmd {
	defaultKey := p.backend.DefaultProject()
	items := make([]list.Item, len(projects))
	for i, pr := range projects {
		items[i] = projectItem{Project: pr, isDefault: pr.Key == defaultKey}
	}
	p.loaded = true
	return p.list.SetItems(items)
}

func (p *projectPopup) loading() bool {
	return !p.loaded && p.err == nil
}

func (p *projectPopup) filtering() bool {
	return p.list.FilterState() == list.Filtering
}

// Update implements tea.Model.
func (p *projectPopup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width = msg.Width
		p.list.SetSize(msg.Width, msg.Height)
		return p, nil
	case spinner.TickMsg:
		return p, tickSpinner(&p.spin, msg, p.loading())
	case ThemeChangedMsg:
		p.list.Styles.Title = styles.Title()
		p.list.SetDelegate(themedDelegate())
		return p, nil
	case projectsLoadedMsg:
		if msg.id != p.id {
			return p, nil
		}
		if msg.err != nil {
			p.err = msg.err
			return p, nil
		}
		return p, p.setProjects(msg.projects)
	case tea.KeyPressMsg:
		return p, p.handleKey(msg)
	}
	return p.forward(msg)
}

func (p *projectPopup) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	if !p.loaded {
		if key.Matches(msg, pickerCloseKey) && p.cancellable {
			return popup.Close()
		}
		return nil
	}
	if p.filtering() {
		return p.forwardCmd(msg)
	}
	switch {
	case key.Matches(msg, pickerCloseKey):
		if p.list.IsFiltered() {
			break
		}
		if p.cancellable {
			return popup.Close()
		}
		return nil
	case key.Matches(msg, pickerOpenKey):
		sel, ok := p.list.SelectedItem().(projectItem)
		if !ok {
			return nil
		}
		return popup.CloseWith(projectSelectedMsg{key: sel.Key})
	}
	return p.forwardCmd(msg)
}

func (p *projectPopup) forward(msg tea.Msg) (tea.Model, tea.Cmd) {
	return p, p.forwardCmd(msg)
}

func (p *projectPopup) forwardCmd(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return cmd
}

// View implements tea.Model.
func (p *projectPopup) View() tea.View {
	var body string
	switch {
	case p.err != nil:
		body = styles.Error().Render("Error loading projects: " + p.err.Error())
	case !p.loaded:
		body = p.spin.View() + " Loading projects…"
	default:
		body = p.list.View()
	}
	return tea.NewView(lipgloss.NewStyle().MaxWidth(p.width).Render(body))
}
