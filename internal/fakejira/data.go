package fakejira

import (
	"fmt"
	"strings"
	"time"
)

// Login is the email of the signed-in user. Any login and token are accepted,
// but /myself always reports this user.
const Login = "sam@example.com"

// DefaultProject is the project with the most seeded data.
const DefaultProject = "DEMO"

type user struct {
	AccountID string
	Name      string
	Email     string
	Active    bool
}

type project struct {
	Key, Name string
}

type status struct {
	ID, Name string
}

type column struct {
	Name     string
	Statuses []string
}

type quickFilter struct {
	Name, JQL string
}

type board struct {
	ID      int
	Name    string
	Type    string
	Project string
	Columns []column
	Quick   []quickFilter
}

type comment struct {
	ID       string
	AuthorID string
	Body     string
	Created  time.Time
}

type issue struct {
	Key        string
	Type       string
	StatusID   string
	AssigneeID string
	ReporterID string
	Created    time.Time
	Updated    time.Time
	// Fields holds the editable fields by ID, in the shape Jira returns them.
	// Rich text is Jira wiki markup.
	Fields   map[string]any
	Comments []comment
}

func (i *issue) project() string {
	p, _, _ := strings.Cut(i.Key, "-")
	return p
}

func (i *issue) labels() []any {
	labels, _ := i.Fields["labels"].([]any)
	if labels == nil {
		return []any{}
	}
	return labels
}

// fieldDef describes an editable field, as returned in editmeta.
type fieldDef struct {
	ID       string
	Name     string
	Required bool
	Type     string
	Items    string
	Custom   string
	Allowed  []allowedValue
}

type allowedValue struct {
	ID   string
	Name string
	// Option marks custom field options, which Jira names with "value".
	Option bool
}

func (a allowedValue) json() map[string]any {
	if a.Option {
		return map[string]any{"id": a.ID, "value": a.Name}
	}
	return map[string]any{"id": a.ID, "name": a.Name}
}

var (
	statusToDo       = status{"10000", "To Do"}
	statusInProgress = status{"3", "In Progress"}
	statusInReview   = status{"10001", "In Review"}
	statusDone       = status{"10002", "Done"}

	allStatuses = []status{statusToDo, statusInProgress, statusInReview, statusDone}
)

var priorities = []allowedValue{
	{ID: "1", Name: "Highest"},
	{ID: "2", Name: "High"},
	{ID: "3", Name: "Medium"},
	{ID: "4", Name: "Low"},
	{ID: "5", Name: "Lowest"},
}

func fieldDefs() []fieldDef {
	const custom = "com.atlassian.jira.plugin.system.customfieldtypes:"
	return []fieldDef{
		{ID: "summary", Name: "Summary", Required: true, Type: "string"},
		{ID: "description", Name: "Description", Type: "string"},
		{ID: "priority", Name: "Priority", Type: "priority", Allowed: priorities},
		{ID: "labels", Name: "Labels", Type: "array", Items: "string"},
		{ID: "duedate", Name: "Due date", Type: "date"},
		{ID: "components", Name: "Components", Type: "array", Items: "component", Allowed: []allowedValue{
			{ID: "100", Name: "API"},
			{ID: "101", Name: "Frontend"},
			{ID: "102", Name: "Infrastructure"},
		}},
		{ID: "customfield_10016", Name: "Story points", Type: "number", Custom: custom + "float"},
		{ID: "customfield_10030", Name: "Team", Type: "option", Custom: custom + "select", Allowed: []allowedValue{
			{ID: "200", Name: "Platform", Option: true},
			{ID: "201", Name: "Growth", Option: true},
		}},
		{ID: "customfield_10020", Name: "Reviewers", Type: "array", Items: "user", Custom: custom + "multiuserpicker"},
		{ID: "customfield_10040", Name: "Design doc", Type: "string", Custom: custom + "url"},
	}
}

func seedUsers() []user {
	return []user{
		{AccountID: "user-sam", Name: "Sam Tester", Email: Login, Active: true},
		{AccountID: "user-ada", Name: "Ada Lovelace", Email: "ada@example.com", Active: true},
		{AccountID: "user-grace", Name: "Grace Hopper", Email: "grace@example.com", Active: true},
		{AccountID: "user-alan", Name: "Alan Turing", Email: "alan@example.com", Active: true},
		{AccountID: "user-old", Name: "Former Employee", Email: "old@example.com", Active: false},
	}
}

func seedProjects() []project {
	return []project{
		{Key: "DEMO", Name: "Demo"},
		{Key: "EMPTY", Name: "Project without boards"},
		{Key: "OPS", Name: "Operations"},
	}
}

func seedBoards() []board {
	return []board{
		{
			ID: 1, Name: "Demo board", Type: "kanban", Project: "DEMO",
			Columns: []column{
				{Name: "To Do", Statuses: []string{statusToDo.ID}},
				{Name: "In Progress", Statuses: []string{statusInProgress.ID}},
				{Name: "Review", Statuses: []string{statusInReview.ID}},
				{Name: "Done", Statuses: []string{statusDone.ID}},
			},
			Quick: []quickFilter{
				{Name: "Bugs", JQL: "issuetype = Bug"},
				{Name: "Backend", JQL: "labels = backend"},
			},
		},
		{
			ID: 2, Name: "Ops sprint board", Type: "scrum", Project: "OPS",
			Columns: []column{
				{Name: "Backlog", Statuses: []string{statusToDo.ID}},
				{Name: "Doing", Statuses: []string{statusInProgress.ID, statusInReview.ID}},
				{Name: "Done", Statuses: []string{statusDone.ID}},
			},
		},
	}
}

const longDescription = `h2. Background

The board takes a long time to load for projects with thousands of issues. This ticket tracks paging through the results and showing a spinner while the board loads.

h2. Acceptance criteria

* Boards with more than 100 issues load every page
* A spinner shows while the board loads
* Errors from Jira show in the status line

h2. Notes

{code:go}
for start := 0; start < maxIssues; start += pageSize {
	// fetch a page
}
{code}

h3. Steps to reproduce

# Open a project with a large board
# Wait
# Keep waiting

h3. Filler for scrolling

`

func seedIssues(now time.Time) []*issue {
	const day = 24 * time.Hour
	var filler strings.Builder
	for i := 1; i <= 40; i++ {
		fmt.Fprintf(&filler, "Line %d of filler text so the issue view has something to scroll through.\n\n", i)
	}
	filler.WriteString("The end of the description.")

	mk := func(key, typ string, st status, assignee, reporter string, age time.Duration, fields map[string]any) *issue {
		created := now.Add(-age)
		return &issue{
			Key: key, Type: typ, StatusID: st.ID, AssigneeID: assignee, ReporterID: reporter,
			Created: created, Updated: created.Add(age / 2), Fields: fields,
		}
	}
	prio := func(name string) map[string]any {
		for _, p := range priorities {
			if p.Name == name {
				return p.json()
			}
		}
		return nil
	}

	big := mk("DEMO-1", "Story", statusInProgress, "user-sam", "user-ada", 12*day, map[string]any{
		"summary":           "Load large boards page by page",
		"description":       longDescription + filler.String(),
		"priority":          prio("High"),
		"labels":            []any{"backend", "performance"},
		"components":        []any{map[string]any{"id": "100", "name": "API"}},
		"duedate":           now.Add(7 * day).Format("2006-01-02"),
		"customfield_10016": 5.0,
		"customfield_10030": map[string]any{"id": "200", "value": "Platform"},
		"customfield_10020": []any{map[string]any{"accountId": "user-grace", "displayName": "Grace Hopper", "active": true}},
		"customfield_10040": "https://example.com/design/boards",
	})
	authors := []string{"user-ada", "user-grace", "user-sam", "user-alan"}
	for i := range 12 {
		big.Comments = append(big.Comments, comment{
			ID:       fmt.Sprint(1000 + i),
			AuthorID: authors[i%len(authors)],
			Body:     fmt.Sprintf("Update %d: still working through the paging edge cases.\n\nNext up is error handling.", i+1),
			Created:  big.Created.Add(time.Duration(i+1) * 12 * time.Hour),
		})
	}

	bug := mk("DEMO-2", "Bug", statusToDo, "user-ada", "user-sam", 3*day, map[string]any{
		"summary":     "Crash when a column has no statuses",
		"description": "Opening a board whose column has no statuses panics.\n\nSee the trace in the comments.",
		"priority":    prio("Highest"),
		"labels":      []any{"backend"},
	})
	bug.Comments = []comment{{ID: "2000", AuthorID: "user-sam", Body: "I can reproduce this on main.", Created: bug.Created.Add(time.Hour)}}

	return []*issue{
		big,
		bug,
		mk("DEMO-3", "Task", statusToDo, "", "user-grace", 2*day, map[string]any{
			"summary":  "Write release notes for 0.2",
			"priority": prio("Low"),
		}),
		mk("DEMO-4", "Story", statusToDo, "user-grace", "user-ada", 5*day, map[string]any{
			"summary":     "Filter the board by assignee",
			"description": "Add a filter menu listing everyone with an issue on the board.",
			"priority":    prio("Medium"),
			"labels":      []any{"frontend"},
		}),
		mk("DEMO-5", "Bug", statusInProgress, "user-alan", "user-sam", 6*day, map[string]any{
			"summary":     "Comments with tables render badly",
			"description": "Tables in comments lose their borders.",
			"priority":    prio("Medium"),
		}),
		mk("DEMO-6", "Story", statusInReview, "user-sam", "user-grace", 9*day, map[string]any{
			"summary":     "Copy the issue URL to the clipboard",
			"description": "Pressing y copies the issue's browser URL.",
			"priority":    prio("Low"),
			"labels":      []any{"frontend"},
		}),
		mk("DEMO-7", "Task", statusInReview, "", "user-ada", 4*day, map[string]any{
			"summary":  "Bump dependencies",
			"priority": prio("Lowest"),
		}),
		mk("DEMO-8", "Story", statusDone, "user-ada", "user-sam", 20*day, map[string]any{
			"summary":     "Pick a theme from bubbletint",
			"description": "Press t to choose a theme.",
			"priority":    prio("Medium"),
		}),
		mk("DEMO-9", "Bug", statusDone, "user-grace", "user-alan", 15*day, map[string]any{
			"summary":  "Status line overflows on narrow terminals",
			"priority": prio("High"),
			"labels":   []any{"frontend"},
		}),
		mk("OPS-1", "Task", statusToDo, "user-alan", "user-alan", day, map[string]any{
			"summary":  "Rotate the staging certificates",
			"priority": prio("High"),
		}),
		mk("OPS-2", "Task", statusInProgress, "user-sam", "user-alan", 2*day, map[string]any{
			"summary":     "Move CI runners to the new cluster",
			"description": "Runners should autoscale.",
			"priority":    prio("Medium"),
		}),
		mk("OPS-3", "Bug", statusInReview, "", "user-grace", 3*day, map[string]any{
			"summary":  "Backups skip the attachments volume",
			"priority": prio("Highest"),
		}),
		mk("OPS-4", "Task", statusDone, "user-ada", "user-alan", 8*day, map[string]any{
			"summary": "Upgrade the database",
		}),
	}
}
