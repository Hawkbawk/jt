// Package projectpicker is a tea.Model that lists the Jira projects and
// reports the one the user picks. It's meant to be hosted by a popup.
package projectpicker

import (
	"sync/atomic"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/popup"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
)

// Source is where the picker gets projects and which one is the default.
type Source interface {
	Projects() ([]jira.Project, error)
	DefaultProject() string
}

// LoadedMsg carries the result of Load. A picker ignores the ones that carry
// another picker's ID.
type LoadedMsg struct {
	ID       int64
	Projects []jira.Project
	Err      error
}

// SelectedMsg reports the project the user picked.
type SelectedMsg struct {
	Key string
}

var (
	openKey  = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open"))
	closeKey = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close"))
)

var lastID atomic.Int64

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

// Load fetches the projects and returns them in a LoadedMsg tagged with id.
func Load(src Source, id int64) tea.Cmd {
	return func() tea.Msg {
		ps, err := src.Projects()
		return LoadedMsg{ID: id, Projects: ps, Err: err}
	}
}

// Picker lists the projects and closes its popup with a SelectedMsg when one
// is picked. The caller can keep the projects from the LoadedMsg and give them
// to later pickers.
type Picker struct {
	id          int64
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
	return &Picker{
		id:          lastID.Add(1),
		src:         src,
		spin:        spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(styles.Title())),
		cancellable: cancellable,
		list:        newList(cancellable),
		projects:    projects,
	}
}

func newList(cancellable bool) list.Model {
	l := list.New(nil, styles.ListDelegate(), 0, 0)
	l.Title = "Select a project"
	l.Styles.Title = styles.Title()
	// The list re-enables its own bindings, so quitting is removed by giving
	// those bindings no keys; jt handles quitting itself.
	l.KeyMap.Quit = key.NewBinding()
	l.KeyMap.ForceQuit = key.NewBinding()
	l.AdditionalShortHelpKeys = func() []key.Binding {
		keys := []key.Binding{openKey}
		if cancellable {
			keys = append(keys, closeKey)
		}
		return keys
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
	return tea.Batch(Load(p.src, p.id), p.spin.Tick)
}

func (p *Picker) setProjects(projects []jira.Project) tea.Cmd {
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
	case LoadedMsg:
		if msg.ID != p.id {
			return p, nil
		}
		if msg.Err != nil {
			p.err = msg.Err
			return p, nil
		}
		return p, p.setProjects(msg.Projects)
	case tea.KeyPressMsg:
		return p, p.handleKey(msg)
	}
	return p, p.forward(msg)
}

func (p *Picker) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	if !p.loaded {
		if key.Matches(msg, closeKey) && p.cancellable {
			return popup.Close()
		}
		return nil
	}
	if p.Filtering() {
		return p.forward(msg)
	}
	switch {
	case key.Matches(msg, closeKey):
		if p.list.IsFiltered() {
			break
		}
		if p.cancellable {
			return popup.Close()
		}
		return nil
	case key.Matches(msg, openKey):
		sel, ok := p.list.SelectedItem().(item)
		if !ok {
			return nil
		}
		return popup.CloseWith(SelectedMsg{Key: sel.Key})
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
