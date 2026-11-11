// Package projectpicker is a tea.Model that lists the Jira projects and
// reports the one the user picks. It's meant to be hosted by a popup.
package projectpicker

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hawkbawk/jt/internal/components/pane"
	"github.com/hawkbawk/jt/internal/icons"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/keys"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
)

// Source is where the picker gets projects and which one is the default.
type Source interface {
	Projects() ([]jira.Project, error)
	DefaultProject() string
}

// LoadedMsg carries the result of Load. A picker sends the projects it loaded
// to the screen in one, so the screen can keep them for later pickers.
type LoadedMsg struct {
	Projects []jira.Project
	Err      error
}

// ForParent marks LoadedMsg for the popup host.
func (LoadedMsg) ForParent() {}

// FavoriteMsg asks the host to make a project the default. The picker stays
// open, and the host sends it DefaultChanged once the default has changed.
type FavoriteMsg struct {
	Key string
}

// ForParent marks FavoriteMsg for the popup host.
func (FavoriteMsg) ForParent() {}

// DefaultChanged tells the picker to mark the source's current default
// project.
type DefaultChanged struct{}

// SelectedMsg reports the project the user picked.
type SelectedMsg struct {
	Key string
}

// ForParent marks SelectedMsg for the popup host.
func (SelectedMsg) ForParent() {}

// ClosesPopup marks SelectedMsg for the popup host.
func (SelectedMsg) ClosesPopup() {}

// keyMap is the picker's key bindings. The close binding only shows when the
// picker can be cancelled.
type keyMap struct {
	pane.KeyMap
	open, favorite key.Binding
}

func newKeyMap(cancellable bool) keyMap {
	k := keys.Get()
	open := keys.Describe(k.Accept, "open")
	return keyMap{
		KeyMap: pane.NewKeyMap(k.Dismiss).WithLead(open, k.Favorite).WithCloseShown(cancellable),
		open:   open, favorite: k.Favorite,
	}
}

type item struct {
	jira.Project
	isDefault bool
}

func (i item) Title() string {
	if i.isDefault {
		return i.Key + " " + icons.Or(icons.Star, "★")
	}
	return i.Key
}

func (i item) Description() string { return i.Name }
func (i item) FilterValue() string { return i.Key + " " + i.Name }

// loadedMsg is the picker's own load, which comes back to the picker.
type loadedMsg LoadedMsg

// Load fetches the projects and returns them in a LoadedMsg.
func Load(src Source) tea.Cmd {
	return func() tea.Msg {
		ps, err := src.Projects()
		return LoadedMsg{Projects: ps, Err: err}
	}
}

// Picker lists the projects and closes its popup with a SelectedMsg when one
// is picked. The caller can keep the projects from the LoadedMsg and give them
// to later pickers.
type Picker struct {
	keys        keyMap
	src         Source
	spin        spinner.Model
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
		spin:        spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(styles.Title())),
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

func (p *Picker) load() tea.Cmd {
	src := p.src
	return func() tea.Msg {
		ps, err := src.Projects()
		return loadedMsg{Projects: ps, Err: err}
	}
}

// Init implements tea.Model.
func (p *Picker) Init() tea.Cmd {
	if p.projects != nil {
		return p.setProjects(p.projects)
	}
	return tea.Batch(p.load(), p.spin.Tick)
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
		if !p.loading() {
			return p, nil
		}
		var cmd tea.Cmd
		p.spin, cmd = p.spin.Update(msg)
		return p, cmd
	case tuimsg.ThemeChanged:
		p.list.Styles.Title = styles.Title()
		p.list.SetDelegate(styles.ListDelegate())
		return p, nil
	case DefaultChanged:
		return p, p.setProjects(p.projects)
	case loadedMsg:
		if msg.Err != nil {
			p.err = msg.Err
			return p, nil
		}
		return p, tea.Batch(p.setProjects(msg.Projects), tuimsg.Send(LoadedMsg(msg)))
	case tea.KeyPressMsg:
		return p, p.handleKey(msg)
	}
	return p, p.forward(msg)
}

func (p *Picker) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	if !p.loaded {
		if key.Matches(msg, p.keys.Close()) && p.cancellable {
			return tuimsg.Close()
		}
		return nil
	}
	if p.Filtering() {
		return p.forward(msg)
	}
	switch {
	case key.Matches(msg, p.keys.Close()):
		if p.list.IsFiltered() {
			break
		}
		if p.cancellable {
			return tuimsg.Close()
		}
		return nil
	case key.Matches(msg, p.keys.favorite):
		sel, ok := p.list.SelectedItem().(item)
		if !ok {
			return nil
		}
		return tuimsg.Send(FavoriteMsg{Key: sel.Key})
	case key.Matches(msg, p.keys.open):
		sel, ok := p.list.SelectedItem().(item)
		if !ok {
			return nil
		}
		return tuimsg.Send(SelectedMsg{Key: sel.Key})
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
