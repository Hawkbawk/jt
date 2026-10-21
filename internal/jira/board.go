package jira

import (
	"errors"
	"fmt"
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
