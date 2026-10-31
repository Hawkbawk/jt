// Package fakejira is an in-memory stand-in for jt's Jira client. It is
// seeded with demo projects, boards and issues and keeps changes (moves,
// assignments, comments and edits) until the process exits, so jt can be
// run and tried out without a Jira site.
package fakejira

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hawkbawk/jt/internal/jira"
)

const (
	displayLayout = "2006-01-02 15:04"
	dateLayout    = "2006-01-02"
	maxUsers      = 20
)

// Backend implements the methods jt's TUI calls on the Jira client.
type Backend struct {
	mu       sync.Mutex
	now      func() time.Time
	users    []user
	projects []jira.Project
	boards   []board
	fields   []jira.EditField
	issues   []*issue
}

// New returns a backend seeded with demo data.
func New() *Backend {
	b := &Backend{
		now:      time.Now,
		users:    seedUsers(),
		projects: seedProjects(),
		boards:   seedBoards(),
		fields:   fieldDefs(),
	}
	b.issues = seedIssues(b.now())
	return b
}

// DefaultProject returns the project to open on start.
func (b *Backend) DefaultProject() string {
	return DefaultProject
}

// Projects lists the projects, sorted by key.
func (b *Backend) Projects() ([]jira.Project, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := slices.Clone(b.projects)
	slices.SortFunc(out, func(x, y jira.Project) int { return strings.Compare(x.Key, y.Key) })
	return out, nil
}

// Board loads the project's board, narrowed by filterJQL when set.
func (b *Backend) Board(project, filterJQL string) (*jira.Board, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	i := slices.IndexFunc(b.boards, func(bd board) bool { return bd.Project == project })
	if i < 0 {
		return nil, fmt.Errorf("no boards found for project %s", project)
	}
	bd := b.boards[i]

	cols := make([]jira.Column, len(bd.Columns))
	byStatus := make(map[string]int)
	for i, c := range bd.Columns {
		cols[i] = jira.Column{Name: c.Name, StatusIDs: slices.Clone(c.StatusIDs)}
		for _, id := range c.StatusIDs {
			byStatus[id] = i
		}
	}
	match := jqlMatcher(filterJQL)
	var loaded []jira.Issue
	for _, iss := range b.issues {
		if iss.project() != project || !match(iss) {
			continue
		}
		card := b.card(iss)
		loaded = append(loaded, card)
		if c, ok := byStatus[iss.StatusID]; ok {
			cols[c].Issues = append(cols[c].Issues, card)
		}
	}
	return &jira.Board{
		ID: bd.ID, Name: bd.Name, Type: bd.Type, Project: project, Columns: cols,
		QuickFilters: slices.Clone(bd.Quick), Assignees: jira.AssigneeFilters(loaded),
	}, nil
}

// Issue returns an issue with its description and comments.
func (b *Backend) Issue(key string) (*jira.IssueDetail, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	iss, err := b.issue(key)
	if err != nil {
		return nil, err
	}
	d := &jira.IssueDetail{
		Issue:       b.card(iss),
		Description: iss.Values["description"].Text,
		Reporter:    b.userName(iss.ReporterID),
		Labels:      slices.Clone(iss.Values["labels"].Chosen),
		Created:     iss.Created.Local().Format(displayLayout),
		Updated:     iss.Updated.Local().Format(displayLayout),
	}
	for _, c := range iss.Comments {
		d.Comments = append(d.Comments, jira.Comment{
			Author:  b.userName(c.AuthorID),
			Created: c.Created.Local().Format(displayLayout),
			Body:    c.Body,
		})
	}
	return d, nil
}

// Transitions lists a move to every status but the issue's current one.
func (b *Backend) Transitions(iss jira.Issue) ([]jira.Transition, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	stored, err := b.issue(iss.Key)
	if err != nil {
		return nil, err
	}
	var out []jira.Transition
	for _, st := range allStatuses {
		if st.ID != stored.StatusID {
			out = append(out, jira.Transition{ID: st.ID, Name: st.Name, To: st.Name})
		}
	}
	return out, nil
}

// Transition moves an issue to the status with the transition's ID.
func (b *Backend) Transition(key, transitionID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	iss, err := b.issue(key)
	if err != nil {
		return err
	}
	if transitionID == iss.StatusID || !slices.ContainsFunc(allStatuses, func(st status) bool { return st.ID == transitionID }) {
		return fmt.Errorf("transition %q is not valid for %s", transitionID, key)
	}
	iss.StatusID = transitionID
	iss.Updated = b.now()
	return nil
}

// Assign sets an issue's assignee. An empty assigneeID unassigns it.
func (b *Backend) Assign(key, assigneeID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	iss, err := b.issue(key)
	if err != nil {
		return err
	}
	if assigneeID != "" {
		if u := b.user(assigneeID); u == nil || !u.Active {
			return fmt.Errorf("user %q cannot be assigned issues", assigneeID)
		}
	}
	iss.AssigneeID = assigneeID
	iss.Updated = b.now()
	return nil
}

// AddComment adds a comment by the signed-in user.
func (b *Backend) AddComment(key, body string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	iss, err := b.issue(key)
	if err != nil {
		return err
	}
	if strings.TrimSpace(body) == "" {
		return errors.New("comment body can not be empty")
	}
	now := b.now()
	iss.Comments = append(iss.Comments, comment{AuthorID: b.users[0].ID, Body: body, Created: now})
	iss.Updated = now
	return nil
}

// EditFields lists the editable fields with the issue's current values.
func (b *Backend) EditFields(iss jira.Issue) ([]jira.EditField, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	stored, err := b.issue(iss.Key)
	if err != nil {
		return nil, err
	}
	out := make([]jira.EditField, 0, len(b.fields))
	for _, def := range b.fields {
		f := def
		f.Options = slices.Clone(def.Options)
		v := stored.Values[def.ID]
		f.Text = v.Text
		for _, id := range v.Chosen {
			option := jira.Option{ID: id, Name: id}
			if def.Kind == jira.FieldUser || def.Kind == jira.FieldMultiUser {
				option.Name = b.userName(id)
				f.Options = append(f.Options, option)
			} else if i := slices.IndexFunc(def.Options, func(o jira.Option) bool { return o.ID == id }); i >= 0 {
				option = def.Options[i]
			}
			f.Chosen = append(f.Chosen, option)
		}
		out = append(out, f)
	}
	return out, nil
}

// Edit sets the given fields. Nothing changes unless every value is valid.
func (b *Backend) Edit(key string, fields []jira.EditField) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	iss, err := b.issue(key)
	if err != nil {
		return err
	}
	updates := make(map[string]value, len(fields))
	for _, f := range fields {
		i := slices.IndexFunc(b.fields, func(def jira.EditField) bool { return def.ID == f.ID })
		if i < 0 {
			return fmt.Errorf("field %q cannot be set", f.ID)
		}
		v, err := b.validate(b.fields[i], f)
		if err != nil {
			return err
		}
		updates[f.ID] = v
	}
	for id, v := range updates {
		iss.Values[id] = v
	}
	iss.Updated = b.now()
	return nil
}

func (b *Backend) validate(def, f jira.EditField) (value, error) {
	text := strings.TrimSpace(f.Text)
	switch def.Kind {
	case jira.FieldText, jira.FieldTextArea:
		if text == "" {
			if def.Required {
				return value{}, fmt.Errorf("%s is required", def.Name)
			}
			return value{}, nil
		}
		return value{Text: f.Text}, nil
	case jira.FieldNumber:
		if text == "" {
			return value{}, nil
		}
		n, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return value{}, fmt.Errorf("%s: %q is not a number", def.Name, f.Text)
		}
		return value{Text: strconv.FormatFloat(n, 'f', -1, 64)}, nil
	case jira.FieldDate:
		if text == "" {
			return value{}, nil
		}
		if _, err := time.Parse(dateLayout, text); err != nil {
			return value{}, fmt.Errorf("%s: %q is not a date like 2006-01-02", def.Name, f.Text)
		}
		return value{Text: text}, nil
	case jira.FieldSelect, jira.FieldMultiSelect:
		if def.Kind == jira.FieldSelect && len(f.Chosen) > 1 {
			return value{}, fmt.Errorf("%s takes one value", def.Name)
		}
		for _, option := range f.Chosen {
			if !slices.ContainsFunc(def.Options, func(o jira.Option) bool { return o.ID == option.ID }) {
				return value{}, fmt.Errorf("specify a valid value for %s", def.Name)
			}
		}
	case jira.FieldUser, jira.FieldMultiUser:
		if def.Kind == jira.FieldUser && len(f.Chosen) > 1 {
			return value{}, fmt.Errorf("%s takes one person", def.Name)
		}
		for _, option := range f.Chosen {
			if b.user(option.ID) == nil {
				return value{}, fmt.Errorf("user %q does not exist", option.ID)
			}
		}
	}
	chosen := make([]string, len(f.Chosen))
	for i, option := range f.Chosen {
		chosen[i] = option.ID
	}
	return value{Chosen: chosen}, nil
}

// SearchUsers finds active people whose name or email contain query.
func (b *Backend) SearchUsers(_, query string) ([]jira.Option, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	query = strings.ToLower(query)
	var out []jira.Option
	for _, u := range b.users {
		if len(out) == maxUsers {
			break
		}
		if u.Active && (strings.Contains(strings.ToLower(u.Name), query) || strings.Contains(strings.ToLower(u.Email), query)) {
			out = append(out, jira.Option{ID: u.ID, Name: u.Name})
		}
	}
	return out, nil
}

// IssueURL returns a placeholder browser URL for an issue.
func (b *Backend) IssueURL(key string) string {
	return "https://jira.example.com/browse/" + url.PathEscape(key)
}

func (b *Backend) issue(key string) (*issue, error) {
	for _, iss := range b.issues {
		if strings.EqualFold(iss.Key, key) {
			return iss, nil
		}
	}
	return nil, fmt.Errorf("issue %s does not exist", key)
}

func (b *Backend) user(id string) *user {
	i := slices.IndexFunc(b.users, func(u user) bool { return u.ID == id })
	if i < 0 {
		return nil
	}
	return &b.users[i]
}

func (b *Backend) userName(id string) string {
	if u := b.user(id); u != nil {
		return u.Name
	}
	return ""
}

func (b *Backend) card(iss *issue) jira.Issue {
	out := jira.Issue{
		Key:      iss.Key,
		Summary:  iss.Values["summary"].Text,
		StatusID: iss.StatusID,
		Assignee: jira.Option{
			ID:   iss.AssigneeID,
			Name: b.userName(iss.AssigneeID),
		},
		Type: iss.Type,
	}
	if i := slices.IndexFunc(allStatuses, func(st status) bool { return st.ID == iss.StatusID }); i >= 0 {
		out.Status = allStatuses[i].Name
	}
	if p := iss.Values["priority"].Chosen; len(p) > 0 {
		if i := slices.IndexFunc(priorities, func(o jira.Option) bool { return o.ID == p[0] }); i >= 0 {
			out.Priority = priorities[i].Name
		}
	}
	return out
}

var (
	jqlAssigneeRe = regexp.MustCompile(`(?i)\bassignee\s*=\s*"?([^"\s)]+)"?`)
	jqlEmptyRe    = regexp.MustCompile(`(?i)\bassignee\s+is\s+EMPTY\b`)
	jqlTypeRe     = regexp.MustCompile(`(?i)\bissuetype\s*=\s*"?([^"\s)]+)"?`)
	jqlLabelRe    = regexp.MustCompile(`(?i)\blabels\s*=\s*"?([^"\s)]+)"?`)
)

// jqlMatcher understands just enough JQL for jt's filters: "assignee = x",
// "assignee is EMPTY", "issuetype = x" and "labels = x", all ANDed together.
// Anything else is ignored.
func jqlMatcher(jql string) func(*issue) bool {
	var conds []func(*issue) bool
	for _, m := range jqlAssigneeRe.FindAllStringSubmatch(jql, -1) {
		id := m[1]
		conds = append(conds, func(i *issue) bool { return i.AssigneeID == id })
	}
	if jqlEmptyRe.MatchString(jql) {
		conds = append(conds, func(i *issue) bool { return i.AssigneeID == "" })
	}
	for _, m := range jqlTypeRe.FindAllStringSubmatch(jql, -1) {
		typ := m[1]
		conds = append(conds, func(i *issue) bool { return strings.EqualFold(i.Type, typ) })
	}
	for _, m := range jqlLabelRe.FindAllStringSubmatch(jql, -1) {
		label := m[1]
		conds = append(conds, func(i *issue) bool { return slices.Contains(i.Values["labels"].Chosen, label) })
	}
	return func(i *issue) bool {
		for _, c := range conds {
			if !c(i) {
				return false
			}
		}
		return true
	}
}
