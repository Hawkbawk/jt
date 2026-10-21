package tui

import "github.com/hawkbawk/jt/internal/jira"

// boardNav tracks the focused column and a row shared by every column, so
// moving between columns behaves like moving across a grid.
type boardNav struct {
	board *jira.Board
	col   int
	row   int
}

func newBoardNav(b *jira.Board) boardNav {
	return boardNav{board: b}
}

func (n *boardNav) left() {
	if n.col > 0 {
		n.col--
		n.row = n.rowIn(n.col)
	}
}

func (n *boardNav) right() {
	if n.col < len(n.board.Columns)-1 {
		n.col++
		n.row = n.rowIn(n.col)
	}
}

func (n *boardNav) up() {
	if n.row > 0 {
		n.row--
	}
}

func (n *boardNav) down() {
	if n.row < len(n.board.Columns[n.col].Issues)-1 {
		n.row++
	}
}

func (n *boardNav) top() {
	n.row = 0
}

func (n *boardNav) bottom() {
	n.row = max(0, len(n.board.Columns[n.col].Issues)-1)
}

// rowIn returns the current row clamped to the last issue in column c.
func (n *boardNav) rowIn(c int) int {
	return max(0, min(n.row, len(n.board.Columns[c].Issues)-1))
}

// selected returns the focused issue, or nil when the column is empty.
func (n *boardNav) selected() *jira.Issue {
	if n.board == nil || len(n.board.Columns) == 0 {
		return nil
	}
	issues := n.board.Columns[n.col].Issues
	if len(issues) == 0 {
		return nil
	}
	return &issues[n.row]
}

// focus moves the cursor to the issue with the given key, if present.
func (n *boardNav) focus(key string) bool {
	for c, col := range n.board.Columns {
		for r, iss := range col.Issues {
			if iss.Key == key {
				n.col, n.row = c, r
				return true
			}
		}
	}
	return false
}
