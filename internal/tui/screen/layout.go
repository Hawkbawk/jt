// Package screen holds the chrome that jt's screens share: the window and modal
// host, the quit prompt, issue actions and the theme setting.
package screen

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	tint "github.com/lrstanley/bubbletint/v2"

	"github.com/hawkbawk/jt/internal/components/pane"
	"github.com/hawkbawk/jt/internal/components/themepicker"
	"github.com/hawkbawk/jt/internal/icons"
	"github.com/hawkbawk/jt/internal/keys"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/modal"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
)

// Layout is the state and behavior the jt screens share: the window size,
// spinner, status line, help footer, quit confirmation and the modal host.
// Screens embed it.
type Layout struct {
	Settings      Settings
	Width, Height int
	Spinner       spinner.Model
	Status        string
	statusSeq     int
	Help          help.Model

	Modal modal.Model
	Quit  QuitGuard
}

// NewLayout returns a layout that saves its choices to settings.
func NewLayout(settings Settings) Layout {
	return Layout{
		Settings: settings,
		Spinner:  pane.NewSpinner(),
		Help:     styles.Help(),
		Modal:    modal.New(modal.WithStyle(styles.Modal), modal.WithTitleStyle(styles.Title), modal.WithHelp(styles.Help)),
	}
}

// StatusTime is how long a routine status stays in the footer.
const StatusTime = 5 * time.Second

// StatusExpiredMsg tells the screen that the timer for a status ran out. Screens
// pass it to Layout.ExpireStatus.
type StatusExpiredMsg struct{ seq int }

// Notify shows text in the status line and returns the command that clears it
// after d.
func (c *Layout) Notify(text string, d time.Duration) tea.Cmd {
	c.Status = text
	c.statusSeq++
	if text == "" {
		return nil
	}
	seq := c.statusSeq
	return tea.Tick(d, func(time.Time) tea.Msg { return StatusExpiredMsg{seq: seq} })
}

// CommandStatus shows the status a command message produced next to the
// command it started. The progress shown while a command loads stays until the
// result replaces it, and any other status expires.
func (c *Layout) CommandStatus(msg tea.Msg, status string, cmd tea.Cmd) tea.Cmd {
	if _, preparing := msg.(tuimsg.CommandSelected); preparing || status == "" {
		c.Status = status
		c.statusSeq++
		return cmd
	}
	return tea.Batch(cmd, c.Notify(status, StatusTime))
}

// ExpireStatus clears the status if the timer is the latest one set.
func (c *Layout) ExpireStatus(msg StatusExpiredMsg) {
	if c.statusSeq == msg.seq {
		c.Status = ""
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

// Resize applies a new window size and refits the modal.
func (c *Layout) Resize(msg tea.WindowSizeMsg, km help.KeyMap) tea.Cmd {
	c.Width, c.Height = msg.Width, msg.Height
	return c.UpdateModal(tea.WindowSizeMsg{Width: c.Width, Height: c.BodyHeight(km)})
}

// Restyle rebuilds what the layout copied from the styles when it was built.
func (c *Layout) Restyle() {
	c.Spinner.Style = styles.Title()
	c.Help = styles.Help()
}

// SelectTheme applies the tint picked in the theme modal and saves it.
func (c *Layout) SelectTheme(msg themepicker.SelectedMsg) tea.Cmd {
	tint.SetTint(msg.Tint)
	c.Restyle()
	return SaveTheme(c.Settings, msg.ID)
}

// OnThemeSaved handles the outcome of saving the theme.
func (c *Layout) OnThemeSaved(msg ThemeSavedMsg) tea.Cmd {
	if msg.Err != nil {
		return c.Notify(styles.Error().Render("saving theme: "+msg.Err.Error()), StatusTime)
	}
	return nil
}

// UpdateModal passes msg to the modal host, which gives the content anything
// that isn't the host's own. Screens call it for messages the modal owns
// (check with Modal.Routes) and to broadcast to the content.
func (c *Layout) UpdateModal(msg tea.Msg) tea.Cmd {
	was := c.Modal.IsOpen()
	var cmd tea.Cmd
	c.Modal, cmd = c.Modal.Update(msg)
	c.collapseOnOpen(was)
	return cmd
}

// collapseOnOpen collapses the help when a modal opened, since the modal's
// own help replaces it.
func (c *Layout) collapseOnOpen(was bool) {
	if !was && c.Modal.IsOpen() {
		c.CollapseHelp()
	}
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
		prefix = styles.Notice().Render(c.Status) + styles.Dim().Render(" · ")
	}
	return styles.Dim().Padding(0, 1).Width(c.Width).Render(prefix + c.helpText(km, lipgloss.Width(prefix)))
}

// ToggleHelp shows or hides the full key help in the footer.
func (c *Layout) ToggleHelp() { c.Help.ShowAll = !c.Help.ShowAll }

// DismissHelp collapses the full help when msg is the dismiss key and reports
// whether it did, so the key has no other effect.
func (c *Layout) DismissHelp(msg tea.KeyPressMsg) bool {
	if !c.Help.ShowAll || !key.Matches(msg, keys.Get().Dismiss) {
		return false
	}
	c.CollapseHelp()
	return true
}

// CollapseHelp returns the footer to the short help.
func (c *Layout) CollapseHelp() { c.Help.ShowAll = false }

func (c Layout) helpText(km help.KeyMap, prefixWidth int) string {
	h := c.Help
	h.SetWidth(max(0, c.Width-2-prefixWidth))
	return h.View(km)
}
