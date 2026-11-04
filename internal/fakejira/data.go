package fakejira

import (
	"fmt"
	mathrand "math/rand"
	"reflect"
	"strings"
	"time"

	"github.com/go-faker/faker/v4"
	"github.com/go-faker/faker/v4/pkg/options"
	"github.com/hawkbawk/jt/internal/jira"
)

// DefaultProject is the project the fake opens on start.
const DefaultProject = "DEMO"

type user struct {
	jira.Option
	Email  string `faker:"email"`
	Active bool   `faker:"demo_active"`
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
	Author  jira.Option `faker:"demo_user_option"`
	Body    string      `faker:"paragraph"`
	Created time.Time   `faker:"-"`
}

// value is an editable field's value: Text for text, number and date kinds,
// Chosen for the rest.
type value struct {
	Text   string
	Chosen []jira.Option
}

type issue struct {
	Key      string      `faker:"demo_issue_key"`
	Type     string      `faker:"oneof: Bug, Epic, Story, Task"`
	Status   jira.Option `faker:"demo_status"`
	Assignee jira.Option `faker:"demo_assignee"`
	Reporter jira.Option `faker:"demo_user_option"`
	Created  time.Time   `faker:"demo_created"`
	Updated  time.Time   `faker:"demo_updated"`
	// Values holds the editable fields by ID. Summary, description, priority
	// and labels are read from here too.
	Values   map[string]value `faker:"demo_values"`
	Comments []comment
}

type fakeBackend struct {
	Users    []user         `faker:"demo_users"`
	Projects []jira.Project `faker:"demo_projects"`
	Boards   []board        `faker:"demo_boards"`
	Issues   []*issue       `faker:"slice_len=84"`
}

type fakeValues struct {
	Summary     string        `faker:"sentence"`
	Description string        `faker:"demo_description"`
	Priority    jira.Option   `faker:"demo_priority"`
	Team        jira.Option   `faker:"demo_team"`
	Labels      []jira.Option `faker:"demo_labels"`
	Components  []jira.Option `faker:"demo_components"`
	DueDate     string        `faker:"date"`
	StoryPoints string        `faker:"oneof: 1, 2, 3, 5, 8, 13"`
	Reviewers   []jira.Option `faker:"demo_reviewers"`
}

type fakeGeneration struct {
	rng           *mathrand.Rand
	now           time.Time
	issueCreated  time.Time
	users         []user
	issueSequence int
	userSequence  int
}

var activeGeneration *fakeGeneration

func init() {
	mustAddProvider("demo_active", func(reflect.Value) (any, error) { return true, nil })
	mustAddProvider("demo_user_id", func(reflect.Value) (any, error) {
		activeGeneration.userSequence++
		return fmt.Sprintf("user-%02d", activeGeneration.userSequence), nil
	})
	mustAddProvider("demo_users", func(reflect.Value) (any, error) {
		var users []user
		if err := faker.FakeData(&users,
			options.WithRandomMapAndSliceMinSize(20),
			options.WithRandomMapAndSliceMaxSize(20),
		); err != nil {
			return nil, err
		}
		activeGeneration.users = users
		return users, nil
	})
	mustAddProvider("demo_projects", func(reflect.Value) (any, error) {
		return seedProjects(), nil
	})
	mustAddProvider("demo_boards", func(reflect.Value) (any, error) {
		return seedBoards(), nil
	})
	mustAddProvider("demo_status", func(reflect.Value) (any, error) {
		return allStatuses[activeGeneration.rng.Intn(len(allStatuses))], nil
	})
	mustAddProvider("demo_assignee", func(reflect.Value) (any, error) {
		if activeGeneration.rng.Intn(5) == 0 {
			return jira.Option{}, nil
		}
		return activeGeneration.users[activeGeneration.rng.Intn(len(activeGeneration.users))].Option, nil
	})
	mustAddProvider("demo_user_option", func(reflect.Value) (any, error) {
		return activeGeneration.users[activeGeneration.rng.Intn(len(activeGeneration.users))].Option, nil
	})
	mustAddProvider("demo_issue_key", func(reflect.Value) (any, error) {
		activeGeneration.issueSequence++
		n := activeGeneration.issueSequence
		if n <= 60 {
			return fmt.Sprintf("DEMO-%d", n), nil
		}
		return fmt.Sprintf("OPS-%d", n-60), nil
	})
	mustAddProvider("demo_created", func(reflect.Value) (any, error) {
		activeGeneration.issueCreated = activeGeneration.now.Add(-time.Duration(activeGeneration.rng.Intn(180)+1) * 24 * time.Hour)
		return activeGeneration.issueCreated, nil
	})
	mustAddProvider("demo_updated", func(reflect.Value) (any, error) {
		return activeGeneration.issueCreated.Add(activeGeneration.now.Sub(activeGeneration.issueCreated) / 2), nil
	})
	mustAddProvider("demo_values", func(reflect.Value) (any, error) {
		var generated fakeValues
		if err := faker.FakeData(&generated, options.WithRandomMapAndSliceMinSize(0), options.WithRandomMapAndSliceMaxSize(8)); err != nil {
			return nil, err
		}
		return map[string]value{
			"summary":           {Text: generated.Summary},
			"description":       {Text: generated.Description},
			"priority":          {Chosen: []jira.Option{generated.Priority}},
			"customfield_10030": {Chosen: []jira.Option{generated.Team}},
			"labels":            {Chosen: generated.Labels},
			"components":        {Chosen: generated.Components},
			"duedate":           {Text: generated.DueDate},
			"customfield_10016": {Text: generated.StoryPoints},
			"customfield_10020": {Chosen: generated.Reviewers},
		}, nil
	})
	mustAddProvider("demo_description", func(reflect.Value) (any, error) {
		paragraphs := make([]string, activeGeneration.rng.Intn(9)+1)
		for i := range paragraphs {
			paragraphs[i] = faker.Paragraph()
		}
		return strings.Join(paragraphs, "\n\n"), nil
	})
	mustAddProvider("demo_priority", func(reflect.Value) (any, error) {
		return priorities[activeGeneration.rng.Intn(len(priorities))], nil
	})
	mustAddProvider("demo_team", func(reflect.Value) (any, error) {
		return teams[activeGeneration.rng.Intn(len(teams))], nil
	})
	mustAddProvider("demo_labels", func(reflect.Value) (any, error) {
		labels := []string{"backend", "frontend", "infrastructure", "documentation", "testing", "performance"}
		count := activeGeneration.rng.Intn(4)
		chosen := make([]jira.Option, 0, count)
		for i := 0; i < count; i++ {
			label := labels[activeGeneration.rng.Intn(len(labels))]
			if jira.IndexID(chosen, label) < 0 {
				chosen = append(chosen, jira.Option{ID: label, Name: label})
			}
		}
		return chosen, nil
	})
	mustAddProvider("demo_components", func(reflect.Value) (any, error) {
		if activeGeneration.rng.Intn(2) == 0 {
			return []jira.Option{}, nil
		}
		return []jira.Option{components[activeGeneration.rng.Intn(len(components))]}, nil
	})
	mustAddProvider("demo_reviewers", func(reflect.Value) (any, error) {
		if activeGeneration.rng.Intn(2) == 0 {
			return []jira.Option{}, nil
		}
		return []jira.Option{activeGeneration.users[activeGeneration.rng.Intn(len(activeGeneration.users))].Option}, nil
	})
}

func mustAddProvider(tag string, provider func(reflect.Value) (any, error)) {
	if err := faker.AddProvider(tag, provider); err != nil {
		panic(err)
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
