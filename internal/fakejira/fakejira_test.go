package fakejira_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/hawkbawk/jt/internal/fakejira"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/tui"
)

var _ tui.Backend = (*fakejira.Backend)(nil)

func columnKeys(b *jira.Board) map[string][]string {
	out := map[string][]string{}
	for _, c := range b.Columns {
		for _, iss := range c.Issues {
			out[c.Name] = append(out[c.Name], iss.Key)
		}
	}
	return out
}

func issueCount(b *jira.Board) int {
	n := 0
	for _, c := range b.Columns {
		n += len(c.Issues)
	}
	return n
}

func TestProjectsAndBoards(t *testing.T) {
	f := fakejira.New()
	ps, err := f.Projects()
	if err != nil || len(ps) != 3 || ps[0].Key != "DEMO" {
		t.Fatalf("projects = %v, %v", ps, err)
	}
	if _, err := f.Board("EMPTY", ""); err == nil {
		t.Fatal("a project without boards should fail to load")
	}

	b, err := f.Board("OPS", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := columnKeys(b)["Doing"]; !slices.Equal(got, []string{"OPS-2", "OPS-3"}) {
		t.Fatalf("Doing = %v, want both mapped statuses", got)
	}

	b, err = f.Board(fakejira.DefaultProject, "")
	if err != nil {
		t.Fatal(err)
	}
	if b.ID != 1 || len(b.Columns) != 4 || len(b.QuickFilters) != 2 || issueCount(b) != 9 {
		t.Fatalf("board = %+v", b)
	}
	if n := len(b.Assignees); n != 5 || b.Assignees[n-1].Name != "Unassigned" {
		t.Fatalf("assignees = %v", b.Assignees)
	}
}

func TestBoardFilters(t *testing.T) {
	f := fakejira.New()
	board, err := f.Board(fakejira.DefaultProject, "")
	if err != nil {
		t.Fatal(err)
	}
	filters := slices.Concat(board.QuickFilters, board.Assignees)
	want := map[string]int{"Bugs": 3, "Backend": 2, "Sam Tester": 2, "Unassigned": 2}
	for _, flt := range filters {
		n, ok := want[flt.Name]
		if !ok {
			continue
		}
		b, err := f.Board(fakejira.DefaultProject, flt.JQL)
		if err != nil {
			t.Fatal(err)
		}
		if got := issueCount(b); got != n {
			t.Errorf("%s (%s) matched %d issues, want %d", flt.Name, flt.JQL, got, n)
		}
	}
}

func TestIssueDetail(t *testing.T) {
	f := fakejira.New()
	d, err := f.Issue("DEMO-1")
	if err != nil {
		t.Fatal(err)
	}
	if d.Summary != "Load large boards page by page" || d.Assignee != "Sam Tester" || d.Reporter != "Ada Lovelace" ||
		d.Priority != "High" || d.Status != "In Progress" || len(d.Comments) != 12 {
		t.Fatalf("detail = %+v", d.Issue)
	}
	if !strings.HasSuffix(d.Description, "The end of the description.") {
		t.Fatal("description should be long enough to scroll")
	}
	if _, err := f.Issue("DEMO-404"); err == nil {
		t.Fatal("missing issue should fail")
	}
}

func TestMoveAssignAndComment(t *testing.T) {
	f := fakejira.New()
	iss := jira.Issue{Key: "DEMO-3"}
	ts, err := f.Transitions(iss)
	if err != nil || len(ts) != 3 {
		t.Fatalf("transitions = %v, %v", ts, err)
	}
	if err := f.Transition("DEMO-3", ts[len(ts)-1].ID); err != nil {
		t.Fatal(err)
	}
	if err := f.Transition("DEMO-3", "nope"); err == nil {
		t.Fatal("unknown transition should fail")
	}

	users, err := f.SearchUsers("DEMO-3", "gr")
	if err != nil || len(users) != 1 || users[0].ID != "user-grace" {
		t.Fatalf("users = %v, %v", users, err)
	}
	if users, _ := f.SearchUsers("DEMO-3", "former"); len(users) != 0 {
		t.Fatal("inactive users should be left out")
	}
	if err := f.Assign("DEMO-3", "user-old"); err == nil {
		t.Fatal("inactive users can't be assigned")
	}
	if err := f.Assign("DEMO-3", "user-grace"); err != nil {
		t.Fatal(err)
	}
	if err := f.AddComment("DEMO-3", "Done and **dusted**"); err != nil {
		t.Fatal(err)
	}

	d, err := f.Issue("DEMO-3")
	if err != nil {
		t.Fatal(err)
	}
	if d.Status != "Done" || d.Assignee != "Grace Hopper" || len(d.Comments) != 1 || d.Comments[0].Author != "Sam Tester" {
		t.Fatalf("detail = %+v comments=%v", d.Issue, d.Comments)
	}

	if err := f.Assign("DEMO-3", ""); err != nil {
		t.Fatal(err)
	}
	if d, _ := f.Issue("DEMO-3"); d.Assignee != "" {
		t.Fatalf("assignee = %q after unassigning", d.Assignee)
	}
}

func TestEditFields(t *testing.T) {
	f := fakejira.New()
	iss := jira.Issue{Key: "DEMO-1"}
	fields, err := f.EditFields(iss)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]jira.EditField{}
	for _, fld := range fields {
		byID[fld.ID] = fld
	}
	if r := byID["customfield_10020"]; !slices.Equal(r.Options, []jira.Option{{ID: "user-grace", Name: "Grace Hopper"}}) {
		t.Fatalf("reviewer options = %v, want the chosen people", r.Options)
	}

	summary, points, team, priority := byID["summary"], byID["customfield_10016"], byID["customfield_10030"], byID["priority"]
	summary.Text = "Renamed"
	points.Text = "8.0"
	team.Chosen = []string{"201"}
	priority.Chosen = []string{"5"}
	if err := f.Edit("DEMO-1", []jira.EditField{summary, points, team, priority}); err != nil {
		t.Fatal(err)
	}
	d, _ := f.Issue("DEMO-1")
	if d.Summary != "Renamed" || d.Priority != "Lowest" {
		t.Fatalf("detail = %+v", d.Issue)
	}
	fields, _ = f.EditFields(iss)
	for _, fld := range fields {
		if fld.ID == "customfield_10016" && fld.Text != "8" {
			t.Errorf("points = %q", fld.Text)
		}
	}

	for _, bad := range []func(){
		func() { summary.Text = " " },
		func() { summary.Text, points.Text = "Fine", "eight" },
		func() { points.Text, team.Chosen = "8", []string{"999"} },
	} {
		bad()
		if err := f.Edit("DEMO-1", []jira.EditField{summary, points, team}); err == nil {
			t.Errorf("edit should fail: summary=%q points=%q team=%v", summary.Text, points.Text, team.Chosen)
		}
	}
	if d, _ := f.Issue("DEMO-1"); d.Summary != "Renamed" {
		t.Fatal("a failed edit should change nothing")
	}
}
