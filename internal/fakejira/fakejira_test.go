package fakejira_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/hawkbawk/jt/internal/command"
	"github.com/hawkbawk/jt/internal/fakejira"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/tui/board"
	"github.com/hawkbawk/jt/internal/tui/search"
)

var (
	_ board.Backend  = (*fakejira.Backend)(nil)
	_ search.Backend = (*fakejira.Backend)(nil)
)

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

	word := strings.ToUpper(strings.Fields(all[1].Summary)[0])
	text, _ := f.Search(`textfields ~ "` + word + `"`)
	if !slices.ContainsFunc(text, func(i jira.Issue) bool { return i.Key == all[1].Key }) || len(text) == len(all) {
		t.Errorf("text search for %q found %d of %d issues", word, len(text), len(all))
	}
	none, _ := f.Search(`created >= "2999-01-01 00:00"`)
	if len(none) != 0 {
		t.Errorf("future search found %d issues", len(none))
	}
}

func TestSeedRepeatsData(t *testing.T) {
	a, b, other := fakejira.New(fakejira.WithSeed(7)), fakejira.New(fakejira.WithSeed(7)), fakejira.New(fakejira.WithSeed(8))
	if a.Seed() != 7 {
		t.Fatalf("Seed() = %d", a.Seed())
	}
	const jql = `project = "DEMO" ORDER BY key ASC`
	issues := func(f *fakejira.Backend) []jira.Issue {
		t.Helper()
		out, err := f.Search(jql)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	if !slices.Equal(issues(a), issues(b)) {
		t.Error("the same seed generated different issues")
	}
	if slices.Equal(issues(a), issues(other)) {
		t.Error("different seeds generated the same issues")
	}
}

func TestGeneratedDataVaries(t *testing.T) {
	f := fakejira.New(fakejira.WithSeed(1))
	all, err := f.Search(`project = "DEMO" ORDER BY key ASC`)
	if err != nil || len(all) < 50 {
		t.Fatalf("got %d issues, %v", len(all), err)
	}
	counts, lengths, unassigned := map[int]bool{}, map[int]bool{}, 0
	for _, iss := range all {
		c, err := f.Comments(iss.Key)
		if err != nil {
			t.Fatal(err)
		}
		counts[c.Total] = true
		d, err := f.Issue(iss.Key)
		if err != nil {
			t.Fatal(err)
		}
		lengths[len(d.Description)/200] = true
		if iss.Assignee.ID == "" {
			unassigned++
		}
	}
	if len(counts) < 10 || len(lengths) < 5 || unassigned == 0 {
		t.Errorf("little variety: %d comment counts, %d description lengths, %d unassigned", len(counts), len(lengths), unassigned)
	}
	users, _ := f.SearchUsers("", "")
	if len(users) < 20 {
		t.Errorf("found %d assignable users", len(users))
	}
}

func TestWithSize(t *testing.T) {
	f := fakejira.New(fakejira.WithSeed(1), fakejira.WithSize(fakejira.Size{
		Users: 6, DemoIssues: 12, OpsIssues: 4,
	}))
	demo, _ := f.Search(`project = "DEMO"`)
	ops, _ := f.Search(`project = "OPS"`)
	if len(demo) != 12 || len(ops) != 4 {
		t.Fatalf("got %d DEMO and %d OPS issues", len(demo), len(ops))
	}
	if users, _ := f.SearchUsers("", ""); len(users) > 6 {
		t.Errorf("found %d users", len(users))
	}
}

func TestDemoCommands(t *testing.T) {
	commands := fakejira.DemoCommands()
	if len(commands) != 3 {
		t.Fatalf("got %d demo commands, want 3", len(commands))
	}
	if commands[0].Command != "tmux" || !strings.Contains(commands[0].Name, "Neovim") {
		t.Fatalf("first demo command = %+v", commands[0])
	}
	args, err := command.RenderArgs(commands[0].Args, command.Data{
		Key: "DEMO-1", Title: `Title with "'; $(unsafe)`,
		URL: "https://jira.example.com/browse/DEMO-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if args[2] != "DEMO-1" || args[4] != "JT_DEMO_KEY=DEMO-1" ||
		args[6] != `JT_DEMO_TITLE=Title with "'; $(unsafe)` ||
		args[8] != "JT_DEMO_URL=https://jira.example.com/browse/DEMO-1" ||
		strings.Contains(args[9], "unsafe") {
		t.Fatalf("Neovim command does not show ticket details: %#v", args)
	}
}

func TestCreateAddsIssue(t *testing.T) {
	b := fakejira.New(fakejira.WithSeed(1))
	types, err := b.CreateIssueTypes("DEMO")
	if err != nil || len(types) == 0 {
		t.Fatalf("types = %v, err = %v", types, err)
	}
	var bug jira.IssueType
	for _, ty := range types {
		if ty.Name == "Bug" {
			bug = ty
		}
	}
	fields, err := b.CreateFields("DEMO", bug.ID)
	if err != nil || !slices.ContainsFunc(fields, func(f jira.IssueField) bool { return f.Name == "Steps to reproduce" && f.Required }) {
		t.Fatalf("bugs should have their own required field: %v, err = %v", fields, err)
	}
	summary := jira.IssueField{ID: "summary", Kind: jira.FieldText, Text: "Broken"}
	if _, err := b.Create("DEMO", bug.ID, []jira.IssueField{summary}); err == nil {
		t.Fatal("create should need the bug's required field")
	}

	steps := jira.IssueField{ID: "customfield_10050", Kind: jira.FieldTextArea, Text: "Click it"}
	key, err := b.Create("DEMO", bug.ID, []jira.IssueField{summary, steps})
	if err != nil {
		t.Fatal(err)
	}
	found, err := b.Issue(key)
	if err != nil || found.Summary != "Broken" || found.Type != "Bug" || found.Status != "To Do" {
		t.Fatalf("created issue = %+v, err = %v", found, err)
	}
	if _, err := b.CreateFields("EMPTY", bug.ID); err == nil {
		t.Error("EMPTY has no issue types to create")
	}
}
