package jira

import (
	"encoding/json"
	"testing"
)

func TestPickBoardPrefersBoardLocatedInProject(t *testing.T) {
	var boards []boardInfo
	err := json.Unmarshal([]byte(`[
		{"id": 630, "name": "Canvas PM", "type": "kanban"},
		{"id": 1848, "name": "Interop - Coconut", "type": "scrum", "location": {"projectKey": "INTEROP"}}
	]`), &boards)
	if err != nil {
		t.Fatal(err)
	}

	b, err := pickBoard("INTEROP", boards)
	if err != nil {
		t.Fatal(err)
	}
	if b.ID != 1848 {
		t.Fatalf("picked board %d", b.ID)
	}

	b, _ = pickBoard("OTHER", boards)
	if b.ID != 630 {
		t.Fatalf("expected fallback to first board, got %d", b.ID)
	}

	if _, err := pickBoard("NONE", nil); err == nil {
		t.Fatal("expected error with no boards")
	}
}

func TestBoardJQL(t *testing.T) {
	if boardJQL("scrum") != scrumJQL || boardJQL("kanban") != kanbanJQL {
		t.Fatal("unexpected JQL for board type")
	}
}

func TestColumnsAndDistribute(t *testing.T) {
	var cfg boardConfig
	err := json.Unmarshal([]byte(`{"columnConfig": {"columns": [
		{"name": "Open", "statuses": [{"id": "1"}]},
		{"name": "In Progress", "statuses": [{"id": "3"}, {"id": "4"}]},
		{"name": "Closed", "statuses": [{"id": "6"}]}
	]}}`), &cfg)
	if err != nil {
		t.Fatal(err)
	}

	cols, err := columnsFrom(cfg)
	if err != nil {
		t.Fatal(err)
	}
	distribute(cols, []Issue{
		{Key: "A-1", StatusID: "1"},
		{Key: "A-2", StatusID: "4"},
		{Key: "A-3", StatusID: "3"},
		{Key: "A-4", StatusID: "99"},
	})

	counts := [3]int{len(cols[0].Issues), len(cols[1].Issues), len(cols[2].Issues)}
	if counts != [3]int{1, 2, 0} {
		t.Fatalf("column counts = %v", counts)
	}
	if cols[1].Issues[0].Key != "A-2" {
		t.Fatalf("issue order not preserved: %v", cols[1].Issues)
	}
}

func TestColumnsFromEmptyConfig(t *testing.T) {
	if _, err := columnsFrom(boardConfig{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestTransitionLabel(t *testing.T) {
	cases := []struct {
		t    Transition
		want string
	}{
		{Transition{Name: "In Progress", To: "In Progress"}, "Move to In Progress"},
		{Transition{Name: "Close - Won't Do", To: "Closed"}, "Move to Closed (Close - Won't Do)"},
		{Transition{Name: "Reopen"}, "Move to Reopen"},
	}
	for _, c := range cases {
		if got := c.t.Label(); got != c.want {
			t.Errorf("Label() = %q, want %q", got, c.want)
		}
	}
}
