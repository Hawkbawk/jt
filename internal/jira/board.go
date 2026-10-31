package jira

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

const (
	boardTypeScrum = "scrum"

	scrumJQL  = "sprint in openSprints()"
	kanbanJQL = "statusCategory != Done OR resolved >= -14d"
)

type boardInfo struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Location struct {
		ProjectKey string `json:"projectKey"`
	} `json:"location"`
}

type quickFilterList struct {
	Values []struct {
		Name string `json:"name"`
		JQL  string `json:"jql"`
	} `json:"values"`
}

type boardConfig struct {
	ColumnConfig struct {
		Columns []struct {
			Name     string `json:"name"`
			Statuses []struct {
				ID string `json:"id"`
			} `json:"statuses"`
		} `json:"columns"`
	} `json:"columnConfig"`
}

// pickBoard chooses the board to display for a project. A board located in
// the project is preferred over one that merely includes its issues.
func pickBoard(project string, boards []boardInfo) (boardInfo, error) {
	if len(boards) == 0 {
		return boardInfo{}, fmt.Errorf("no boards found for project %s", project)
	}
	for _, b := range boards {
		if b.Location.ProjectKey == project {
			return b, nil
		}
	}
	return boards[0], nil
}

// boardJQL limits the issues loaded so large boards stay usable: scrum boards
// show the open sprints, kanban boards show open work plus recently resolved.
func boardJQL(boardType string) string {
	if boardType == boardTypeScrum {
		return scrumJQL
	}
	return kanbanJQL
}

// combineJQL narrows base with an extra condition.
func combineJQL(base, extra string) string {
	if extra == "" {
		return base
	}
	return "(" + base + ") AND (" + extra + ")"
}

// quickFilters maps the board's configured quick filters to Filters.
func quickFilters(res quickFilterList) []Filter {
	out := make([]Filter, 0, len(res.Values))
	for _, q := range res.Values {
		out = append(out, Filter{Name: q.Name, JQL: q.JQL})
	}
	return out
}

// AssigneeFilters builds one filter per distinct assignee, sorted by name,
// with Unassigned always last.
func AssigneeFilters(issues []Issue) []Filter {
	seen := make(map[string]bool)
	var out []Filter
	for _, iss := range issues {
		if iss.AssigneeID == "" {
			continue
		}
		if seen[iss.AssigneeID] {
			continue
		}
		seen[iss.AssigneeID] = true
		out = append(out, Filter{Name: iss.Assignee, JQL: fmt.Sprintf("assignee = %q", iss.AssigneeID), AssigneeID: iss.AssigneeID})
	}
	slices.SortFunc(out, func(a, b Filter) int { return strings.Compare(a.Name, b.Name) })
	return append(out, Filter{Name: "Unassigned", JQL: "assignee is EMPTY"})
}

func columnsFrom(cfg boardConfig) ([]Column, error) {
	if len(cfg.ColumnConfig.Columns) == 0 {
		return nil, errors.New("board has no columns configured")
	}
	cols := make([]Column, 0, len(cfg.ColumnConfig.Columns))
	for _, c := range cfg.ColumnConfig.Columns {
		col := Column{Name: c.Name}
		for _, s := range c.Statuses {
			col.StatusIDs = append(col.StatusIDs, s.ID)
		}
		cols = append(cols, col)
	}
	return cols, nil
}

// distribute places each issue into the column that owns its status. Issues
// with an unmapped status are dropped, matching Jira's own board behavior.
func distribute(cols []Column, issues []Issue) {
	byStatus := make(map[string]int)
	for i, c := range cols {
		for _, id := range c.StatusIDs {
			byStatus[id] = i
		}
	}
	for _, iss := range issues {
		if i, ok := byStatus[iss.StatusID]; ok {
			cols[i].Issues = append(cols[i].Issues, iss)
		}
	}
}
