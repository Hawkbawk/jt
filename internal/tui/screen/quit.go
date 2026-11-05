package screen

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hawkbawk/jt/internal/keys"
	"github.com/hawkbawk/jt/internal/styles"
)

// QuitGuard implements the quit keys: ctrl+d exits at once, while ctrl+c asks
// for confirmation first.
type QuitGuard struct {
	Confirming bool
}

// Handle reports whether msg was consumed and whether the app should exit.
func (q *QuitGuard) Handle(msg tea.KeyPressMsg) (consumed, quit bool) {
	switch {
	case key.Matches(msg, keys.Get().ForceQuit):
		return true, true
	case key.Matches(msg, keys.Get().Interrupt):
		if q.Confirming {
			return true, true
		}
		q.Confirming = true
		return true, false
	}
	if !q.Confirming {
		return false, false
	}
	switch {
	case key.Matches(msg, keys.Get().Confirm):
		return true, true
	case key.Matches(msg, keys.Get().Deny):
		q.Confirming = false
	}
	return true, false
}

// View renders the quit prompt.
func (q QuitGuard) View() string {
	h := styles.Help()
	prompt := styles.Error().Bold(true).Render("Quit jt?")
	return lipgloss.NewStyle().Padding(0, 1).Render(prompt + " " + h.View(newQuitKeys()))
}

// quitKeyMap is the help shown beside the quit prompt.
type quitKeyMap struct {
	confirm, deny, interrupt key.Binding
}

func newQuitKeys() quitKeyMap {
	k := keys.Get()
	return quitKeyMap{
		confirm:   keys.Describe(k.Confirm, "quit"),
		deny:      keys.Describe(k.Deny, "stay"),
		interrupt: keys.Describe(k.Interrupt, "quit"),
	}
}

// ShortHelp implements help.KeyMap.
func (k quitKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.confirm, k.deny, k.interrupt}
}

// FullHelp implements help.KeyMap.
func (k quitKeyMap) FullHelp() [][]key.Binding { return [][]key.Binding{k.ShortHelp()} }
