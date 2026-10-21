package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/ryanhawkins/jt/internal/jira"
)

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func field(label, value string) string {
	return dimStyle.Render(label+": ") + orDash(value)
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
		b.WriteString(dimStyle.Render("No description."))
	} else {
		b.WriteString(md(d.Description, width))
	}
	return b.String()
}
