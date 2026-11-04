package fakejira

import (
	"fmt"
	mathrand "math/rand"
	"reflect"
	"strings"
	"time"

	"github.com/go-faker/faker/v4"
	"github.com/hawkbawk/jt/internal/jira"
)

// DefaultProject is the project the fake opens on start.
const DefaultProject = "DEMO"

type user struct {
	jira.Option
	Email  string
	Active bool
}

type fakePerson struct {
	Name  string `faker:"name"`
	Email string `faker:"email"`
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
	Author  jira.Option `faker:"demo_comment_author"`
	Body    string      `faker:"demo_comment_body"`
	Created time.Time   `faker:"-"`
}

type fakeIssue struct {
	Summary     string        `faker:"sentence"`
	Description string        `faker:"demo_description"`
	Type        string        `faker:"demo_issue_type"`
	Status      jira.Option   `faker:"demo_status"`
	Assignee    jira.Option   `faker:"demo_assignee"`
	Reporter    jira.Option   `faker:"demo_reporter"`
	Priority    jira.Option   `faker:"demo_priority"`
	Team        jira.Option   `faker:"demo_team"`
	Labels      []jira.Option `faker:"demo_labels"`
	Components  []jira.Option `faker:"demo_components"`
	DueDate     string        `faker:"demo_due_date"`
	StoryPoints string        `faker:"demo_story_points"`
	Reviewers   []jira.Option `faker:"demo_reviewers"`
	Comments    []comment     `faker:"demo_comments"`
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

type fakeGeneration struct {
	rng          *mathrand.Rand
	now          time.Time
	issueCreated time.Time
	users        []user
}

var activeGeneration *fakeGeneration

func init() {
	providers := map[string]func(reflect.Value) (any, error){
		"demo_description": fieldProvider(func(g *fakeGeneration) any {
			switch g.rng.Intn(5) {
			case 0:
				return faker.Sentence()
			case 1, 2, 3:
				return faker.Paragraph()
			default:
				return strings.Join([]string{
					faker.Paragraph(),
					faker.Paragraph(),
					faker.Paragraph(),
					faker.Paragraph(),
				}, "\n\n")
			}
		}),
		"demo_issue_type": fieldProvider(func(g *fakeGeneration) any {
			return []string{"Bug", "Epic", "Story", "Task"}[g.rng.Intn(4)]
		}),
		"demo_status": fieldProvider(func(g *fakeGeneration) any {
			return allStatuses[g.rng.Intn(len(allStatuses))]
		}),
		"demo_assignee": fieldProvider(func(g *fakeGeneration) any {
			if g.rng.Intn(5) == 0 {
				return jira.Option{}
			}
			return g.users[g.rng.Intn(len(g.users)-1)].Option
		}),
		"demo_reporter": fieldProvider(func(g *fakeGeneration) any {
			return g.users[g.rng.Intn(len(g.users)-1)].Option
		}),
		"demo_priority": fieldProvider(func(g *fakeGeneration) any {
			return priorities[g.rng.Intn(len(priorities))]
		}),
		"demo_team": fieldProvider(func(g *fakeGeneration) any {
			return teams[g.rng.Intn(len(teams))]
		}),
		"demo_labels": fieldProvider(func(g *fakeGeneration) any {
			labels := []string{"backend", "frontend", "infrastructure", "documentation", "testing", "performance"}
			var chosen []jira.Option
			count := g.rng.Intn(4)
			for i := 0; i < count; i++ {
				label := labels[g.rng.Intn(len(labels))]
				if jira.IndexID(chosen, label) < 0 {
					chosen = append(chosen, jira.Option{ID: label, Name: label})
				}
			}
			return chosen
		}),
		"demo_components": fieldProvider(func(g *fakeGeneration) any {
			if g.rng.Intn(2) == 0 {
				return []jira.Option{}
			}
			return []jira.Option{components[g.rng.Intn(len(components))]}
		}),
		"demo_due_date": fieldProvider(func(g *fakeGeneration) any {
			if g.rng.Intn(3) != 0 {
				return ""
			}
			return g.now.AddDate(0, 0, g.rng.Intn(60)+1).Format("2006-01-02")
		}),
		"demo_story_points": fieldProvider(func(g *fakeGeneration) any {
			if g.rng.Intn(2) == 0 {
				return ""
			}
			return fmt.Sprint([]int{1, 2, 3, 5, 8, 13}[g.rng.Intn(6)])
		}),
		"demo_reviewers": fieldProvider(func(g *fakeGeneration) any {
			if g.rng.Intn(2) == 0 {
				return []jira.Option{}
			}
			return []jira.Option{g.users[g.rng.Intn(len(g.users)-1)].Option}
		}),
		"demo_comments": func(reflect.Value) (any, error) {
			if activeGeneration == nil {
				return nil, fmt.Errorf("fake data generator is not active")
			}
			g := activeGeneration
			count := g.rng.Intn(9)
			comments := make([]comment, 0, count)
			for i := 0; i < count; i++ {
				var c comment
				if err := faker.FakeData(&c); err != nil {
					return nil, err
				}
				c.Created = g.issueCreated.Add(time.Duration(i+1) * g.now.Sub(g.issueCreated) / time.Duration(count+1))
				comments = append(comments, c)
			}
			return comments, nil
		},
		"demo_comment_author": fieldProvider(func(g *fakeGeneration) any {
			return g.users[g.rng.Intn(len(g.users)-1)].Option
		}),
		"demo_comment_body": fieldProvider(func(g *fakeGeneration) any {
			switch g.rng.Intn(3) {
			case 0:
				return faker.Sentence()
			case 1:
				return faker.Paragraph()
			default:
				return strings.Join([]string{faker.Paragraph(), faker.Paragraph(), faker.Paragraph()}, "\n\n")
			}
		}),
	}
	for tag, provider := range providers {
		if err := faker.AddProvider(tag, provider); err != nil {
			panic(err)
		}
	}
}

func fieldProvider(generate func(*fakeGeneration) any) func(reflect.Value) (any, error) {
	return func(reflect.Value) (any, error) {
		if activeGeneration == nil {
			return nil, fmt.Errorf("fake data generator is not active")
		}
		return generate(activeGeneration), nil
	}
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
	users := []user{
		{Option: jira.Option{ID: "user-sam", Name: "Sam Tester"}, Email: "sam@example.com", Active: true},
		{Option: jira.Option{ID: "user-ada", Name: "Ada Lovelace"}, Email: "ada@example.com", Active: true},
		{Option: jira.Option{ID: "user-grace", Name: "Grace Hopper"}, Email: "grace@example.com", Active: true},
		{Option: jira.Option{ID: "user-alan", Name: "Alan Turing"}, Email: "alan@example.com", Active: true},
	}
	for i := 4; i < maxUsers-1; i++ {
		var person fakePerson
		if err := faker.FakeData(&person); err != nil {
			panic(err)
		}
		users = append(users, user{
			Option: jira.Option{ID: fmt.Sprintf("user-%02d", i), Name: person.Name},
			Email:  person.Email,
			Active: true,
		})
	}
	return append(users, user{
		Option: jira.Option{ID: "user-old", Name: "Former Employee"},
		Email:  "old@example.com",
	})
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
			ID: 3, Name: "Demo team board", Type: "scrum", Project: "DEMO",
			Columns: []jira.Column{
				{Name: "Open", StatusIDs: []string{statusToDo.ID}},
				{Name: "Active", StatusIDs: []string{statusInProgress.ID, statusInReview.ID}},
				{Name: "Closed", StatusIDs: []string{statusDone.ID}},
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

func seedIssues(now time.Time, users []user, rng *mathrand.Rand) []*issue {
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

	issues := []*issue{big, bug}

	for _, project := range []struct {
		key   string
		first int
		last  int
	}{
		{key: "DEMO", first: 3, last: 60},
		{key: "OPS", first: 1, last: 24},
	} {
		for n := project.first; n <= project.last; n++ {
			age := time.Duration(rng.Intn(180)+1)*day + time.Duration(rng.Intn(24))*time.Hour
			created := now.Add(-age)
			activeGeneration.issueCreated = created
			var generated fakeIssue
			if err := faker.FakeData(&generated); err != nil {
				panic(err)
			}
			iss := mk(fmt.Sprintf("%s-%d", project.key, n), generated.Type, generated.Status,
				generated.Assignee.ID, generated.Reporter.ID, age, generatedValues(generated))
			iss.Comments = generated.Comments
			issues = append(issues, iss)
		}
	}
	return issues
}

func generatedValues(generated fakeIssue) map[string]value {
	values := map[string]value{
		"summary":           {Text: generated.Summary},
		"priority":          {Chosen: []jira.Option{generated.Priority}},
		"customfield_10030": {Chosen: []jira.Option{generated.Team}},
		"labels":            {Chosen: generated.Labels},
		"components":        {Chosen: generated.Components},
	}
	if generated.Description != "" {
		values["description"] = value{Text: generated.Description}
	}
	if generated.DueDate != "" {
		values["duedate"] = value{Text: generated.DueDate}
	}
	if generated.StoryPoints != "" {
		values["customfield_10016"] = value{Text: generated.StoryPoints}
	}
	if len(generated.Reviewers) > 0 {
		values["customfield_10020"] = value{Chosen: generated.Reviewers}
	}
	return values
}
