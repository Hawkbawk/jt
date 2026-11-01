package tui

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	tint "github.com/lrstanley/bubbletint/v2"

	"github.com/hawkbawk/jt/internal/styles"
)

// chrome is the state and behavior the jt screens share: the window size,
// spinner, status line, help footer, quit confirmation and the open popup.
type chrome struct {
	settings      Settings
	width, height int
	spinner       spinner.Model
	status        string
	help          help.Model

	popup popup
	quit  quitGuard
}

func newChrome(settings Settings) chrome {
	return chrome{
		settings: settings,
		spinner:  spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(styles.Title())),
		help:     styles.Help(),
	}
}

// startBusy runs cmd alongside the spinner.
func (c chrome) startBusy(cmd tea.Cmd) tea.Cmd {
	return tea.Batch(cmd, c.spinner.Tick)
}

// bodyHeight is the height left between the header and the footer.
func (c chrome) bodyHeight() int {
	return max(0, c.height-2)
}

// resize applies a new window size and refits the popup.
func (c *chrome) resize(msg tea.WindowSizeMsg) {
	c.width, c.height = msg.Width, msg.Height
	if c.popup != nil {
		c.popup.setSize(popupSize(c.width, c.bodyHeight()))
	}
}

// restyle rebuilds what the chrome copied from the styles when it was built.
func (c *chrome) restyle() {
	c.spinner.Style = styles.Title()
	c.help = styles.Help()
}

// selectTheme applies the tint picked in the theme popup and saves it.
func (c *chrome) selectTheme(msg themeSelectedMsg) tea.Cmd {
	tint.SetTint(msg.tint)
	c.restyle()
	return saveTheme(c.settings, msg.id)
}

// themeSaved shows an error from saving the theme.
func (c *chrome) themeSaved(msg themeSavedMsg) {
	if msg.err != nil {
		c.status = styles.Error().Render("saving theme: " + msg.err.Error())
	}
}

// popupLoading reports whether the open popup is waiting on the network.
func (c chrome) popupLoading() bool {
	p, ok := c.popup.(interface{ loading() bool })
	return ok && p.loading()
}

// tick advances the spinner while busy. The tick chain ends once nothing is
// busy.
func (c *chrome) tick(msg spinner.TickMsg, busy bool) tea.Cmd {
	if !busy {
		return nil
	}
	var cmd tea.Cmd
	c.spinner, cmd = c.spinner.Update(msg)
	return cmd
}

// forwardToPopup passes msg to the open popup. otherBusy is whether anything
// besides the popup is already running the spinner.
func (c *chrome) forwardToPopup(msg tea.Msg, otherBusy bool) tea.Cmd {
	if c.popup == nil {
		return nil
	}
	wasBusy := otherBusy || c.popupLoading()
	var cmd tea.Cmd
	c.popup, cmd = c.popup.update(msg)
	// Popups start their own loads (sending or reloading a comment), so the
	// spinner has to be started here once they become busy.
	if !wasBusy && (otherBusy || c.popupLoading()) {
		cmd = c.startBusy(cmd)
	}
	return cmd
}

// openPopup shows p and runs load, the command that fills it.
func (c *chrome) openPopup(p popup, load tea.Cmd) tea.Cmd {
	p.setSize(popupSize(c.width, c.bodyHeight()))
	c.popup = p
	return c.startBusy(load)
}

// newView wraps the rendered screen in a full-window view themed to match.
func (c chrome) newView(content string) tea.View {
	v := tea.NewView(content)
	v.AltScreen = true
	v.BackgroundColor = styles.Background()
	v.ForegroundColor = styles.Foreground()
	return v
}

// headerView renders the title bar with the screen's parts after the name.
func (c chrome) headerView(parts []string) string {
	parts = append([]string{styles.Title().Render("jt")}, parts...)
	return styles.Header().Width(c.width).Render(strings.Join(parts, styles.Dim().Render(" · ")))
}

// footerView renders the quit prompt, or else the status and key help.
func (c chrome) footerView(km help.KeyMap) string {
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
