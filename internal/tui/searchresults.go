package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/evertras/bubble-table/table"

	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/styles"
)

type searchLoadedMsg struct {
	issues []jira.Issue
	err    error
}

func runSearch(b Backend, jql string) tea.Cmd {
	return func() tea.Msg {
		issues, err := b.Search(jql)
		return searchLoadedMsg{issues: issues, err: err}
	}
}

const (
	colKey      = "key"
	colType     = "type"
	colStatus   = "status"
	colAssignee = "assignee"
	colSummary  = "summary"
)

// tableKeys leaves out the keys jt uses itself: g/G jump through resultList.move,
// and the table's filter and horizontal paging aren't wanted.
var tableKeys = table.KeyMap{
	RowDown:   key.NewBinding(key.WithKeys("down", "j")),
	RowUp:     key.NewBinding(key.WithKeys("up", "k")),
	PageDown:  key.NewBinding(key.WithKeys("pgdown", "ctrl+d")),
	PageUp:    key.NewBinding(key.WithKeys("pgup", "ctrl+u")),
	PageFirst: key.NewBinding(key.WithKeys("home")),
	PageLast:  key.NewBinding(key.WithKeys("end")),
}

// resultList shows the search results as a table.
type resultList struct {
	issues        []jira.Issue
	table         table.Model
	width, height int
}

func (l *resultList) restyle() {
	l.table = l.table.
		WithBaseStyle(lipgloss.NewStyle()).
		HeaderStyle(styles.ColumnHeader().Padding(0)).
		HighlightStyle(lipgloss.NewStyle().Bold(true).Foreground(styles.Highlight())).
		Border(table.Border{}).
		WithOuterBorder(false).
		Focused(true)
}

func (l *resultList) set(issues []jira.Issue) {
	l.issues = issues
	rows := make([]table.Row, len(issues))
	for i, iss := range issues {
		assignee := iss.Assignee.Name
		if assignee == "" {
			assignee = "Unassigned"
		}
		rows[i] = table.NewRow(table.RowData{
			colKey: iss.Key, colType: iss.Type, colStatus: iss.Status,
			colAssignee: assignee, colSummary: iss.Summary,
		})
	}
	l.table = table.New([]table.Column{
		table.NewColumn(colKey, "Key", 10),
		table.NewColumn(colType, "Type", 10),
		table.NewColumn(colStatus, "Status", 14),
		table.NewColumn(colAssignee, "Assignee", 16),
		table.NewFlexColumn(colSummary, "Summary", 1),
	}).WithKeyMap(tableKeys).WithRows(rows)
	l.restyle()
	l.fit()
}

// selected is the issue under the cursor.
func (l resultList) selected() *jira.Issue {
	if len(l.issues) == 0 {
		return nil
	}
	k, _ := l.table.HighlightedRow().Data[colKey].(string)
	for i := range l.issues {
		if l.issues[i].Key == k {
			return &l.issues[i]
		}
	}
	return nil
}

func (l *resultList) resize(width, height int) {
	l.width, l.height = width, height
	l.fit()
}

// fit sizes the table, keeping the cursor on the same issue.
func (l *resultList) fit() {
	cursor := l.table.GetHighlightedRowIndex()
	l.table = l.table.WithTargetWidth(l.width).WithTargetHeight(max(2, l.height)).WithHighlightedRow(cursor)
}

// move puts the cursor on the first or last result.
func (l *resultList) move(last bool) {
	i := 0
	if last && len(l.issues) > 0 {
		i = len(l.issues) - 1
	}
	l.table = l.table.WithHighlightedRow(i)
}

func (l *resultList) update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	l.table, cmd = l.table.Update(msg)
	return cmd
}

func (l resultList) view() string { return l.table.View() }
