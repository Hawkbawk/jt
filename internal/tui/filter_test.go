package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/hawkbawk/jt/internal/jira"
)

func filterModel(t *testing.T) (Model, *fakeBackend) {
	t.Helper()
	fb := &fakeBackend{board: testBoard()}
	fb.board.QuickFilters = []jira.Filter{{Name: "Only Bugs", JQL: "type = Bug"}, {Name: "Recent", JQL: "updated >= -7d"}}
	fb.board.Assignees = []jira.Filter{{Name: "Alice", JQL: `assignee = "a"`}, {Name: "Unassigned", JQL: "assignee is EMPTY"}}
	m := newTestModel(t, "TEST", fb)
	return drain(t, m, m.Init()), fb
}

func TestFilterPopupSwitchesBetweenTabs(t *testing.T) {
	m, _ := filterModel(t)
	m, _ = press(t, m, "f")
	if _, ok := m.popup.(*filterPopup); !ok {
		t.Fatal("f should open the filter popup")
	}
	v := ansi.Strip(m.View().Content)
	for _, want := range []string{"Quick filters", "Assignees", "All issues", "Only Bugs", "Recent"} {
		if !strings.Contains(v, want) {
			t.Errorf("quick filter tab missing %q", want)
		}
	}
	if strings.Contains(v, "Alice") {
		t.Error("assignees should not show on the quick filters tab")
	}

	m, _ = press(t, m, "tab")
	v = m.View().Content
	if !strings.Contains(v, "Alice") || strings.Contains(v, "Only Bugs") {
		t.Error("tab should switch to the assignees list")
	}
	m, _ = press(t, m, "shift+tab")
	if !strings.Contains(m.View().Content, "Only Bugs") {
		t.Error("shift+tab should switch back")
	}
}

func TestFilterPopupOpensOnAssigneesWithoutQuickFilters(t *testing.T) {
	p := newFilterPopup(nil, []jira.Filter{{Name: "Alice"}}, nil)
	if p.tab != 1 {
		t.Fatalf("tab = %d, want the assignees tab", p.tab)
	}
}

func TestFilterSelectionReloadsBoardAndClears(t *testing.T) {
	m, fb := filterModel(t)
	m = pressDrain(t, m, "f", "tab", "j", "enter")

	if got := fb.filterCalls[len(fb.filterCalls)-1]; got != `assignee = "a"` {
		t.Fatalf("filterJQL = %q", got)
	}
	if m.popup != nil || m.board.filter == nil || m.board.filter.Name != "Alice" {
		t.Fatal("selected filter should be active and the popup closed")
	}
	if !strings.Contains(m.View().Content, "filter: Alice") {
		t.Error("header should show the active filter")
	}

	// The popup reopens on the tab and row of the active filter.
	m, _ = press(t, m, "f")
	if p := m.popup.(*filterPopup); p.tab != 1 || p.cursor != 1 {
		t.Fatalf("tab, cursor = %d, %d", p.tab, p.cursor)
	}
	m = pressDrain(t, m, "k", "enter")
	if got := fb.filterCalls[len(fb.filterCalls)-1]; got != "" || m.board.filter != nil {
		t.Fatalf("All issues should clear the filter, got %q", got)
	}
}
