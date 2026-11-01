package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/styles"
)

const (
	maxStatusWidth   = 16
	maxAssigneeWidth = 20
	minSummaryWidth  = 10
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

// resultList is the issues a search found, with the cursor on one of them.
type resultList struct {
	issues []jira.Issue
	cursor int
}

// set replaces the issues and moves the cursor to focusKey when it's found.
func (l *resultList) set(issues []jira.Issue, focusKey string) {
	l.issues = issues
	l.cursor = 0
	for i, iss := range issues {
		if iss.Key == focusKey {
			l.cursor = i
		}
	}
}

// selected returns the issue under the cursor, or nil when there is none.
func (l *resultList) selected() *jira.Issue {
	if l.cursor >= len(l.issues) {
		return nil
	}
	return &l.issues[l.cursor]
}

// move applies a navigation key and reports whether it was one.
func (l *resultList) move(msg tea.KeyPressMsg) bool {
	last := max(0, len(l.issues)-1)
	switch {
	case key.Matches(msg, keys.down):
		l.cursor = min(l.cursor+1, last)
	case key.Matches(msg, keys.up):
		l.cursor = max(l.cursor-1, 0)
	case key.Matches(msg, keys.top):
		l.cursor = 0
	case key.Matches(msg, keys.bottom):
		l.cursor = last
	default:
		return false
	}
	return true
}

// view draws up to height lines: a title line and a page of issues around the
// cursor.
func (l resultList) view(width, height int, focused bool) string {
	if height <= 0 {
		return ""
	}
	title := styles.Dim().Render(fmt.Sprintf("Results (%d)", len(l.issues)))
	if focused {
		title = styles.FocusedColumnHeader().Padding(0).Render(fmt.Sprintf("Results (%d)", len(l.issues)))
	}
	perPage := max(1, height-1)
	start := pageStart(l.cursor, perPage)
	end := min(len(l.issues), start+perPage)

	keyW, statusW, assigneeW := 0, 0, 0
	for _, iss := range l.issues {
		keyW = max(keyW, ansi.StringWidth(iss.Key))
		statusW = min(max(statusW, ansi.StringWidth(iss.Status)), maxStatusWidth)
		assigneeW = min(max(assigneeW, ansi.StringWidth(assigneeName(iss))), maxAssigneeWidth)
	}
	// Narrow terminals drop the columns beside the summary before squeezing it.
	if width-2-keyW-statusW-assigneeW-3 < minSummaryWidth {
		statusW, assigneeW = 0, 0
	}

	lines := []string{title}
	for i := start; i < end; i++ {
		lines = append(lines, renderResultRow(l.issues[i], i == l.cursor && focused, width, keyW, statusW, assigneeW))
	}
	return strings.Join(lines, "\n")
}

func assigneeName(iss jira.Issue) string {
	if iss.Assignee.Name == "" {
		return "Unassigned"
	}
	return iss.Assignee.Name
}

// renderResultRow draws one issue on a line. A zero statusW or assigneeW
// leaves that column out.
func renderResultRow(iss jira.Issue, selected bool, width, keyW, statusW, assigneeW int) string {
	cell := func(s string, w int) string {
		return ansi.Truncate(s, w, "…") + strings.Repeat(" ", max(0, w-ansi.StringWidth(s)))
	}
	parts := []string{cell(iss.Key, keyW)}
	if statusW > 0 {
		parts = append(parts, cell(iss.Status, statusW), cell(assigneeName(iss), assigneeW))
	}
	prefix := "  "
	if selected {
		prefix = "› "
	}
	summaryW := max(1, width-ansi.StringWidth(prefix)-ansi.StringWidth(strings.Join(parts, " "))-1)
	line := prefix + strings.Join(parts, " ") + " " + ansi.Truncate(iss.Summary, summaryW, "…")
	if selected {
		return lipgloss.NewStyle().Foreground(styles.Highlight()).Bold(true).Render(ansi.Strip(line))
	}
	// The key is styled after layout so padding isn't colored.
	return strings.Replace(line, iss.Key, styles.Key().Render(iss.Key), 1)
}
