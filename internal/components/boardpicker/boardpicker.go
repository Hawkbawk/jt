// Package boardpicker is a tea.Model that lists a project's boards and reports
// the one the user picks. It's meant to be hosted by a modal.
package boardpicker

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

// Source is where the picker gets a project's boards.
type Source interface {
	Boards(project string) ([]jira.BoardSummary, error)
}

// loadedMsg carries the result of load.
type loadedMsg struct {
	Boards []jira.BoardSummary
	Err    error
}

// SelectedMsg reports the board the user picked.
type SelectedMsg struct {
	Project string
	Board   jira.BoardSummary
}

// ForParent marks SelectedMsg for the modal host.
func (SelectedMsg) ForParent() {}

// ClosesModal marks SelectedMsg for the modal host.
func (SelectedMsg) ClosesModal() {}

// FavoriteMsg asks the host to make a board the default. The picker stays open.
type FavoriteMsg struct {
	ID int
}

// ForParent marks FavoriteMsg for the modal host.
func (FavoriteMsg) ForParent() {}

// keyMap is the picker's key bindings.
type keyMap struct {
	pane.KeyMap
	open, favorite key.Binding
}

func newKeyMap() keyMap {
	k := keys.Get()
	open := keys.Describe(k.Accept, "open")
	return keyMap{KeyMap: pane.NewKeyMap(k.Dismiss).WithLead(open, k.Favorite), open: open, favorite: k.Favorite}
}

type item struct {
	jira.BoardSummary
	current bool
}

func (i item) Title() string {
	if i.current {
		return i.Name + " " + icons.Or(icons.Check, "✓")
	}
	return i.Name
}

func (i item) Description() string { return i.Type }
func (i item) FilterValue() string { return i.Name }

// load fetches the project's boards and returns them in a loadedMsg.
func load(src Source, project string) tea.Cmd {
	return func() tea.Msg {
		bs, err := src.Boards(project)
		return loadedMsg{Boards: bs, Err: err}
	}
}

// Picker lists a project's boards and closes its modal with a SelectedMsg when
// one is picked.
type Picker struct {
	keys    keyMap
	src     Source
	project string
	current int
	spin    spinner.Model
	list    list.Model
	loaded  bool
	err     error
	width   int
}

// New builds a picker for project's boards, marking the board with the
// current ID.
func New(src Source, project string, current int) *Picker {
	km := newKeyMap()
	l := list.New(nil, styles.ListDelegate(), 0, 0)
	l.Title = "Select a board"
	l.Styles.Title = styles.Title()
	l.KeyMap.Quit = key.NewBinding()
	l.KeyMap.ForceQuit = key.NewBinding()
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return km.ShortHelp()
	}
	return &Picker{
		src:     src,
		project: project,
		current: current,
		spin:    spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(styles.Title())),
		list:    l,
		keys:    km,
	}
}

// Loaded reports whether the boards are listed.
func (p *Picker) Loaded() bool { return p.loaded }

// Len is how many boards are listed.
func (p *Picker) Len() int { return len(p.list.Items()) }

// Filtering reports whether the user is typing a filter.
func (p *Picker) Filtering() bool { return p.list.FilterState() == list.Filtering }

// Init implements tea.Model.
func (p *Picker) Init() tea.Cmd {
	return tea.Batch(load(p.src, p.project), p.spin.Tick)
}

func (p *Picker) setBoards(boards []jira.BoardSummary) tea.Cmd {
	items := make([]list.Item, len(boards))
	selected := 0
	for i, b := range boards {
		items[i] = item{BoardSummary: b, current: b.ID == p.current}
		if b.ID == p.current {
			selected = i
		}
	}
	p.loaded = true
	cmd := p.list.SetItems(items)
	p.list.Select(selected)
	return cmd
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
	case loadedMsg:
		if msg.Err != nil {
			p.err = msg.Err
			return p, nil
		}
		return p, p.setBoards(msg.Boards)
	case tea.KeyPressMsg:
		return p, p.handleKey(msg)
	}
	return p, p.forward(msg)
}

func (p *Picker) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	if !p.loaded {
		if key.Matches(msg, p.keys.Close()) {
			return tuimsg.Close()
		}
		return nil
	}
	if p.Filtering() {
		return p.forward(msg)
	}
	switch {
	case key.Matches(msg, p.keys.Close()):
		if !p.list.IsFiltered() {
			return tuimsg.Close()
		}
	case key.Matches(msg, p.keys.favorite):
		sel, ok := p.list.SelectedItem().(item)
		if !ok {
			return nil
		}
		return tuimsg.Send(FavoriteMsg{ID: sel.ID})
	case key.Matches(msg, p.keys.open):
		sel, ok := p.list.SelectedItem().(item)
		if !ok {
			return nil
		}
		return tuimsg.Send(SelectedMsg{Project: p.project, Board: sel.BoardSummary})
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
		body = styles.Error().Render("Error loading boards: " + p.err.Error())
	case !p.loaded:
		body = p.spin.View() + " Loading boards…"
	default:
		body = p.list.View()
	}
	return tea.NewView(lipgloss.NewStyle().MaxWidth(p.width).Render(body))
}
