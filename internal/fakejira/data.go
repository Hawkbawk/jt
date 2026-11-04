package fakejira

import (
	"strings"
	"time"

	"github.com/hawkbawk/jt/internal/jira"
)

// DefaultProject is the project the fake opens on start.
const DefaultProject = "DEMO"

type user struct {
	jira.Option `fake:"{jirauser}"`
	Email       string `fake:"{email}"`
	Active      bool   `fake:"{chance:90}"`
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
	Edited  bool
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
