package tui

import (
	"context"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	tint "github.com/lrstanley/bubbletint/v2"

	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/popup"
)

// Settings persists the user's choices.
type Settings interface {
	SaveTheme(id string) error
}

var pickerCloseKey = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close"))

// themeSelectedMsg reports the tint picked in the theme popup, and the ID to
// save for it. An empty ID follows the terminal's theme.
type themeSelectedMsg struct {
	tint *tint.Tint
	id   string
}

// themeResetMsg carries the terminal's own theme, once it has answered.
type themeResetMsg struct{ tint *tint.Tint }

// themeSavedMsg reports whether the picked tint was written to the settings.
type themeSavedMsg struct {
	err error
}

func saveTheme(s Settings, id string) tea.Cmd {
	return func() tea.Msg {
		return themeSavedMsg{err: s.SaveTheme(id)}
	}
}

type tintItem struct {
	*tint.Tint
	current bool
}

func (i tintItem) Title() string {
	if i.current {
		return i.DisplayName + " ✓"
	}
	return i.DisplayName
}

func (i tintItem) Description() string {
	if i.Dark {
		return "dark · " + i.ID
	}
	return "light · " + i.ID
}

func (i tintItem) FilterValue() string { return i.DisplayName + " " + i.ID }

var (
	themeApplyKey = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "apply"))
	themeResetKey = key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reset to terminal"))
)

// themePopup lists every registered tint and applies the chosen one.
type themePopup struct {
	list  list.Model
	width int
}

func newThemePopup() *themePopup {
	current := styles.Current()
	tints := tint.Tints()
	items := make([]list.Item, len(tints))
	selected := 0
	for i, t := range tints {
		items[i] = tintItem{Tint: t, current: t == current}
		if t == current {
			selected = i
		}
	}

	l := list.New(items, styles.ListDelegate(), 0, 0)
	l.Title = "Theme"
	l.Styles.Title = styles.Title()
	l.KeyMap.Quit = key.NewBinding()
	l.KeyMap.ForceQuit = key.NewBinding()
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{themeApplyKey, themeResetKey, pickerCloseKey}
	}
	l.Select(selected)
	return &themePopup{list: l}
}

// Init implements tea.Model.
func (p *themePopup) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (p *themePopup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		p.width = size.Width
		p.list.SetSize(size.Width, size.Height)
		return p, nil
	}
	if reset, ok := msg.(themeResetMsg); ok {
		return p, popup.CloseWith(themeSelectedMsg{tint: reset.tint})
	}
	if km, ok := msg.(tea.KeyPressMsg); ok && p.list.FilterState() != list.Filtering {
		switch {
		case key.Matches(km, pickerCloseKey) && !p.list.IsFiltered():
			return p, popup.Close()
		case key.Matches(km, themeApplyKey):
			it, ok := p.list.SelectedItem().(tintItem)
			if !ok {
				return p, nil
			}
			return p, popup.CloseWith(themeSelectedMsg{tint: it.Tint, id: it.ID})
		case key.Matches(km, themeResetKey):
			// Reading the terminal's theme waits on it, so it happens in a command.
			return p, func() tea.Msg { return themeResetMsg{tint: styles.Default(context.Background())} }
		}
	}
	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return p, cmd
}

// View implements tea.Model.
func (p *themePopup) View() tea.View {
	return tea.NewView(lipgloss.NewStyle().MaxWidth(p.width).Render(p.list.View()))
}
