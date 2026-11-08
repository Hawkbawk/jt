// Package commandpicker is a tea.Model that lets the user choose a configured
// command and reports it to its host.
package commandpicker

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/hawkbawk/jt/internal/config"
	"github.com/hawkbawk/jt/internal/keys"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/popup"
	"github.com/hawkbawk/jt/internal/tui/popup/pane"
)

// SelectedMsg reports the command the user picked.
type SelectedMsg struct {
	Command config.Command
}

// Picker lists the configured commands.
type Picker struct {
	commands []config.Command
	choice   int
	field    *huh.Select[int]
	form     *huh.Form
	close    key.Binding
	width    int
	height   int
}

// New builds a picker with the configured commands.
func New(commands []config.Command) *Picker {
	p := &Picker{commands: commands, close: keys.Get().Close}
	if len(commands) > 0 {
		opts := make([]huh.Option[int], 0, len(commands))
		for i, c := range commands {
			opts = append(opts, huh.NewOption(c.DisplayName(), i))
		}
		p.field = huh.NewSelect[int]().Title("Run a command").Options(opts...).Value(&p.choice)
		p.form = pane.NewForm(p.field)
		km := huh.NewDefaultKeyMap()
		km.Quit = key.NewBinding(key.WithDisabled())
		p.form.WithKeyMap(km)
	}
	return p
}

// Init implements tea.Model.
func (p *Picker) Init() tea.Cmd {
	if p.form == nil {
		return nil
	}
	return p.form.Init()
}

// Update implements tea.Model.
func (p *Picker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		p.width, p.height = size.Width, size.Height
		return p, nil
	}
	if p.form == nil {
		if km, ok := msg.(tea.KeyPressMsg); ok && key.Matches(km, p.close) {
			return p, popup.Close()
		}
		return p, nil
	}
	if km, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(km, p.close) && !p.consumesEsc():
			return p, popup.Close()
		case key.Matches(km, keys.Get().Dismiss) && p.consumesEsc():
			_, cmd := p.form.Update(msg)
			return p, cmd
		}
	}
	_, cmd := p.form.Update(msg)
	if p.form.State == huh.StateCompleted {
		return p, popup.CloseWith(SelectedMsg{Command: p.commands[p.choice]})
	}
	return p, cmd
}

func (p *Picker) consumesEsc() bool {
	return p.field != nil && key.Matches(tea.KeyPressMsg{Code: tea.KeyEsc}, p.field.KeyBinds()...)
}

// View implements tea.Model.
func (p *Picker) View() tea.View {
	var body string
	if p.form == nil {
		body = styles.Dim().Render("No commands configured.")
	} else {
		p.form.WithWidth(p.width).WithHeight(max(1, p.height-2))
		body = p.form.View()
	}
	return tea.NewView(popup.Stack(p.height, []string{"Commands", ""}, strings.Split(body, "\n"),
		styles.Dim().Render("enter run · esc close")))
}
