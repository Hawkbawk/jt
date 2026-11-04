package fakejira_test

import (
	"fmt"
	"testing"

	"github.com/hawkbawk/jt/internal/fakejira"
	"github.com/hawkbawk/jt/internal/tui"
)

var _ tui.Backend = (*fakejira.Backend)(nil)

func TestBoardLoadsAndMovesPersist(t *testing.T) {
	f := fakejira.New()
	b, err := f.Board(fakejira.DefaultProject, 0, "")
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

func TestBoardsAndBoardByID(t *testing.T) {
	f := fakejira.New()
	boards, err := f.Boards(fakejira.DefaultProject)
	if err != nil || len(boards) < 2 {
		t.Fatalf("boards = %v, %v", boards, err)
	}
	first, _ := f.Board(fakejira.DefaultProject, 0, "")
	other, err := f.Board(fakejira.DefaultProject, boards[1].ID, "")
	if err != nil || other.ID != boards[1].ID || other.ID == first.ID {
		t.Fatalf("board = %+v, %v", other, err)
	}
}

func TestSearch(t *testing.T) {
	f := fakejira.New()
	all, err := f.Search("project = \"DEMO\" ORDER BY key ASC")
	if err != nil || len(all) < 2 {
		t.Fatalf("all = %v, %v", all, err)
	}
	for i := 1; i < len(all); i++ {
		if all[i-1].Key == all[i].Key {
			t.Fatalf("duplicate %s", all[i].Key)
		}
	}

	one, err := f.Search("key = " + all[1].Key)
	if err != nil || len(one) != 1 || one[0].Key != all[1].Key {
		t.Fatalf("by key = %v, %v", one, err)
	}

	desc, _ := f.Search("project = \"DEMO\" ORDER BY key DESC")
	if desc[0].Key != all[len(all)-1].Key {
		t.Errorf("descending starts with %s, want %s", desc[0].Key, all[len(all)-1].Key)
	}

	none, _ := f.Search(`created >= "2999-01-01 00:00"`)
	if len(none) != 0 {
		t.Errorf("future search found %d issues", len(none))
	}
}

func TestNewWithSeedGeneratesRepeatableRichData(t *testing.T) {
	first := fakejira.NewWithSeed(42)
	second := fakejira.NewWithSeed(42)
	other := fakejira.NewWithSeed(43)

	firstIssues, err := first.Search(`project = "DEMO" ORDER BY key ASC`)
	if err != nil {
		t.Fatal(err)
	}
	secondIssues, err := second.Search(`project = "DEMO" ORDER BY key ASC`)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstIssues) < 50 || len(firstIssues) != len(secondIssues) {
		t.Fatalf("seeded issues = %d and %d, want at least 50 and equal counts", len(firstIssues), len(secondIssues))
	}
	for i := range firstIssues {
		if firstIssues[i].Summary != secondIssues[i].Summary ||
			firstIssues[i].Status != secondIssues[i].Status ||
			firstIssues[i].Assignee != secondIssues[i].Assignee {
			t.Fatalf("issue %s differs for the same seed", firstIssues[i].Key)
		}
	}
	otherDetail, err := other.Issue(firstIssues[2].Key)
	if err != nil {
		t.Fatal(err)
	}
	if firstIssues[2].Summary == otherDetail.Issue.Summary {
		t.Fatal("different seeds produced the same generated summary")
	}

	firstDetail, err := first.Issue("DEMO-3")
	if err != nil {
		t.Fatal(err)
	}
	secondDetail, err := second.Issue("DEMO-3")
	if err != nil {
		t.Fatal(err)
	}
	if firstDetail.Description != secondDetail.Description || len(firstDetail.Comments) != len(secondDetail.Comments) {
		t.Fatal("issue details differ for the same seed")
	}
	for i := range firstDetail.Comments {
		if firstDetail.Comments[i].Body != secondDetail.Comments[i].Body {
			t.Fatalf("comment %d differs for the same seed", i)
		}
	}

	commentCounts := map[int]bool{}
	descriptionLengths := map[int]bool{}
	for n := 3; n <= 60; n++ {
		detail, err := first.Issue(fmt.Sprintf("DEMO-%d", n))
		if err != nil {
			t.Fatal(err)
		}
		commentCounts[len(detail.Comments)] = true
		descriptionLengths[len(detail.Description)] = true
	}
	if len(commentCounts) < 3 || len(descriptionLengths) < 3 {
		t.Fatalf("insufficient variation: comment counts=%v, description lengths=%v", commentCounts, descriptionLengths)
	}

	users, err := first.SearchUsers("", "")
	if err != nil || len(users) < 15 {
		t.Fatalf("active users = %d, %v; want at least 15", len(users), err)
	}
}
