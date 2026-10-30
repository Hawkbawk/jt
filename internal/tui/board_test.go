package tui

import (
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/hawkbawk/jt/internal/jira"
)

func TestCardWidth(t *testing.T) {
	c := renderCard(jira.Issue{Key: "INTEROP-10891", Assignee: jira.Option{Name: "David Varga"}, Summary: "x"}, false, 40)
	if w := lipgloss.Width(c); w != 40 || lipgloss.Height(c) != 2+1+summaryLines {
		t.Fatalf("w=%d h=%d\n%s", w, lipgloss.Height(c), c)
	}
}
