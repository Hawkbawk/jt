// Package screen holds the chrome that jt's screens share: the window and popup
// host, the quit prompt, issue actions and the theme setting.
package screen

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	tint "github.com/lrstanley/bubbletint/v2"

	"github.com/hawkbawk/jt/internal/components/themepicker"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/popup"
	"github.com/hawkbawk/jt/internal/tui/widget"
)

// Layout is the state and behavior the jt screens share: the window size,
// spinner, status line, help footer, quit confirmation and the popup host.
// Screens embed it.
type Layout struct {
	Settings      Settings
	Width, Height int
	Spinner       widget.Spinner
	Status        string
	Help          help.Model

	Popup popup.Model
	Quit  QuitGuard
}

// NewLayout returns a layout that saves its choices to settings.
func NewLayout(settings Settings) Layout {
	return Layout{
		Settings: settings,
		Spinner:  widget.NewSpinner(),
		Help:     styles.Help(),
		Popup:    popup.New(popup.WithStyle(styles.Popup)),
	}
}

// RunWithSpinner runs cmd with the spinner going until the screen isn't busy.
func (c *Layout) RunWithSpinner(cmd tea.Cmd) tea.Cmd {
	return tea.Batch(cmd, c.Spinner.Sync(true))
}

// BodyHeight is the height left between the header and the footer.
func (c Layout) BodyHeight() int {
	return max(0, c.Height-2)
}

// Resize applies a new window size and refits the popup.
func (c *Layout) Resize(msg tea.WindowSizeMsg) tea.Cmd {
	c.Width, c.Height = msg.Width, msg.Height
	return c.UpdatePopup(tea.WindowSizeMsg{Width: c.Width, Height: c.BodyHeight()})
}

// Restyle rebuilds what the layout copied from the styles when it was built.
func (c *Layout) Restyle() {
	c.Spinner.Style = styles.Title()
	c.Help = styles.Help()
}

// SelectTheme applies the tint picked in the theme popup and saves it.
func (c *Layout) SelectTheme(msg themepicker.SelectedMsg) tea.Cmd {
	tint.SetTint(msg.Tint)
	c.Restyle()
	return SaveTheme(c.Settings, msg.ID)
}

// HandleCommon handles the messages every screen handles the same way: the
// spinner, opening and closing the popup, saving the theme and the result of
// an issue action. busy is whether the screen is waiting on the network. It
// reports whether msg was one of them.
func (c *Layout) HandleCommon(msg tea.Msg, busy bool) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		return tea.Batch(c.Spinner.Update(msg, busy), c.UpdatePopup(msg)), true
	case popup.OpenMsg, popup.CloseMsg:
		return c.UpdatePopup(msg), true
	case ThemeSavedMsg:
		if msg.Err != nil {
			c.Status = styles.Error().Render("saving theme: " + msg.Err.Error())
		}
		return nil, true
	case ActionDoneMsg:
		c.Status = msg.Display()
		return nil, true
	}
	return nil, false
}

// UpdatePopup passes msg to the popup host.
func (c *Layout) UpdatePopup(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	c.Popup, cmd = c.Popup.Update(msg)
	return cmd
}

// WithView wraps the rendered screen in a full-window view themed to match.
func (c Layout) WithView(content string) tea.View {
	v := tea.NewView(content)
	v.AltScreen = true
	v.BackgroundColor = styles.Background()
	v.ForegroundColor = styles.Foreground()
	return v
}

// HeaderView renders the title bar with the screen's parts after the name.
func (c Layout) HeaderView(parts []string) string {
	parts = append([]string{styles.Title().Render("jt")}, parts...)
	return styles.Header().Width(c.Width).Render(strings.Join(parts, styles.Dim().Render(" · ")))
}

// FooterView renders the quit prompt, or else the status and key help.
func (c Layout) FooterView(km help.KeyMap) string {
	if c.Quit.Confirming {
		return c.Quit.View()
	}
	prefix := ""
	if c.Status != "" {
		prefix = c.Status + styles.Dim().Render(" · ")
	}
	h := c.Help
	h.SetWidth(max(0, c.Width-2-lipgloss.Width(prefix)))
	return styles.Dim().Padding(0, 1).Width(c.Width).Render(prefix + h.View(km))
}
