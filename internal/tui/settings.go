package tui

import (
	tea "charm.land/bubbletea/v2"
)

// Settings persists the user's choices.
type Settings interface {
	SaveTheme(id string) error
	// SaveBoard sets the board to open for the default project.
	SaveBoard(id int) error
	// SaveProject sets the default project, forgetting the default board when
	// the project changes.
	SaveProject(key string) error
}

// themeSavedMsg reports whether the picked tint was written to the settings.
type themeSavedMsg struct {
	err error
}

func saveTheme(s Settings, id string) tea.Cmd {
	return func() tea.Msg {
		return themeSavedMsg{err: s.SaveTheme(id)}
	}
}

// boardSavedMsg reports whether the favorite board was written to the settings.
type boardSavedMsg struct {
	id  int
	err error
}

func saveBoard(s Settings, id int) tea.Cmd {
	return func() tea.Msg {
		return boardSavedMsg{id: id, err: s.SaveBoard(id)}
	}
}

// projectSavedMsg reports whether the favorite project was written to the
// settings.
type projectSavedMsg struct {
	key string
	err error
}

func saveProject(s Settings, key string) tea.Cmd {
	return func() tea.Msg {
		return projectSavedMsg{key: key, err: s.SaveProject(key)}
	}
}
