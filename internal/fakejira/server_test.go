package fakejira

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/hawkbawk/jt/internal/config"
	"github.com/hawkbawk/jt/internal/jira"
)

func newClient(t *testing.T, project string, boardID int) *jira.Client {
	t.Helper()
	srv := httptest.NewServer(New())
	t.Cleanup(srv.Close)
	return jira.New(config.Jira{Server: srv.URL, Login: Login, Project: project, BoardID: boardID}, "token")
}

func columnKeys(b *jira.Board) map[string][]string {
	out := map[string][]string{}
	for _, c := range b.Columns {
		for _, iss := range c.Issues {
			out[c.Name] = append(out[c.Name], iss.Key)
		}
	}
	return out
}

func TestLoginProjectsAndBoards(t *testing.T) {
	c := newClient(t, DefaultProject, 0)
	if err := c.CheckLogin(); err != nil {
		t.Fatal(err)
	}
	ps, err := c.Projects()
	if err != nil || len(ps) != 3 || ps[0].Key != "DEMO" {
		t.Fatalf("projects = %v, %v", ps, err)
	}
	bs, err := c.Boards("OPS")
	if err != nil || len(bs) != 1 || bs[0].Type != "scrum" {
		t.Fatalf("boards = %v, %v", bs, err)
	}
	if _, err := c.Board("EMPTY", ""); err == nil {
		t.Fatal("a project without boards should fail to load")
	}
}

func TestUnauthenticatedRequestsFail(t *testing.T) {
	srv := httptest.NewServer(New())
	defer srv.Close()
	res, err := http.Get(srv.URL + "/rest/api/2/myself")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d", res.StatusCode)
	}
}

func TestBoardColumnsAndFilters(t *testing.T) {
	c := newClient(t, "OPS", 2)
	b, err := c.Board("OPS", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := columnKeys(b)["Doing"]; !slices.Equal(got, []string{"OPS-2", "OPS-3"}) {
		t.Fatalf("Doing = %v, want both mapped statuses", got)
	}

	c = newClient(t, DefaultProject, 0)
	b, err = c.Board(DefaultProject, "")
	if err != nil {
		t.Fatal(err)
	}
	if b.ID != 1 || len(b.Columns) != 4 || len(b.QuickFilters) != 2 {
		t.Fatalf("board = %+v", b)
	}
	for _, f := range []struct {
		jql  string
		want int
	}{
		{"issuetype = Bug", 3},
		{"labels = backend", 2},
		{`assignee = "user-sam"`, 2},
		{"assignee is EMPTY", 2},
	} {
		fb, err := c.Board(DefaultProject, f.jql)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, keys := range columnKeys(fb) {
			n += len(keys)
		}
		if n != f.want {
			t.Errorf("%s matched %d issues, want %d", f.jql, n, f.want)
		}
	}
}

func TestIssueDetail(t *testing.T) {
	c := newClient(t, DefaultProject, 0)
	d, err := c.Issue("DEMO-1")
	if err != nil {
		t.Fatal(err)
	}
	if d.Summary != "Load large boards page by page" || d.Assignee != "Sam Tester" || d.Reporter != "Ada Lovelace" ||
		d.Priority != "High" || d.Status != "In Progress" || len(d.Comments) != 12 {
		t.Fatalf("detail = %+v", d.Issue)
	}
	for _, want := range []string{"## Background", "- A spinner shows", "for start := 0", "The end of the description."} {
		if !strings.Contains(d.Description, want) {
			t.Errorf("description missing %q:\n%s", want, d.Description)
		}
	}
	if _, err := c.Issue("DEMO-404"); err == nil {
		t.Fatal("missing issue should fail")
	}
}

func TestMoveAssignAndComment(t *testing.T) {
	c := newClient(t, DefaultProject, 0)
	iss := jira.Issue{Key: "DEMO-3", Type: "Task", StatusID: statusToDo.ID}
	ts, err := c.Transitions(iss)
	if err != nil || len(ts) != 3 {
		t.Fatalf("transitions = %v, %v", ts, err)
	}
	if err := c.Transition("DEMO-3", statusDone.ID); err != nil {
		t.Fatal(err)
	}
	if err := c.Transition("DEMO-3", "nope"); err == nil {
		t.Fatal("unknown transition should fail")
	}

	users, err := c.SearchUsers("DEMO-3", "gr")
	if err != nil || len(users) != 1 || users[0].ID != "user-grace" {
		t.Fatalf("users = %v, %v", users, err)
	}
	if users, _ := c.SearchUsers("DEMO-3", "former"); len(users) != 0 {
		t.Fatal("inactive users should be left out")
	}
	if err := c.Assign("DEMO-3", "user-grace"); err != nil {
		t.Fatal(err)
	}
	if err := c.AddComment("DEMO-3", "Done and **dusted**"); err != nil {
		t.Fatal(err)
	}

	d, err := c.Issue("DEMO-3")
	if err != nil {
		t.Fatal(err)
	}
	if d.Status != "Done" || d.Assignee != "Grace Hopper" || len(d.Comments) != 1 || d.Comments[0].Author != "Sam Tester" {
		t.Fatalf("detail = %+v comments=%v", d.Issue, d.Comments)
	}

	if err := c.Assign("DEMO-3", ""); err != nil {
		t.Fatal(err)
	}
	if d, _ := c.Issue("DEMO-3"); d.Assignee != "" {
		t.Fatalf("assignee = %q after unassigning", d.Assignee)
	}
}

func TestEditFields(t *testing.T) {
	c := newClient(t, DefaultProject, 0)
	iss := jira.Issue{Key: "DEMO-1", Type: "Story", StatusID: statusInProgress.ID}
	fields, err := c.EditFields(iss)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]jira.EditField{}
	for _, f := range fields {
		byID[f.ID] = f
	}
	if len(byID) != len(fieldDefs()) {
		t.Fatalf("got %d editable fields, want %d", len(byID), len(fieldDefs()))
	}
	if f := byID["customfield_10030"]; f.Kind != jira.FieldSelect || !slices.Equal(f.Chosen, []string{"200"}) {
		t.Fatalf("team = %+v", f)
	}

	summary, points, team, reviewers := byID["summary"], byID["customfield_10016"], byID["customfield_10030"], byID["customfield_10020"]
	summary.Text = "Renamed"
	points.Text = "8"
	team.Chosen = []string{"201"}
	reviewers.Chosen = []string{"user-ada", "user-alan"}
	if err := c.Edit("DEMO-1", []jira.EditField{summary, points, team, reviewers}); err != nil {
		t.Fatal(err)
	}

	fields, err = c.EditFields(iss)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fields {
		switch f.ID {
		case "summary":
			if f.Text != "Renamed" {
				t.Errorf("summary = %q", f.Text)
			}
		case "customfield_10016":
			if f.Text != "8" {
				t.Errorf("points = %q", f.Text)
			}
		case "customfield_10030":
			if !slices.Equal(f.Chosen, []string{"201"}) {
				t.Errorf("team = %v", f.Chosen)
			}
		case "customfield_10020":
			if !slices.Equal(f.Chosen, []string{"user-ada", "user-alan"}) {
				t.Errorf("reviewers = %v", f.Chosen)
			}
		}
	}

	summary.Text = " "
	if err := c.Edit("DEMO-1", []jira.EditField{summary}); err == nil {
		t.Fatal("clearing a required field should fail")
	}
	team.Chosen = []string{"999"}
	if err := c.Edit("DEMO-1", []jira.EditField{team}); err == nil {
		t.Fatal("an unknown option should fail")
	}
}

func TestBrowse(t *testing.T) {
	srv := httptest.NewServer(New())
	defer srv.Close()
	res, err := http.Get(srv.URL + "/browse/DEMO-2")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
}
