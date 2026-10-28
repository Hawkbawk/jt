package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/styles"
)

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func field(label, value string) string {
	return styles.Dim().Render(label+": ") + orDash(value)
}

func renderDetail(d *jira.IssueDetail, width int, md markdownRenderer) string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Width(width).Render(d.Summary))
	b.WriteString("\n\n")
	for _, line := range []string{
		field("Type", d.Type) + "   " + field("Status", d.Status) + "   " + field("Priority", d.Priority),
		field("Assignee", d.Assignee) + "   " + field("Reporter", d.Reporter),
		field("Created", d.Created) + "   " + field("Updated", d.Updated),
		field("Labels", strings.Join(d.Labels, ", ")),
	} {
		b.WriteString(line + "\n")
	}
	b.WriteString("\n")

	if strings.TrimSpace(d.Description) == "" {
		b.WriteString(styles.Dim().Render("No description."))
	} else {
		b.WriteString(md(d.Description, width))
	}
	return b.String()
}

func renderComments(d *jira.IssueDetail, width int, md markdownRenderer) string {
	if len(d.Comments) == 0 {
		return styles.Dim().Render("No comments.")
	}
	sep := styles.Dim().Render(strings.Repeat("─", width))
	parts := make([]string, 0, len(d.Comments))
	for _, c := range d.Comments {
		header := lipgloss.NewStyle().Bold(true).Render(orDash(c.Author)) + styles.Dim().Render(" · "+c.Created)
		parts = append(parts, header+"\n"+strings.TrimRight(md(c.Body, width), "\n"))
	}
	return strings.Join(parts, "\n"+sep+"\n")
}
