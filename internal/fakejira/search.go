package fakejira

import (
	"cmp"
	"context"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/hawkbawk/jt/internal/jira"
)

var (
	jqlProjectRe = regexp.MustCompile(`(?i)\bproject\s*=\s*"([^"]+)"`)
	jqlKeyRe     = regexp.MustCompile(`(?i)\bkey\s*=\s*([A-Za-z][A-Za-z0-9_]*-\d+)`)
	jqlKeysRe    = regexp.MustCompile(`(?i)\bkey\s+in\s*\(([^)]*)\)`)
	jqlTimeRe    = regexp.MustCompile(`(?i)\b(created|updated)\s*(>=|<=)\s*"([^"]+)"`)
	jqlOrderRe   = regexp.MustCompile(`(?i)\border\s+by\s+(\w+)(?:\s+(asc|desc))?`)
)

// Search finds issues with the JQL jt's search form writes, plus what the
// board filters use. Sprint conditions are ignored since the demo issues have
// none.
func (b *Backend) Search(ctx context.Context, jql string) ([]jira.Issue, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	match := searchMatcher(jql)
	var found []*issue
	for _, iss := range b.issues {
		if match(iss) {
			found = append(found, iss)
		}
	}
	if m := jqlOrderRe.FindStringSubmatch(jql); m != nil {
		sortIssues(found, strings.ToLower(m[1]), strings.EqualFold(m[2], "desc"))
	}
	out := make([]jira.Issue, len(found))
	for i, iss := range found {
		out[i] = b.card(iss)
	}
	return out, nil
}

func searchMatcher(jql string) func(*issue) bool {
	conds := []func(*issue) bool{jqlMatcher(jql)}
	if m := jqlProjectRe.FindStringSubmatch(jql); m != nil {
		conds = append(conds, func(i *issue) bool { return i.project() == m[1] })
	}
	keys := map[string]bool{}
	for _, m := range jqlKeyRe.FindAllStringSubmatch(jql, -1) {
		keys[strings.ToUpper(m[1])] = true
	}
	for _, m := range jqlKeysRe.FindAllStringSubmatch(jql, -1) {
		for _, k := range strings.Split(m[1], ",") {
			keys[strings.ToUpper(strings.TrimSpace(k))] = true
		}
	}
	if len(keys) > 0 {
		conds = append(conds, func(i *issue) bool { return keys[i.Key] })
	}
	for _, m := range jqlTimeRe.FindAllStringSubmatch(jql, -1) {
		t, err := time.ParseInLocation("2006-01-02 15:04", m[3], time.Local)
		if err != nil {
			continue
		}
		field, op := strings.ToLower(m[1]), m[2]
		conds = append(conds, func(i *issue) bool {
			v := i.Created
			if field == "updated" {
				v = i.Updated
			}
			if op == ">=" {
				return !v.Before(t)
			}
			return !v.After(t)
		})
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

func sortIssues(issues []*issue, field string, desc bool) {
	cmpIssues := func(a, b *issue) int {
		switch field {
		case "created":
			return a.Created.Compare(b.Created)
		case "updated":
			return a.Updated.Compare(b.Updated)
		case "priority":
			return cmp.Compare(priorityRank(a), priorityRank(b))
		case "status":
			return strings.Compare(a.Status.Name, b.Status.Name)
		case "summary":
			return strings.Compare(a.Values["summary"].Text, b.Values["summary"].Text)
		}
		return cmp.Or(strings.Compare(a.project(), b.project()), cmp.Compare(keyNumber(a), keyNumber(b)))
	}
	slices.SortStableFunc(issues, func(a, b *issue) int {
		if desc {
			return cmpIssues(b, a)
		}
		return cmpIssues(a, b)
	})
}

func priorityRank(i *issue) int {
	p := i.Values["priority"].Chosen
	if len(p) == 0 {
		return len(priorities)
	}
	n, _ := strconv.Atoi(p[0].ID)
	return n
}

func keyNumber(i *issue) int {
	_, n, _ := strings.Cut(i.Key, "-")
	v, _ := strconv.Atoi(n)
	return v
}
