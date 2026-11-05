package screen

import (
	tea "charm.land/bubbletea/v2"
)

// Settings persists the theme choice.
type Settings interface {
	SaveTheme(id string) error
}

// ThemeSavedMsg reports whether the picked tint was written to the settings.
type ThemeSavedMsg struct {
	Err error
}

// SaveTheme writes the theme id to s off the update loop.
func SaveTheme(s Settings, id string) tea.Cmd {
	return func() tea.Msg {
		return ThemeSavedMsg{Err: s.SaveTheme(id)}
	}
}
