package fakejira

import (
	"fmt"
	"strings"
	"time"

	"github.com/hawkbawk/jt/internal/jira"
)

// DefaultProject is the project the fake opens on start.
const DefaultProject = "DEMO"

type user struct {
	jira.Option
	Email  string
	Active bool
}

type board struct {
	ID      int
	Name    string
	Type    string
	Project string
	Columns []jira.Column
	Quick   []jira.Filter
}

type comment struct {
	Author  jira.Option
	Body    string
	Created time.Time
}

// value is an editable field's value: Text for text, number and date kinds,
// Chosen for the rest.
type value struct {
	Text   string
	Chosen []jira.Option
}

type issue struct {
	Key      string
	Type     string
	Status   jira.Option
	Assignee jira.Option
	Reporter jira.Option
	Created  time.Time
	Updated  time.Time
	// Values holds the editable fields by ID. Summary, description, priority
	// and labels are read from here too.
	Values   map[string]value
	Comments []comment
}

func (i *issue) project() string {
	p, _, _ := strings.Cut(i.Key, "-")
	return p
}

var (
	statusToDo       = jira.Option{ID: "10000", Name: "To Do"}
	statusInProgress = jira.Option{ID: "3", Name: "In Progress"}
	statusInReview   = jira.Option{ID: "10001", Name: "In Review"}
	statusDone       = jira.Option{ID: "10002", Name: "Done"}

	allStatuses = []jira.Option{statusToDo, statusInProgress, statusInReview, statusDone}
)

var (
	components = []jira.Option{
		{ID: "100", Name: "API"},
		{ID: "101", Name: "Frontend"},
		{ID: "102", Name: "Infrastructure"},
	}
	teams = []jira.Option{
		{ID: "200", Name: "Platform"},
		{ID: "201", Name: "Growth"},
	}
)

var priorities = []jira.Option{
	{ID: "1", Name: "Highest"},
	{ID: "2", Name: "High"},
	{ID: "3", Name: "Medium"},
	{ID: "4", Name: "Low"},
	{ID: "5", Name: "Lowest"},
}

// fieldDefs are the editable fields, in the order the real client sorts them:
// common system fields first, then the rest by name.
func fieldDefs() []jira.EditField {
	return []jira.EditField{
		{ID: "summary", Name: "Summary", Kind: jira.FieldText, Required: true},
		{ID: "description", Name: "Description", Kind: jira.FieldTextArea},
		{ID: "priority", Name: "Priority", Kind: jira.FieldSelect, Options: priorities},
		{ID: "labels", Name: "Labels", Kind: jira.FieldLabels},
		{ID: "components", Name: "Components", Kind: jira.FieldMultiSelect, Options: components},
		{ID: "duedate", Name: "Due date", Kind: jira.FieldDate},
		{ID: "customfield_10040", Name: "Design doc", Kind: jira.FieldText},
		{ID: "customfield_10020", Name: "Reviewers", Kind: jira.FieldMultiUser},
		{ID: "customfield_10016", Name: "Story points", Kind: jira.FieldNumber},
		{ID: "customfield_10030", Name: "Team", Kind: jira.FieldSelect, Options: teams},
	}
}

func seedUsers() []user {
	return []user{
		{Option: jira.Option{ID: "user-sam", Name: "Sam Tester"}, Email: "sam@example.com", Active: true},
		{Option: jira.Option{ID: "user-ada", Name: "Ada Lovelace"}, Email: "ada@example.com", Active: true},
		{Option: jira.Option{ID: "user-grace", Name: "Grace Hopper"}, Email: "grace@example.com", Active: true},
		{Option: jira.Option{ID: "user-alan", Name: "Alan Turing"}, Email: "alan@example.com", Active: true},
		{Option: jira.Option{ID: "user-old", Name: "Former Employee"}, Email: "old@example.com", Active: false},
	}
}

func seedProjects() []jira.Project {
	return []jira.Project{
		{Key: "DEMO", Name: "Demo"},
		{Key: "EMPTY", Name: "Project without boards"},
		{Key: "OPS", Name: "Operations"},
	}
}

func seedBoards() []board {
	return []board{
		{
			ID: 1, Name: "Demo board", Type: "kanban", Project: "DEMO",
			Columns: []jira.Column{
				{Name: "To Do", StatusIDs: []string{statusToDo.ID}},
				{Name: "In Progress", StatusIDs: []string{statusInProgress.ID}},
				{Name: "Review", StatusIDs: []string{statusInReview.ID}},
				{Name: "Done", StatusIDs: []string{statusDone.ID}},
			},
			Quick: []jira.Filter{
				{Name: "Bugs", JQL: "issuetype = Bug"},
				{Name: "Backend", JQL: "labels = backend"},
			},
		},
		{
			ID: 2, Name: "Ops sprint board", Type: "scrum", Project: "OPS",
			Columns: []jira.Column{
				{Name: "Backlog", StatusIDs: []string{statusToDo.ID}},
				{Name: "Doing", StatusIDs: []string{statusInProgress.ID, statusInReview.ID}},
				{Name: "Done", StatusIDs: []string{statusDone.ID}},
			},
		},
	}
}

const longDescription = "## Background\n\n" +
	"The board takes a long time to load for projects with thousands of issues. This ticket tracks paging through the results and showing a spinner while the board loads.\n\n" +
	"## Acceptance criteria\n\n" +
	"- Boards with more than 100 issues load every page\n" +
	"- A spinner shows while the board loads\n" +
	"- Errors from Jira show in the status line\n\n" +
	"## Notes\n\n" +
	"```go\nfor start := 0; start < maxIssues; start += pageSize {\n\t// fetch a page\n}\n```\n\n" +
	"### Steps to reproduce\n\n" +
	"1. Open a project with a large board\n" +
	"2. Wait\n" +
	"3. Keep waiting\n\n" +
	"### Filler for scrolling\n\n"

func seedIssues(now time.Time, users []user) []*issue {
	const day = 24 * time.Hour
	var filler strings.Builder
	for i := 1; i <= 40; i++ {
		fmt.Fprintf(&filler, "Line %d of filler text so the issue view has something to scroll through.\n\n", i)
	}
	filler.WriteString("The end of the description.")

	who := func(id string) jira.Option {
		for _, u := range users {
			if u.ID == id {
				return u.Option
			}
		}
		return jira.Option{}
	}
	mk := func(key, typ string, st jira.Option, assignee, reporter string, age time.Duration, values map[string]value) *issue {
		created := now.Add(-age)
		return &issue{
			Key: key, Type: typ, Status: st, Assignee: who(assignee), Reporter: who(reporter),
			Created: created, Updated: created.Add(age / 2), Values: values,
		}
	}
	text := func(s string) value { return value{Text: s} }
	chosen := func(opts ...jira.Option) value { return value{Chosen: opts} }
	pick := func(opts []jira.Option, ids ...string) value {
		var out []jira.Option
		for _, id := range ids {
			if i := jira.IndexID(opts, id); i >= 0 {
				out = append(out, opts[i])
			}
		}
		return chosen(out...)
	}
	labels := func(names ...string) value {
		var out []jira.Option
		for _, n := range names {
			out = append(out, jira.Option{ID: n, Name: n})
		}
		return chosen(out...)
	}
	prio := func(name string) value {
		for _, p := range priorities {
			if p.Name == name {
				return chosen(p)
			}
		}
		return value{}
	}

	big := mk("DEMO-1", "Story", statusInProgress, "user-sam", "user-ada", 12*day, map[string]value{
		"summary":           text("Load large boards page by page"),
		"description":       text(longDescription + filler.String()),
		"priority":          prio("High"),
		"labels":            labels("backend", "performance"),
		"components":        pick(components, "100"),
		"duedate":           text(now.Add(7 * day).Format("2006-01-02")),
		"customfield_10016": text("5"),
		"customfield_10030": pick(teams, "200"),
		"customfield_10020": chosen(who("user-grace")),
		"customfield_10040": text("https://example.com/design/boards"),
	})
	authors := []string{"user-ada", "user-grace", "user-sam", "user-alan"}
	for i := range 12 {
		big.Comments = append(big.Comments, comment{
			Author:  who(authors[i%len(authors)]),
			Body:    fmt.Sprintf("Update %d: still working through the paging edge cases.\n\nNext up is error handling.", i+1),
			Created: big.Created.Add(time.Duration(i+1) * 12 * time.Hour),
		})
	}

	bug := mk("DEMO-2", "Bug", statusToDo, "user-ada", "user-sam", 3*day, map[string]value{
		"summary":     text("Crash when a column has no statuses"),
		"description": text("Opening a board whose column has no statuses panics.\n\nSee the trace in the comments."),
		"priority":    prio("Highest"),
		"labels":      labels("backend"),
	})
	bug.Comments = []comment{{Author: who("user-sam"), Body: "I can reproduce this on main.", Created: bug.Created.Add(time.Hour)}}

	return []*issue{
		big,
		bug,
		mk("DEMO-3", "Task", statusToDo, "", "user-grace", 2*day, map[string]value{
			"summary":  text("Write release notes for 0.2"),
			"priority": prio("Low"),
		}),
		mk("DEMO-4", "Story", statusToDo, "user-grace", "user-ada", 5*day, map[string]value{
			"summary":     text("Filter the board by assignee"),
			"description": text("Add a filter menu listing everyone with an issue on the board."),
			"priority":    prio("Medium"),
			"labels":      labels("frontend"),
		}),
		mk("DEMO-5", "Bug", statusInProgress, "user-alan", "user-sam", 6*day, map[string]value{
			"summary":     text("Comments with tables render badly"),
			"description": text("Tables in comments lose their borders."),
			"priority":    prio("Medium"),
		}),
		mk("DEMO-6", "Story", statusInReview, "user-sam", "user-grace", 9*day, map[string]value{
			"summary":     text("Copy the issue URL to the clipboard"),
			"description": text("Pressing y copies the issue's browser URL."),
			"priority":    prio("Low"),
			"labels":      labels("frontend"),
		}),
		mk("DEMO-7", "Task", statusInReview, "", "user-ada", 4*day, map[string]value{
			"summary":  text("Bump dependencies"),
			"priority": prio("Lowest"),
		}),
		mk("DEMO-8", "Story", statusDone, "user-ada", "user-sam", 20*day, map[string]value{
			"summary":     text("Pick a theme from bubbletint"),
			"description": text("Press t to choose a theme."),
			"priority":    prio("Medium"),
		}),
		mk("DEMO-9", "Bug", statusDone, "user-grace", "user-alan", 15*day, map[string]value{
			"summary":  text("Status line overflows on narrow terminals"),
			"priority": prio("High"),
			"labels":   labels("frontend"),
		}),
		mk("OPS-1", "Task", statusToDo, "user-alan", "user-alan", day, map[string]value{
			"summary":  text("Rotate the staging certificates"),
			"priority": prio("High"),
		}),
		mk("OPS-2", "Task", statusInProgress, "user-sam", "user-alan", 2*day, map[string]value{
			"summary":     text("Move CI runners to the new cluster"),
			"description": text("Runners should autoscale."),
			"priority":    prio("Medium"),
		}),
		mk("OPS-3", "Bug", statusInReview, "", "user-grace", 3*day, map[string]value{
			"summary":  text("Backups skip the attachments volume"),
			"priority": prio("Highest"),
		}),
		mk("OPS-4", "Task", statusDone, "user-ada", "user-alan", 8*day, map[string]value{
			"summary": text("Upgrade the database"),
		}),
	}
}
