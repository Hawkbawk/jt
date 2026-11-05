package board

import (
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
