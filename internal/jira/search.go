package jira

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// SprintFilter limits a search by whether issues are in an active sprint.
type SprintFilter int

// The sprint filters.
const (
	SprintAny SprintFilter = iota
	SprintActive
	SprintNotActive
)

// SortField is a field a search can be ordered by. Its value is the JQL name.
type SortField string

// The fields searches can be sorted by.
const (
	SortCreated  SortField = "created"
	SortUpdated  SortField = "updated"
	SortPriority SortField = "priority"
	SortStatus   SortField = "status"
	SortKey      SortField = "key"
	SortSummary  SortField = "summary"
)

// SortFields lists the sort fields in the order they're offered.
var SortFields = []SortField{SortCreated, SortUpdated, SortPriority, SortStatus, SortKey, SortSummary}

// String is the field's name as shown to people.
func (f SortField) String() string {
	s := string(f)
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// TimeRange bounds a date. A zero end is open.
type TimeRange struct {
	From, To time.Time
}

// SearchQuery is the basic search form's criteria. Zero values mean "any".
type SearchQuery struct {
	// Text is matched against the issues' text fields.
	Text      string
	Project   string
	IssueType string
	Sprint    SprintFilter
	// Keys are issue keys, any of which matches.
	Keys    []string
	Created TimeRange
	Updated TimeRange
	// Sort defaults to the issue key's order when empty.
	Sort       SortField
	Descending bool
}

var (
	keyRe    = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*-\d+$`)
	numberRe = regexp.MustCompile(`^\d+$`)
)

// ParseSearchText reads what was typed in the search box. A number is that
// issue in project, an issue key is that issue, and anything else is text to
// look for.
func ParseSearchText(s, project string) (key, text string, err error) {
	s = strings.TrimSpace(s)
	switch {
	case numberRe.MatchString(s):
		if project == "" {
			return "", "", errors.New("set a default project to find an issue by its number")
		}
		return strings.ToUpper(project) + "-" + s, "", nil
	case keyRe.MatchString(s):
		return strings.ToUpper(s), "", nil
	}
	return "", s, nil
}

// ParseKeys reads a comma or space separated list of issue keys.
func ParseKeys(s string) ([]string, error) {
	var keys []string
	for _, k := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' }) {
		if !keyRe.MatchString(k) {
			return nil, fmt.Errorf("%q isn't an issue key like ABC-123", k)
		}
		keys = append(keys, strings.ToUpper(k))
	}
	return keys, nil
}

// String renders the query as JQL.
func (q SearchQuery) String() string {
	var conds []string
	if q.Text != "" {
		conds = append(conds, "textfields ~ "+quoteJQL(q.Text))
	}
	if q.Project != "" {
		conds = append(conds, "project = "+quoteJQL(q.Project))
	}
	if q.IssueType != "" {
		conds = append(conds, "issuetype = "+quoteJQL(q.IssueType))
	}
	switch q.Sprint {
	case SprintActive:
		conds = append(conds, "sprint in openSprints()")
	case SprintNotActive:
		conds = append(conds, "sprint not in openSprints()")
	}
	switch len(q.Keys) {
	case 0:
	case 1:
		conds = append(conds, "key = "+q.Keys[0])
	default:
		conds = append(conds, "key in ("+strings.Join(q.Keys, ", ")+")")
	}
	conds = append(conds, q.Created.conds("created")...)
	conds = append(conds, q.Updated.conds("updated")...)

	jql := strings.Join(conds, " AND ")
	if q.Sort == "" {
		return jql
	}
	order := "ASC"
	if q.Descending {
		order = "DESC"
	}
	return strings.TrimSpace(jql + " ORDER BY " + string(q.Sort) + " " + order)
}

func (r TimeRange) conds(field string) []string {
	const layout = "2006-01-02 15:04"
	var out []string
	if !r.From.IsZero() {
		out = append(out, fmt.Sprintf("%s >= %q", field, r.From.Format(layout)))
	}
	if !r.To.IsZero() {
		out = append(out, fmt.Sprintf("%s <= %q", field, r.To.Format(layout)))
	}
	return out
}

func quoteJQL(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}
