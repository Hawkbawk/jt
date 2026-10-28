package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/styles"
)

const (
	minColumnWidth = 28
	summaryLines   = 2
	// cardHeight is the border plus a key line and the summary lines.
	cardHeight = 2 + 1 + summaryLines
)

// pageStart returns the first index of the page containing cursor.
func pageStart(cursor, perPage int) int {
	if perPage <= 0 {
		return 0
	}
	return (cursor / perPage) * perPage
}

func renderBoard(n *boardNav, width, height int) string {
	cols := n.board.Columns
	if len(cols) == 0 || width <= 0 || height <= 0 {
		return ""
	}

	visibleCols := max(1, min(len(cols), width/minColumnWidth))
	colWidth := width / visibleCols
	first := pageStart(n.col, visibleCols)
	last := min(len(cols), first+visibleCols)

	rendered := make([]string, 0, last-first)
	for c := first; c < last; c++ {
		rendered = append(rendered, renderColumn(cols[c], c == n.col, n.rowIn(c), colWidth, height, c == first && first > 0, c == last-1 && last < len(cols)))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
}

// headerText fits the column title into width, with arrows at the edges when
// more columns exist off-screen in that direction.
func headerText(col jira.Column, width int, moreLeft, moreRight bool) string {
	left, right := "", ""
	if moreLeft {
		left = "◀ "
	}
	if moreRight {
		right = " ▶"
	}
	title := ansi.Truncate(fmt.Sprintf("%s (%d)", col.Name, len(col.Issues)), max(0, width-lipgloss.Width(left)-lipgloss.Width(right)), "…")
	gap := max(0, width-lipgloss.Width(left)-lipgloss.Width(title)-lipgloss.Width(right))
	return left + title + strings.Repeat(" ", gap) + right
}

func renderColumn(col jira.Column, focused bool, row, width, height int, moreLeft, moreRight bool) string {
	header := styles.ColumnHeader()
	if focused {
		header = styles.FocusedColumnHeader()
	}
	lines := []string{header.Width(width).Render(headerText(col, width-2, moreLeft, moreRight))}

	// Reserve the header and both scroll indicator lines.
	perPage := max(1, (height-3)/cardHeight)
	start := pageStart(row, perPage)
	end := min(len(col.Issues), start+perPage)

	if start > 0 {
		lines = append(lines, styles.Dim().Width(width).Align(lipgloss.Center).Render("▲"))
	} else {
		lines = append(lines, "")
	}
	for i := start; i < end; i++ {
		lines = append(lines, renderCard(col.Issues[i], focused && i == row, width))
	}
	if end < len(col.Issues) {
		lines = append(lines, styles.Dim().Width(width).Align(lipgloss.Center).Render("▼"))
	}

	return lipgloss.NewStyle().Width(width).Height(height).MaxHeight(height).Render(strings.Join(lines, "\n"))
}

func renderCard(iss jira.Issue, selected bool, width int) string {
	style := styles.Card()
	if selected {
		style = styles.SelectedCard()
	}
	inner := max(1, width-style.GetHorizontalFrameSize())

	assignee := iss.Assignee
	if assignee == "" {
		assignee = "Unassigned"
	}
	keyWidth := lipgloss.Width(iss.Key)
	assignee = ansi.Truncate(assignee, max(0, inner-keyWidth-1), "…")
	gap := max(1, inner-keyWidth-lipgloss.Width(assignee))
	top := styles.Key().Render(iss.Key) + strings.Repeat(" ", gap) + styles.Dim().Render(assignee)

	return style.Width(width).Render(top + "\n" + clampLines(iss.Summary, inner, summaryLines))
}

// clampLines word-wraps s to width and keeps at most n lines, padding short
// text so every card has the same height.
func clampLines(s string, width, n int) string {
	lines := strings.Split(ansi.Wordwrap(s, width, ""), "\n")
	if len(lines) > n {
		lines = lines[:n]
		lines[n-1] = ansi.Truncate(lines[n-1]+"…", width, "…")
	}
	for len(lines) < n {
		lines = append(lines, "")
	}
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, width, "…")
	}
	return strings.Join(lines, "\n")
}
