package tui

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hawkbawk/jt/internal/jira"
)

type projectsLoadedMsg struct {
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

// picker is the project selection popup. It caches the project list for the
// session.
type picker struct {
	backend Backend

	open        bool
	cancellable bool
	list        list.Model
	projects    []jira.Project
	loaded      bool
	err         error

	// width and height are the size of the whole popup, frame included.
	width, height int
}

func newPicker(b Backend) picker {
	return picker{backend: b}
}

func newProjectList(cancellable bool) list.Model {
	d := list.NewDefaultDelegate()
	d.Styles.SelectedTitle = d.Styles.SelectedTitle.Foreground(highlight).BorderForeground(highlight)
	d.Styles.SelectedDesc = d.Styles.SelectedDesc.Foreground(highlight).BorderForeground(highlight)

	l := list.New(nil, d, 0, 0)
	l.Title = "Select a project"
	l.Styles.Title = titleStyle
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

// show opens a fresh picker. It returns the command that fills the list, from
// the cache when the projects were already fetched.
func (p *picker) show(cancellable bool) tea.Cmd {
	p.open = true
	p.cancellable = cancellable
	p.list = newProjectList(cancellable)
	p.loaded, p.err = false, nil
	p.applySize()
	if p.projects != nil {
		return p.setProjects(p.projects)
	}
	return p.fetch()
}

func (p picker) fetch() tea.Cmd {
	backend := p.backend
	return func() tea.Msg {
		ps, err := backend.Projects()
		return projectsLoadedMsg{projects: ps, err: err}
	}
}

func (p *picker) setProjects(projects []jira.Project) tea.Cmd {
	defaultKey := p.backend.DefaultProject()
	items := make([]list.Item, len(projects))
	for i, pr := range projects {
		items[i] = projectItem{Project: pr, isDefault: pr.Key == defaultKey}
	}
	p.loaded = true
	return p.list.SetItems(items)
}

// loading reports whether the picker is open and waiting on the project list.
func (p picker) loading() bool {
	return p.open && !p.loaded && p.err == nil
}

func (p picker) selected() (jira.Project, bool) {
	it, ok := p.list.SelectedItem().(projectItem)
	return it.Project, ok
}

func (p picker) filtering() bool {
	return p.list.FilterState() == list.Filtering
}

// setSize fits the picker to a window with the given body height.
func (p *picker) setSize(windowWidth, bodyHeight int) {
	w, h := popupSize(windowWidth, bodyHeight)
	p.width, p.height = min(w, 80), h
	p.applySize()
}

func (p *picker) applySize() {
	if !p.open {
		return
	}
	p.list.SetSize(p.width-popupStyle.GetHorizontalFrameSize(), p.height-popupStyle.GetVerticalFrameSize())
}

func (p *picker) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case projectsLoadedMsg:
		if msg.err != nil {
			p.err = msg.err
			return nil
		}
		p.projects = msg.projects
		return p.setProjects(msg.projects)
	case tea.KeyPressMsg:
		return p.handleKey(msg)
	}
	if !p.open {
		return nil
	}
	return p.forward(msg)
}

func (p *picker) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	if !p.loaded {
		if key.Matches(msg, pickerCloseKey) && p.cancellable {
			p.open = false
		}
		return nil
	}
	if p.filtering() {
		return p.forward(msg)
	}
	switch {
	case key.Matches(msg, pickerCloseKey):
		if p.list.IsFiltered() {
			break
		}
		if p.cancellable {
			p.open = false
		}
		return nil
	case key.Matches(msg, pickerOpenKey):
		sel, ok := p.selected()
		if !ok {
			return nil
		}
		p.open = false
		return func() tea.Msg { return projectSelectedMsg{key: sel.Key} }
	}
	return p.forward(msg)
}

func (p *picker) forward(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return cmd
}

func (p picker) view(spin string) string {
	innerW := p.width - popupStyle.GetHorizontalFrameSize()
	innerH := p.height - popupStyle.GetVerticalFrameSize()

	var body string
	switch {
	case p.err != nil:
		body = errorStyle.Render("Error loading projects: " + p.err.Error())
	case !p.loaded:
		body = spin + " Loading projects…"
	default:
		body = p.list.View()
	}
	return popupStyle.Width(innerW + popupStyle.GetHorizontalFrameSize()).
		Height(innerH).
		Render(lipgloss.NewStyle().MaxWidth(innerW).Render(body))
}
