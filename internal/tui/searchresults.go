package tui

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/hawkbawk/jt/internal/jira"
)

type searchLoadedMsg struct {
	id       int
	issues   []jira.Issue
	focusKey string
	err      error
}

func runSearch(ctx context.Context, b Backend, id int, jql, focusKey string) tea.Cmd {
	return func() tea.Msg {
		issues, err := b.Search(ctx, jql)
		return searchLoadedMsg{id: id, issues: issues, focusKey: focusKey, err: err}
	}
}

type resultList struct {
	issues   []jira.Issue
	selector *huh.Select[string]
}

func (l *resultList) set(issues []jira.Issue, focusKey string) {
	l.issues = issues
	l.selector = huh.NewSelect[string]().Key(fieldResults).
		Title(fmt.Sprintf("Results (%d)", len(issues))).Value(&focusKey)
}

// selected uses Hovered rather than GetValue: shortcuts act on the cursor
// without requiring the option to be submitted first.
func (l resultList) selected() *jira.Issue {
	if l.selector == nil {
		return nil
	}
	k, ok := l.selector.Hovered()
	if !ok {
		return nil
	}
	for i := range l.issues {
		if l.issues[i].Key == k {
			return &l.issues[i]
		}
	}
	return nil
}

func (l *resultList) resize(width, height int) {
	if l.selector == nil || len(l.issues) == 0 {
		return
	}
	focusKey, hovered := l.selector.Hovered()
	opts := make([]huh.Option[string], len(l.issues))
	for i, iss := range l.issues {
		assignee := iss.Assignee.Name
		if assignee == "" {
			assignee = "Unassigned"
		}
		label := fmt.Sprintf("%s · %s · %s · %s", iss.Key, iss.Status, assignee, iss.Summary)
		opts[i] = huh.NewOption(ansi.Truncate(label, max(1, width-6), "…"), iss.Key)
	}
	l.selector.Options(opts...).Height(min(len(opts)+1, max(2, min(10, height))))
	if hovered {
		l.selector.Value(&focusKey)
	}
}
