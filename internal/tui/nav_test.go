package tui

import (
	"testing"

	"github.com/hawkbawk/jt/internal/jira"
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
	if n.col != 0 || n.row != 0 {
		t.Fatalf("moved past top-left: col=%d row=%d", n.col, n.row)
	}
	for range 5 {
		n.down()
		n.right()
	}
	if n.col != 2 || n.selected().Key != "T-4" {
		t.Fatalf("expected T-4 in last column, got col=%d %v", n.col, n.selected())
	}
}

func gridBoard() *jira.Board {
	return &jira.Board{
		Columns: []jira.Column{
			{Name: "A", Issues: []jira.Issue{{Key: "A-1"}, {Key: "A-2"}, {Key: "A-3"}, {Key: "A-4"}}},
			{Name: "B", Issues: []jira.Issue{{Key: "B-1"}, {Key: "B-2"}, {Key: "B-3"}, {Key: "B-4"}}},
			{Name: "C", Issues: []jira.Issue{{Key: "C-1"}, {Key: "C-2"}}},
		},
	}
}

func TestNavKeepsRowAcrossColumns(t *testing.T) {
	n := newBoardNav(gridBoard())
	n.down()
	n.down()
	n.right()
	if got := n.selected().Key; got != "B-3" {
		t.Fatalf("selected %s after moving right from row 3", got)
	}
}

func TestNavClampsRowToShorterColumn(t *testing.T) {
	n := newBoardNav(gridBoard())
	n.right()
	n.down()
	n.down()
	n.right()
	if got := n.selected().Key; got != "C-2" {
		t.Fatalf("selected %s in shorter column", got)
	}
	n.left()
	if got := n.selected().Key; got != "B-2" {
		t.Fatalf("selected %s after returning from shorter column", got)
	}
}

func TestNavTopAndBottom(t *testing.T) {
	n := newBoardNav(gridBoard())
	n.bottom()
	if got := n.selected().Key; got != "A-4" {
		t.Fatalf("bottom selected %s", got)
	}
	n.top()
	if got := n.selected().Key; got != "A-1" {
		t.Fatalf("top selected %s", got)
	}
}

func TestNavBottomInEmptyColumn(t *testing.T) {
	n := newBoardNav(testBoard())
	n.right()
	n.bottom()
	if n.row != 0 || n.selected() != nil {
		t.Fatalf("bottom in empty column: row=%d %v", n.row, n.selected())
	}
}

func TestNavDoesNotRememberRowPerColumn(t *testing.T) {
	n := newBoardNav(gridBoard())
	n.right()
	n.down()
	n.right()
	n.left()
	n.down()
	n.down()
	n.left()
	if got := n.selected().Key; got != "A-4" {
		t.Fatalf("selected %s, expected the shared row", got)
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
