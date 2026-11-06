package board

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/hawkbawk/jt/internal/components/filterpicker"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/tui/tuitest"
)

func filterModel(t *testing.T) (Model, *tuitest.Backend) {
	t.Helper()
	fb := &tuitest.Backend{BoardData: testBoard()}
	fb.BoardData.QuickFilters = []jira.Filter{{Name: "Only Bugs", JQL: "type = Bug"}, {Name: "Recent", JQL: "updated >= -7d"}}
	fb.BoardData.Assignees = []jira.Filter{{Name: "Alice", JQL: `assignee = "a"`}, {Name: "Unassigned", JQL: "assignee is EMPTY"}}
	m := newTestModel(t, "TEST", fb)
	return tuitest.Drain(t, m, m.Init()), fb
}

func TestFilterKeyOpensPopupAndSwitchesTabs(t *testing.T) {
	m, _ := filterModel(t)
	m = tuitest.PressDrain(t, m, "f")
	if _, ok := m.Popup.Content().(*filterpicker.Picker); !ok {
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

func TestFilterSelectionReloadsBoardAndClears(t *testing.T) {
	m, fb := filterModel(t)
	m = tuitest.PressDrain(t, m, "f", "tab", "j", "enter")

	if got := fb.FilterCalls[len(fb.FilterCalls)-1]; got != `assignee = "a"` {
		t.Fatalf("filterJQL = %q", got)
	}
	if m.Popup.IsOpen() || m.board.filter == nil || m.board.filter.Name != "Alice" {
		t.Fatal("selected filter should be active and the popup closed")
	}
	if !strings.Contains(m.View().Content, "filter: Alice") {
		t.Error("header should show the active filter")
	}

	// The popup reopens on the tab of the active filter.
	m = tuitest.PressDrain(t, m, "f")
	if v := ansi.Strip(m.View().Content); !strings.Contains(v, "Alice") || strings.Contains(v, "Only Bugs") {
		t.Fatal("the popup should reopen on the assignees tab")
	}
	m = tuitest.PressDrain(t, m, "k", "enter")
	if got := fb.FilterCalls[len(fb.FilterCalls)-1]; got != "" || m.board.filter != nil {
		t.Fatalf("All issues should clear the filter, got %q", got)
	}
}

func TestFilterPopupEmptyListCanClearActiveFilter(t *testing.T) {
	fb := &tuitest.Backend{BoardData: testBoard()}
	fb.BoardData.QuickFilters = []jira.Filter{{Name: "Only Bugs", JQL: "type = Bug"}}
	m := newTestModel(t, "TEST", fb)
	m = tuitest.Drain(t, m, m.Init())
	m = tuitest.PressDrain(t, m, "f", "j", "enter", "f", "tab")
	v := ansi.Strip(m.View().Content)
	if !strings.Contains(v, "No assignees on this board.") || !strings.Contains(v, "All issues") {
		t.Fatalf("an empty list should explain why it is empty and allow clearing:\n%s", v)
	}
	m = tuitest.PressDrain(t, m, "enter")
	if m.Popup.IsOpen() || m.board.filter != nil {
		t.Fatal("All issues on an empty tab should clear the active filter")
	}
}
