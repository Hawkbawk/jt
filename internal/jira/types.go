// Package jira adapts jira-cli's client to the data jt displays.
package jira

import "fmt"

// Project is a Jira project.
type Project struct {
	Key  string
	Name string
}

// Issue is the summary of an issue shown on a board card.
type Issue struct {
	Key      string
	Summary  string
	StatusID string
	Status   string
	Assignee string
	// AssigneeID identifies the assignee in JQL: the account ID on cloud, the
	// username on server.
	AssigneeID string
	Type       string
	Priority   string
}

// Column is a board column and the issues whose status maps to it.
type Column struct {
	Name      string
	StatusIDs []string
	Issues    []Issue
}

// Board is a Jira agile board with its columns populated.
type Board struct {
	ID      int
	Name    string
	Type    string
	Project string
	Columns []Column
	// QuickFilters are the board's configured quick filters. Assignees has one
	// filter per person on the loaded issues.
	QuickFilters []Filter
	Assignees    []Filter
}

// Filter narrows a board to the issues matching JQL.
type Filter struct {
	Name       string
	JQL        string
	AssigneeID string
}

// Comment is a single issue comment rendered as markdown.
type Comment struct {
	Author  string
	Created string
	Body    string
}

// IssueDetail is the full view of an issue.
type IssueDetail struct {
	Issue
	Description string
	Reporter    string
	Labels      []string
	Created     string
	Updated     string
	Comments    []Comment
}

// Transition is a workflow transition available for an issue.
type Transition struct {
	ID   string
	Name string
	To   string
}

// FieldKind is how an editable field's value is entered.
type FieldKind int

// The field kinds jt can edit.
const (
	FieldText FieldKind = iota
	FieldTextArea
	FieldNumber
	FieldDate
	FieldDateTime
	FieldSelect
	FieldMultiSelect
	// FieldLabels holds free-form words rather than options.
	FieldLabels
)

// Option is one of a select field's allowed values.
type Option struct {
	ID   string
	Name string
}

// EditField is a field that can be edited on an issue, with its current value.
// Text holds the value of text, number and date kinds. Chosen holds the option
// IDs of select kinds, or the labels of a labels field.
type EditField struct {
	ID       string
	Name     string
	Kind     FieldKind
	Required bool
	Options  []Option
	Text     string
	Chosen   []string
}

// Label describes the transition as a menu entry.
func (t Transition) Label() string {
	if t.To == "" || t.To == t.Name {
		return "Move to " + t.Name
	}
	return fmt.Sprintf("Move to %s (%s)", t.To, t.Name)
}
