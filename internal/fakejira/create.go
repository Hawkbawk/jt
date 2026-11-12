package fakejira

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/hawkbawk/jt/internal/jira"
)

var (
	typeTask     = jira.IssueType{ID: "10001", Name: "Task"}
	typeBug      = jira.IssueType{ID: "10002", Name: "Bug"}
	typeStory    = jira.IssueType{ID: "10003", Name: "Story"}
	typeIncident = jira.IssueType{ID: "10004", Name: "Incident"}
)

// createTypes are the issue types each project can create. Other projects can
// create the same as DEMO.
var createTypes = map[string][]jira.IssueType{
	"DEMO":  {typeTask, typeBug, typeStory},
	"OPS":   {typeIncident, typeTask},
	"EMPTY": nil,
}

// stepsField is only on bugs, to show that fields depend on the issue type.
var stepsField = jira.IssueField{ID: "customfield_10050", Name: "Steps to reproduce", Kind: jira.FieldTextArea, Required: true}

// CreateIssueTypes lists the issue types a project can create.
func (b *Backend) CreateIssueTypes(project string) ([]jira.IssueType, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.hasProject(project) {
		return nil, fmt.Errorf("project %q does not exist", project)
	}
	types, ok := createTypes[project]
	if !ok {
		types = createTypes["DEMO"]
	}
	return slices.Clone(types), nil
}

// CreateFields lists the fields an issue of a type can be created with.
func (b *Backend) CreateFields(project, issueTypeID string) ([]jira.IssueField, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, err := b.createType(project, issueTypeID); err != nil {
		return nil, err
	}
	return b.createDefs(issueTypeID), nil
}

func (b *Backend) createDefs(issueTypeID string) []jira.IssueField {
	var defs []jira.IssueField
	for _, d := range b.fields {
		d.Options = slices.Clone(d.Options)
		defs = append(defs, d)
		if d.ID == "priority" {
			defs = append(defs, jira.IssueField{ID: "assignee", Name: "Assignee", Kind: jira.FieldUser})
		}
	}
	if issueTypeID == typeBug.ID {
		defs = append(defs, stepsField)
	}
	return defs
}

func (b *Backend) createType(project, issueTypeID string) (jira.IssueType, error) {
	types, ok := createTypes[project]
	if !ok {
		types = createTypes["DEMO"]
	}
	i := slices.IndexFunc(types, func(t jira.IssueType) bool { return t.ID == issueTypeID })
	if !b.hasProject(project) || i < 0 {
		return jira.IssueType{}, fmt.Errorf("%s can't create issue type %q", project, issueTypeID)
	}
	return types[i], nil
}

func (b *Backend) hasProject(key string) bool {
	return slices.ContainsFunc(b.projects, func(p jira.Project) bool { return p.Key == key })
}

// Create adds an issue. Nothing is added unless every value is valid.
func (b *Backend) Create(project, issueTypeID string, fields []jira.IssueField) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	t, err := b.createType(project, issueTypeID)
	if err != nil {
		return "", err
	}
	defs := b.createDefs(issueTypeID)
	values := make(map[string]value, len(fields))
	for _, f := range fields {
		i := slices.IndexFunc(defs, func(d jira.IssueField) bool { return d.ID == f.ID })
		if i < 0 {
			return "", fmt.Errorf("field %q cannot be set", f.ID)
		}
		v, err := b.validate(defs[i], f)
		if err != nil {
			return "", err
		}
		values[f.ID] = v
	}
	for _, d := range defs {
		if d.Required && strings.TrimSpace(values[d.ID].Text) == "" && len(values[d.ID].Chosen) == 0 {
			return "", errors.New(d.Name + " is required")
		}
	}

	next := 1
	for _, iss := range b.issues {
		if iss.project() != project {
			continue
		}
		_, n, _ := strings.Cut(iss.Key, "-")
		if i, err := strconv.Atoi(n); err == nil {
			next = max(next, i+1)
		}
	}
	now := b.now()
	iss := &issue{
		Key: fmt.Sprintf("%s-%d", project, next), Type: t.Name, Status: statusToDo,
		Reporter: b.activeUser().Option, Created: now, Updated: now, Values: values,
	}
	if a := values["assignee"].Chosen; len(a) > 0 {
		iss.Assignee = a[0]
	}
	delete(values, "assignee")
	b.issues = append(b.issues, iss)
	return iss.Key, nil
}

// SearchProjectUsers finds active people whose name or email contain query.
func (b *Backend) SearchProjectUsers(_, query string) ([]jira.Option, error) {
	return b.SearchUsers("", query)
}
