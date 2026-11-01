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

func TestSearch(t *testing.T) {
	f := fakejira.New()
	all, err := f.Search(t.Context(), "project = \"DEMO\" ORDER BY key ASC")
	if err != nil || len(all) < 2 {
		t.Fatalf("all = %v, %v", all, err)
	}
	for i := 1; i < len(all); i++ {
		if all[i-1].Key == all[i].Key {
			t.Fatalf("duplicate %s", all[i].Key)
		}
	}

	one, err := f.Search(t.Context(), "key = "+all[1].Key)
	if err != nil || len(one) != 1 || one[0].Key != all[1].Key {
		t.Fatalf("by key = %v, %v", one, err)
	}

	desc, _ := f.Search(t.Context(), "project = \"DEMO\" ORDER BY key DESC")
	if desc[0].Key != all[len(all)-1].Key {
		t.Errorf("descending starts with %s, want %s", desc[0].Key, all[len(all)-1].Key)
	}

	none, _ := f.Search(t.Context(), `created >= "2999-01-01 00:00"`)
	if len(none) != 0 {
		t.Errorf("future search found %d issues", len(none))
	}
}
