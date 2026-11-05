package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hawkbawk/jt/internal/keys"
	"github.com/hawkbawk/jt/internal/styles"
)

// quitGuard implements the quit keys: ctrl+d exits at once, while ctrl+c asks
// for confirmation first.
type quitGuard struct {
	confirming bool
}

// handle reports whether msg was consumed and whether the app should exit.
func (q *quitGuard) handle(msg tea.KeyPressMsg) (consumed, quit bool) {
	switch {
	case key.Matches(msg, keys.Get().ForceQuit):
		return true, true
	case key.Matches(msg, keys.Get().Interrupt):
		if q.confirming {
			return true, true
		}
		q.confirming = true
		return true, false
	}
	if !q.confirming {
		return false, false
	}
	switch {
	case key.Matches(msg, keys.Get().Confirm):
		return true, true
	case key.Matches(msg, keys.Get().Deny):
		q.confirming = false
	}
	return true, false
}

func (q quitGuard) view() string {
	h := styles.Help()
	prompt := styles.Error().Bold(true).Render("Quit jt?")
	return lipgloss.NewStyle().Padding(0, 1).Render(prompt + " " + h.View(newQuitKeys()))
}
