package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// quitGuard implements the quit keys: ctrl+d exits at once, while ctrl+c asks
// for confirmation first.
type quitGuard struct {
	confirming bool
}

// handle reports whether msg was consumed and whether the app should exit.
func (q *quitGuard) handle(msg tea.KeyMsg) (consumed, quit bool) {
	switch {
	case key.Matches(msg, keys.forceQuit):
		return true, true
	case key.Matches(msg, keys.interrupt):
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
	case key.Matches(msg, keys.confirm):
		return true, true
	case key.Matches(msg, keys.deny):
		q.confirming = false
	}
	return true, false
}

func (q quitGuard) view() string {
	return errorStyle.Bold(true).Padding(0, 1).Render("Quit jt? (y/n, ctrl+c again to quit)")
}
