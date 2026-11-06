package tui

import (
	"slices"

	"charm.land/bubbles/v2/key"

	"github.com/hawkbawk/jt/internal/keys"
)

// boardKeyMap is the board's help footer.
type boardKeyMap struct {
	navigate, topBottom, view, comments, move, assign key.Binding
	edit, yank, open, filter, theme, refresh          key.Binding
	projects, boards, quit                            key.Binding
}

func newBoardKeys() boardKeyMap {
	k := keys.Get()
	return boardKeyMap{
		navigate: k.Navigate, topBottom: k.TopBottom, view: k.View, comments: k.Comments,
		move: k.Move, assign: k.Assign, edit: k.Edit, yank: k.Yank, open: k.Open,
		filter: k.Filter, theme: k.Theme, refresh: k.Refresh,
		projects: k.Projects, boards: k.Boards, quit: k.Quit,
	}
}

// ShortHelp implements help.KeyMap.
func (k boardKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.navigate, k.topBottom, k.view, k.comments, k.move, k.assign, k.edit, k.yank, k.open, k.filter, k.theme, k.refresh, k.projects, k.boards, k.quit}
}

// FullHelp implements help.KeyMap.
func (k boardKeyMap) FullHelp() [][]key.Binding { return [][]key.Binding{k.ShortHelp()} }

// searchKeyMap is the search screen's help footer. form holds the focused
// form field's own bindings; the other fields say which footer to show.
type searchKeyMap struct {
	listNavigate, topBottom, viewResult, comments, move, assign key.Binding
	edit, yank, open, refresh, theme, back, quit                key.Binding
	search, jqlSearch, switchMode, focusResults                 key.Binding

	form                        []key.Binding
	results, searched, advanced bool
}

func newSearchKeys() searchKeyMap {
	k := keys.Get()
	return searchKeyMap{
		listNavigate: k.ListNavigate, topBottom: k.TopBottom, viewResult: k.ViewResult,
		comments: k.Comments, move: k.Move, assign: k.Assign, edit: k.Edit, yank: k.Yank,
		open: k.Open, refresh: k.Refresh, theme: k.Theme, back: k.Back, quit: k.Quit,
		search:       keys.Describe(k.Send, "search"),
		jqlSearch:    keys.Describe(k.Accept, "search"),
		switchMode:   k.SwitchMode,
		focusResults: k.FocusResults,
	}
}

// ShortHelp implements help.KeyMap.
func (k searchKeyMap) ShortHelp() []key.Binding {
	if k.results {
		return []key.Binding{k.listNavigate, k.topBottom, k.viewResult, k.comments, k.move, k.assign, k.edit, k.yank, k.open, k.refresh, k.theme, k.back, k.quit}
	}
	binds := append(slices.Clone(k.form), k.search, k.switchMode)
	if k.searched {
		binds = append(binds, k.focusResults)
	}
	if k.advanced {
		binds = append(binds, k.jqlSearch)
	}
	return binds
}

// FullHelp implements help.KeyMap.
func (k searchKeyMap) FullHelp() [][]key.Binding { return [][]key.Binding{k.ShortHelp()} }
