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
	"github.com/hawkbawk/jt/internal/icons"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/popup"
	"github.com/hawkbawk/jt/internal/tui/popup/pane"
)

// Layout is the state and behavior the jt screens share: the window size,
// spinner, status line, help footer, quit confirmation and the popup host.
// Screens embed it.
type Layout struct {
	Settings      Settings
	Width, Height int
	Spinner       spinner.Model
	Status        string
	Help          help.Model

	Popup popup.Model
	Quit  QuitGuard
}

// NewLayout returns a layout that saves its choices to settings.
func NewLayout(settings Settings) Layout {
	return Layout{
		Settings: settings,
		Spinner:  pane.NewSpinner(),
		Help:     styles.Help(),
		Popup:    popup.New(popup.WithStyle(styles.Popup)),
	}
}

// RunWithSpinner runs cmd alongside the spinner.
func (c Layout) RunWithSpinner(cmd tea.Cmd) tea.Cmd {
	return tea.Batch(cmd, c.Spinner.Tick)
}

// BodyHeight is the height left between the header and the footer that shows
// km, which is taller while the full help is expanded.
func (c Layout) BodyHeight(km help.KeyMap) int {
	if !c.Help.ShowAll {
		return max(0, c.Height-2)
	}
	return max(0, c.Height-1-lipgloss.Height(c.FooterView(km)))
}

// Resize applies a new window size and refits the popup.
func (c *Layout) Resize(msg tea.WindowSizeMsg, km help.KeyMap) tea.Cmd {
	c.Width, c.Height = msg.Width, msg.Height
	return c.UpdatePopup(tea.WindowSizeMsg{Width: c.Width, Height: c.BodyHeight(km)})
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

// OnThemeSaved handles the outcome of saving the theme.
func (c *Layout) OnThemeSaved(msg ThemeSavedMsg) {
	if msg.Err != nil {
		c.Status = styles.Error().Render("saving theme: " + msg.Err.Error())
	}
}

// UpdatePopup passes msg to the popup host.
func (c *Layout) UpdatePopup(msg tea.Msg) tea.Cmd {
	if _, ok := msg.(popup.OpenMsg); ok {
		c.CollapseHelp()
	}
	var cmd tea.Cmd
	c.Popup, cmd = c.Popup.Update(msg)
	return cmd
}

// Tick advances the spinner while busy. The tick chain ends once nothing is
// busy.
func (c *Layout) Tick(msg spinner.TickMsg, busy bool) tea.Cmd {
	return pane.Tick(&c.Spinner, msg, busy)
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
	parts = append([]string{styles.Brand(icons.Label(icons.Jira, "jt"))}, parts...)
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
	return styles.Dim().Padding(0, 1).Width(c.Width).Render(prefix + c.helpText(km, lipgloss.Width(prefix)))
}

// ToggleHelp shows or hides the full key help in the footer.
func (c *Layout) ToggleHelp() { c.Help.ShowAll = !c.Help.ShowAll }

// CollapseHelp returns the footer to the short help.
func (c *Layout) CollapseHelp() { c.Help.ShowAll = false }

func (c Layout) helpText(km help.KeyMap, prefixWidth int) string {
	h := c.Help
	h.SetWidth(max(0, c.Width-2-prefixWidth))
	return h.View(km)
}
