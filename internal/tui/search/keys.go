package search

import (
	"slices"

	"charm.land/bubbles/v2/key"

	"github.com/hawkbawk/jt/internal/keys"
)

// searchKeyMap is the search screen's help footer. form holds the focused
// form field's own bindings; the other fields say which footer to show.
type searchKeyMap struct {
	listNavigate, topBottom, viewResult, comments, commands, move, assign key.Binding
	edit, yank, open, refresh, theme, back, help, quit                    key.Binding
	search, jqlSearch, switchMode, showFilters, focusResults              key.Binding

	form                        []key.Binding
	results, searched, advanced bool
}

func newSearchKeys() searchKeyMap {
	k := keys.Get()
	return searchKeyMap{
		listNavigate: k.ListNavigate, topBottom: k.TopBottom, viewResult: k.ViewResult,
		comments: k.Comments, commands: k.Commands, move: k.Move, assign: k.Assign, edit: k.Edit, yank: k.Yank,
		open: k.Open, refresh: k.Refresh, theme: k.Theme, back: k.Back, help: k.Help, quit: k.Quit,
		search:       keys.Describe(k.Send, "search"),
		jqlSearch:    keys.Describe(k.Accept, "search"),
		switchMode:   k.SwitchMode,
		showFilters:  k.ShowFilters,
		focusResults: k.FocusResults,
	}
}

// ShortHelp implements help.KeyMap.
func (k searchKeyMap) ShortHelp() []key.Binding {
	if k.results {
		return []key.Binding{k.listNavigate, k.viewResult, k.commands, k.back, k.help, k.quit}
	}
	binds := append(slices.Clone(k.form), k.search, k.switchMode)
	if !k.advanced {
		binds = append(binds, k.showFilters)
	}
	if k.searched {
		binds = append(binds, k.focusResults)
	}
	if k.advanced {
		binds = append(binds, k.jqlSearch)
	}
	return binds
}

// FullHelp implements help.KeyMap.
func (k searchKeyMap) FullHelp() [][]key.Binding {
	if !k.results {
		return [][]key.Binding{k.ShortHelp()}
	}
	return [][]key.Binding{
		{k.listNavigate, k.topBottom, k.back},
		{k.viewResult, k.comments, k.edit, k.move, k.assign},
		{k.commands, k.yank, k.open, k.refresh, k.theme},
		{k.help, k.quit},
	}
}
