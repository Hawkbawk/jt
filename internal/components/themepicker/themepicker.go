// Package themepicker is a tea.Model that lists every registered tint and
// reports the one the user picks. It's meant to be hosted by a popup.
package themepicker

import (
	"context"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	tint "github.com/lrstanley/bubbletint/v2"

	"github.com/hawkbawk/jt/internal/icons"
	"github.com/hawkbawk/jt/internal/keys"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/popup"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
)

// SelectedMsg reports the tint the user picked, and the ID to save for it. An
// empty ID follows the terminal's theme.
type SelectedMsg struct {
	Tint *tint.Tint
	ID   string
}

// resetMsg carries the terminal's own theme, once it has answered.
type resetMsg struct{ tint *tint.Tint }

// keyMap is the picker's key bindings.
type keyMap struct {
	popup.KeyMap
	apply, reset key.Binding
}

func newKeyMap() keyMap {
	k := keys.Get()
	apply, reset := keys.Describe(k.Accept, "apply"), keys.Describe(k.Reset, "reset to terminal")
	return keyMap{KeyMap: popup.NewKeyMap(k.Dismiss).WithLead(apply, reset), apply: apply, reset: reset}
}

type item struct {
	*tint.Tint
	current bool
}

func (i item) Title() string {
	if i.current {
		return i.DisplayName + " " + icons.Or(icons.Check, "✓")
	}
	return i.DisplayName
}

func (i item) Description() string {
	if i.Dark {
		return "dark · " + i.ID
	}
	return "light · " + i.ID
}

func (i item) FilterValue() string { return i.DisplayName + " " + i.ID }

// Picker lists every registered tint and closes its popup with a SelectedMsg
// when one is chosen.
type Picker struct {
	keys  keyMap
	list  list.Model
	width int
}

// New builds a picker that starts on the current tint.
func New() *Picker {
	current := styles.Current()
	tints := tint.Tints()
	items := make([]list.Item, len(tints))
	selected := 0
	for i, t := range tints {
		items[i] = item{Tint: t, current: t == current}
		if t == current {
			selected = i
		}
	}

	km := newKeyMap()
	l := list.New(items, styles.ListDelegate(), 0, 0)
	l.Title = "Theme"
	l.Styles.Title = styles.Title()
	l.KeyMap.Quit = key.NewBinding()
	l.KeyMap.ForceQuit = key.NewBinding()
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return km.ShortHelp()
	}
	l.Select(selected)
	return &Picker{list: l, keys: km}
}

// Len is how many tints are listed.
func (p *Picker) Len() int { return len(p.list.Items()) }

// Selected is the highlighted tint, or nil when the filter matches none.
func (p *Picker) Selected() *tint.Tint {
	it, ok := p.list.SelectedItem().(item)
	if !ok {
		return nil
	}
	return it.Tint
}

// Filtering reports whether the user is typing a filter.
func (p *Picker) Filtering() bool { return p.list.FilterState() == list.Filtering }

// Filtered reports whether a filter is applied.
func (p *Picker) Filtered() bool { return p.list.IsFiltered() }

// Init implements tea.Model.
func (p *Picker) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (p *Picker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width = msg.Width
		p.list.SetSize(msg.Width, msg.Height)
		return p, nil
	case tuimsg.ThemeChanged:
		p.list.Styles.Title = styles.Title()
		p.list.SetDelegate(styles.ListDelegate())
		return p, nil
	case resetMsg:
		return p, popup.CloseWith(SelectedMsg{Tint: msg.tint})
	case tea.KeyPressMsg:
		if cmd, ok := p.handleKey(msg); ok {
			return p, cmd
		}
	}
	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return p, cmd
}

// handleKey reports whether the picker consumed the key.
func (p *Picker) handleKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	if p.Filtering() {
		return nil, false
	}
	switch {
	case key.Matches(msg, p.keys.Close()) && !p.list.IsFiltered():
		return popup.Close(), true
	case key.Matches(msg, p.keys.apply):
		t := p.Selected()
		if t == nil {
			return nil, true
		}
		return popup.CloseWith(SelectedMsg{Tint: t, ID: t.ID}), true
	case key.Matches(msg, p.keys.reset):
		// Reading the terminal's theme waits on it, so it happens in a command.
		return func() tea.Msg { return resetMsg{tint: styles.Default(context.Background())} }, true
	}
	return nil, false
}

// View implements tea.Model.
func (p *Picker) View() tea.View {
	return tea.NewView(lipgloss.NewStyle().MaxWidth(p.width).Render(p.list.View()))
}
