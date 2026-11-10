package board

import (
	"charm.land/bubbles/v2/key"

	"github.com/hawkbawk/jt/internal/icons"
	"github.com/hawkbawk/jt/internal/keys"
)

// boardKeyMap is the board's help footer.
type boardKeyMap struct {
	navigate, topBottom, view, comments, move, assign key.Binding
	edit, yank, open, filter, theme, refresh          key.Binding
	projects, boards, commands, help, quit            key.Binding
}

func newBoardKeys() boardKeyMap {
	k := keys.Get()
	return boardKeyMap{
		navigate: k.Navigate, topBottom: k.TopBottom, view: k.View, comments: k.Comments,
		move: k.Move, assign: k.Assign, edit: k.Edit, yank: k.Yank, open: k.Open,
		filter: k.Filter, theme: k.Theme, refresh: k.Refresh,
		projects: k.Projects, boards: k.Boards, commands: k.Commands, help: k.Help, quit: k.Quit,
	}
}

// ShortHelp implements help.KeyMap.
func (k boardKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		keys.WithIcon(k.navigate, icons.Navigate), keys.WithIcon(k.view, icons.View),
		keys.WithIcon(k.move, icons.Move), keys.WithIcon(k.commands, icons.Commands),
		keys.WithIcon(k.help, icons.Help), keys.WithIcon(k.quit, icons.Quit),
	}
}

// FullHelp implements help.KeyMap.
func (k boardKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.navigate, k.topBottom, k.projects, k.boards, k.filter},
		{k.view, k.comments, k.edit, k.move, k.assign},
		{k.commands, k.yank, k.open, k.refresh, k.theme},
		{k.help, k.quit},
	}
}
