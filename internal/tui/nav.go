package tui

import "github.com/ryanhawkins/jt/internal/jira"

// boardNav tracks the focused column and the cursor within each column.
type boardNav struct {
	board *jira.Board
	col   int
	rows  []int
}

func newBoardNav(b *jira.Board) boardNav {
	return boardNav{board: b, rows: make([]int, len(b.Columns))}
}

func (n *boardNav) left() {
	if n.col > 0 {
		n.col--
	}
}

func (n *boardNav) right() {
	if n.col < len(n.board.Columns)-1 {
		n.col++
	}
}

func (n *boardNav) up() {
	if n.rows[n.col] > 0 {
		n.rows[n.col]--
	}
}

func (n *boardNav) down() {
	if n.rows[n.col] < len(n.board.Columns[n.col].Issues)-1 {
		n.rows[n.col]++
	}
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
	return &issues[n.rows[n.col]]
}

// focus moves the cursor to the issue with the given key, if present.
func (n *boardNav) focus(key string) bool {
	for c, col := range n.board.Columns {
		for r, iss := range col.Issues {
			if iss.Key == key {
				n.col, n.rows[c] = c, r
				return true
			}
		}
	}
	return false
}
