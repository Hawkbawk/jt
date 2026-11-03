package tui

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	tint "github.com/lrstanley/bubbletint/v2"

	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/popup"
)

// layout is the state and behavior the jt screens share: the window size,
// spinner, status line, help footer, quit confirmation and the open popup.
//
// Popups are moving from legacy, which layout sizes and draws itself, to the
// popup host, which owns its content.
type layout struct {
	settings      Settings
	width, height int
	spinner       spinner.Model
	status        string
	help          help.Model

	legacy legacyPopup
	popup  popup.Model
	quit   quitGuard
}

func newLayout(settings Settings) layout {
	return layout{
		settings: settings,
		spinner:  newSpinner(),
		help:     styles.Help(),
		popup:    popup.New(popup.WithStyle(styles.Popup)),
	}
}

// runWithSpinner runs cmd alongside the spinner.
func (c layout) runWithSpinner(cmd tea.Cmd) tea.Cmd {
	return tea.Batch(cmd, c.spinner.Tick)
}

// bodyHeight is the height left between the header and the footer.
func (c layout) bodyHeight() int {
	return max(0, c.height-2)
}

// resize applies a new window size and refits the popup.
func (c *layout) resize(msg tea.WindowSizeMsg) tea.Cmd {
	c.width, c.height = msg.Width, msg.Height
	if c.legacy != nil {
		c.legacy.setSize(popupSize(c.width, c.bodyHeight()))
	}
	return c.updatePopup(tea.WindowSizeMsg{Width: c.width, Height: c.bodyHeight()})
}

// restyle rebuilds what the layout copied from the styles when it was built.
func (c *layout) restyle() {
	c.spinner.Style = styles.Title()
	c.help = styles.Help()
}

// selectTheme applies the tint picked in the theme popup and saves it.
func (c *layout) selectTheme(msg themeSelectedMsg) tea.Cmd {
	tint.SetTint(msg.tint)
	c.restyle()
	return saveTheme(c.settings, msg.id)
}

// onThemeSaved handles the outcome of saving the theme.
func (c *layout) onThemeSaved(msg themeSavedMsg) {
	if msg.err != nil {
		c.status = styles.Error().Render("saving theme: " + msg.err.Error())
	}
}

// updatePopup passes msg to the popup host.
func (c *layout) updatePopup(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	c.popup, cmd = c.popup.Update(msg)
	return cmd
}

// popupOpen reports whether either kind of popup is showing.
func (c layout) popupOpen() bool {
	return c.legacy != nil || c.popup.IsOpen()
}

// popupLoading reports whether the open popup is waiting on the network.
func (c layout) popupLoading() bool {
	p, ok := c.legacy.(interface{ loading() bool })
	return ok && p.loading()
}

// tick advances the spinner while busy. The tick chain ends once nothing is
// busy.
func (c *layout) tick(msg spinner.TickMsg, busy bool) tea.Cmd {
	return tickSpinner(&c.spinner, msg, busy)
}

// forwardToPopup passes msg to the open popup. otherBusy is whether anything
// besides the popup is already running the spinner.
func (c *layout) forwardToPopup(msg tea.Msg, otherBusy bool) tea.Cmd {
	hostCmd := c.updatePopup(msg)
	if c.legacy == nil {
		return hostCmd
	}
	wasBusy := otherBusy || c.popupLoading()
	var cmd tea.Cmd
	c.legacy, cmd = c.legacy.update(msg)
	// Popups start their own loads (sending or reloading a comment), so the
	// spinner has to be started here once they become busy.
	if !wasBusy && (otherBusy || c.popupLoading()) {
		cmd = c.runWithSpinner(cmd)
	}
	return tea.Batch(hostCmd, cmd)
}

// withView wraps the rendered screen in a full-window view themed to match.
func (c layout) withView(content string) tea.View {
	v := tea.NewView(content)
	v.AltScreen = true
	v.BackgroundColor = styles.Background()
	v.ForegroundColor = styles.Foreground()
	return v
}

// headerView renders the title bar with the screen's parts after the name.
func (c layout) headerView(parts []string) string {
	parts = append([]string{styles.Title().Render("jt")}, parts...)
	return styles.Header().Width(c.width).Render(strings.Join(parts, styles.Dim().Render(" · ")))
}

// footerView renders the quit prompt, or else the status and key help.
func (c layout) footerView(km help.KeyMap) string {
	if c.quit.confirming {
		return c.quit.view()
	}
	prefix := ""
	if c.status != "" {
		prefix = c.status + styles.Dim().Render(" · ")
	}
	h := c.help
	h.SetWidth(max(0, c.width-2-lipgloss.Width(prefix)))
	return styles.Dim().Padding(0, 1).Width(c.width).Render(prefix + h.View(km))
}
