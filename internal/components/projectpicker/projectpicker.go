// Package projectpicker is a tea.Model that lists the Jira projects and
// reports the one the user picks. It ends with a SelectedMsg, or tuimsg.Cancelled.
package projectpicker

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/keys"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/layout"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
	"github.com/hawkbawk/jt/internal/tui/widget"
)

// Source is where the picker gets projects and which one is the default.
type Source interface {
	Projects() ([]jira.Project, error)
	DefaultProject() string
}

// LoadedMsg carries the result of Load. A picker that loaded the projects
// itself sends one to its host once they're listed, so the host can keep them
// for later pickers.
type LoadedMsg struct {
	Projects []jira.Project
	Err      error
}

// loadedMsg is the response to a picker's own load.
type loadedMsg LoadedMsg

// FavoriteMsg asks the host to make a project the default. The picker stays
// open, and the host calls Refresh once the default has changed.
type FavoriteMsg struct {
	Key string
}

// SelectedMsg reports the project the user picked.
type SelectedMsg struct {
	Key string
}

// Final implements tuimsg.Final.
func (SelectedMsg) Final() {}

// keyMap is the picker's key bindings. The close binding only shows when the
// picker can be cancelled.
type keyMap struct {
	layout.KeyMap
	open, favorite key.Binding
}

func newKeyMap(cancellable bool) keyMap {
	k := keys.Get()
	open := keys.Describe(k.Accept, "open")
	return keyMap{
		KeyMap: layout.NewKeyMap(k.Dismiss).WithLead(open, k.Favorite).WithDismissShown(cancellable),
		open:   open, favorite: k.Favorite,
	}
}

type item struct {
	jira.Project
	isDefault bool
}

func (i item) Title() string {
	if i.isDefault {
		return i.Key + " ★"
	}
	return i.Key
}

func (i item) Description() string { return i.Name }
func (i item) FilterValue() string { return i.Key + " " + i.Name }

// Load fetches the projects and returns them in a LoadedMsg.
func Load(src Source) tea.Cmd {
	return func() tea.Msg {
		ps, err := src.Projects()
		return LoadedMsg{Projects: ps, Err: err}
	}
}

// Picker lists the projects and ends with a SelectedMsg when one
// is picked. The caller can keep the projects from the LoadedMsg and give them
// to later pickers.
type Picker struct {
	tuimsg.Requests
	keys        keyMap
	src         Source
	spin        widget.Spinner
	cancellable bool
	list        list.Model
	projects    []jira.Project
	loaded      bool
	err         error
	width       int
}

// New builds a picker. It loads the projects unless it's given them. A picker
// that isn't cancellable can only be closed by picking a project.
func New(src Source, projects []jira.Project, cancellable bool) *Picker {
	km := newKeyMap(cancellable)
	return &Picker{
		keys:        km,
		src:         src,
		spin:        widget.NewSpinner(),
		cancellable: cancellable,
		list:        newList(km),
		projects:    projects,
	}
}

func newList(km keyMap) list.Model {
	l := list.New(nil, styles.ListDelegate(), 0, 0)
	l.Title = "Select a project"
	l.Styles.Title = styles.Title()
	// The list re-enables its own bindings, so quitting is removed by giving
	// those bindings no keys; jt handles quitting itself.
	l.KeyMap.Quit = key.NewBinding()
	l.KeyMap.ForceQuit = key.NewBinding()
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return km.ShortHelp()
	}
	return l
}

// Cancellable reports whether esc closes the picker without a choice.
func (p *Picker) Cancellable() bool { return p.cancellable }

// Loaded reports whether the projects are listed.
func (p *Picker) Loaded() bool { return p.loaded }

// Filtering reports whether the user is typing a filter.
func (p *Picker) Filtering() bool { return p.list.FilterState() == list.Filtering }

// Len is how many projects are listed.
func (p *Picker) Len() int { return len(p.list.Items()) }

// Init implements tea.Model.
func (p *Picker) Init() tea.Cmd {
	if p.projects != nil {
		return p.setProjects(p.projects)
	}
	load := Load(p.src)
	return tea.Batch(p.Request(func() tea.Msg { return loadedMsg(load().(LoadedMsg)) }), p.spin.Sync(p.loading()))
}

// Refresh marks the source's current default project.
func (p *Picker) Refresh() tea.Cmd {
	return p.setProjects(p.projects)
}

func (p *Picker) setProjects(projects []jira.Project) tea.Cmd {
	p.projects = projects
	defaultKey := p.src.DefaultProject()
	items := make([]list.Item, len(projects))
	for i, pr := range projects {
		items[i] = item{Project: pr, isDefault: pr.Key == defaultKey}
	}
	p.loaded = true
	return p.list.SetItems(items)
}

func (p *Picker) loading() bool {
	return !p.loaded && p.err == nil
}

// Update implements tea.Model.
func (p *Picker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width = msg.Width
		p.list.SetSize(msg.Width, msg.Height)
		return p, nil
	case spinner.TickMsg:
		return p, p.spin.Update(msg, p.loading())
	case tuimsg.ThemeChanged:
		p.list.Styles.Title = styles.Title()
		p.list.SetDelegate(styles.ListDelegate())
		return p, nil
	case loadedMsg:
		if msg.Err != nil {
			p.err = msg.Err
			return p, nil
		}
		ps := msg.Projects
		return p, tea.Batch(p.setProjects(ps), func() tea.Msg { return LoadedMsg{Projects: ps} })
	case tea.KeyPressMsg:
		return p, p.handleKey(msg)
	}
	return p, p.forward(msg)
}

func (p *Picker) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	if !p.loaded {
		if key.Matches(msg, p.keys.Dismiss()) && p.cancellable {
			return tuimsg.Cancel()
		}
		return nil
	}
	if p.Filtering() {
		return p.forward(msg)
	}
	switch {
	case key.Matches(msg, p.keys.Dismiss()):
		if p.list.IsFiltered() {
			break
		}
		if p.cancellable {
			return tuimsg.Cancel()
		}
		return nil
	case key.Matches(msg, p.keys.favorite):
		sel, ok := p.list.SelectedItem().(item)
		if !ok {
			return nil
		}
		return func() tea.Msg { return FavoriteMsg{Key: sel.Key} }
	case key.Matches(msg, p.keys.open):
		sel, ok := p.list.SelectedItem().(item)
		if !ok {
			return nil
		}
		return tuimsg.End(SelectedMsg{Key: sel.Key})
	}
	return p.forward(msg)
}

func (p *Picker) forward(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return cmd
}

// View implements tea.Model.
func (p *Picker) View() tea.View {
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
