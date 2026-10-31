package fakejira_test

import (
	"testing"

	"github.com/hawkbawk/jt/internal/fakejira"
	"github.com/hawkbawk/jt/internal/tui"
)

var _ tui.Backend = (*fakejira.Backend)(nil)

func TestBoardLoadsAndMovesPersist(t *testing.T) {
	f := fakejira.New()
	b, err := f.Board(fakejira.DefaultProject, "")
	if err != nil || len(b.Columns) == 0 {
		t.Fatalf("board = %v, %v", b, err)
	}
	iss := b.Columns[0].Issues[0]
	ts, err := f.Transitions(iss)
	if err != nil || len(ts) == 0 {
		t.Fatalf("transitions = %v, %v", ts, err)
	}
	if err := f.Transition(iss.Key, ts[0].ID); err != nil {
		t.Fatal(err)
	}
	d, err := f.Issue(iss.Key)
	if err != nil || d.Status != ts[0].Name {
		t.Fatalf("after move: %+v, %v", d, err)
	}
}
