package tui

import (
	"testing"

	"github.com/ryanhawkins/jt/internal/jira"
)

func testBoard() *jira.Board {
	return &jira.Board{
		ID: 1, Name: "Test Board", Project: "TEST",
		Columns: []jira.Column{
			{Name: "Open", Issues: []jira.Issue{{Key: "T-1", Summary: "one"}, {Key: "T-2", Summary: "two"}, {Key: "T-3", Summary: "three"}}},
			{Name: "In Progress"},
			{Name: "Closed", Issues: []jira.Issue{{Key: "T-4", Summary: "four"}}},
		},
	}
}

func TestNavClampsAtEdges(t *testing.T) {
	n := newBoardNav(testBoard())
	n.left()
	n.up()
	if n.col != 0 || n.rows[0] != 0 {
		t.Fatalf("moved past top-left: col=%d row=%d", n.col, n.rows[0])
	}
	for range 5 {
		n.down()
		n.right()
	}
	if n.col != 2 || n.selected().Key != "T-4" {
		t.Fatalf("expected T-4 in last column, got col=%d %v", n.col, n.selected())
	}
}

func TestNavRemembersRowPerColumn(t *testing.T) {
	n := newBoardNav(testBoard())
	n.down()
	n.down()
	n.right()
	n.right()
	n.left()
	n.left()
	if got := n.selected().Key; got != "T-3" {
		t.Fatalf("selected %s after returning to column", got)
	}
}

func TestNavEmptyColumnHasNoSelection(t *testing.T) {
	n := newBoardNav(testBoard())
	n.right()
	n.down()
	if n.selected() != nil {
		t.Fatal("expected no selection in empty column")
	}
}

func TestNavFocus(t *testing.T) {
	n := newBoardNav(testBoard())
	if !n.focus("T-4") || n.col != 2 || n.selected().Key != "T-4" {
		t.Fatal("focus did not move to T-4")
	}
	if n.focus("NOPE-1") {
		t.Fatal("focus reported success for missing key")
	}
}
