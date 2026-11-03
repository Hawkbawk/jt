package tui

import (
	tea "charm.land/bubbletea/v2"
)

// legacyPopup is a modal shown over the board. update returns nil to close it.
type legacyPopup interface {
	update(tea.Msg) (legacyPopup, tea.Cmd)
	view(spin string) string
	setSize(width, height int)
}
