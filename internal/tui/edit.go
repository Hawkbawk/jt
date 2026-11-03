package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/popup"
)

// issueChromeLines is what the move, assign and edit popups draw around their
// body: a title and a blank line above it, and the help footer below.
const issueChromeLines = 3

// issueHeader is the title the move, assign and edit popups share.
func issueHeader(title, issueKey string) []string {
	return []string{styles.Title().Render(title + " · " + issueKey), ""}
}

// paneKeys is the help footer for the move, assign and edit popups. The
// bindings only carry help text; the matching happens against the shared keys.
type paneKeys struct {
	choose, apply, cancel key.Binding
}

var paneHelpKeys = paneKeys{
	choose: key.NewBinding(key.WithHelp("j/k", "select")),
	apply:  key.NewBinding(key.WithHelp("enter", "apply")),
	cancel: key.NewBinding(key.WithHelp("esc", "cancel")),
}

// ShortHelp implements help.KeyMap.
func (k paneKeys) ShortHelp() []key.Binding {
	return []key.Binding{k.choose, k.apply, k.cancel}
}

// FullHelp implements help.KeyMap.
func (k paneKeys) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}

// issueEditedMsg reports that a popup changed an issue, so the screen behind it
// can reload.
type issueEditedMsg struct {
	key    string
	status string
}

// closeEdited closes the popup after a change to an issue.
func closeEdited(issueKey, status string) tea.Cmd {
	return popup.CloseWith(issueEditedMsg{key: issueKey, status: status})
}
