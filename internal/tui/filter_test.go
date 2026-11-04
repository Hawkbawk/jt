package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/hawkbawk/jt/internal/jira"
)

func filterModel(t *testing.T) (BoardModel, *fakeBackend) {
	t.Helper()
	fb := &fakeBackend{board: testBoard()}
	fb.board.QuickFilters = []jira.Filter{{Name: "Only Bugs", JQL: "type = Bug"}, {Name: "Recent", JQL: "updated >= -7d"}}
	fb.board.Assignees = []jira.Filter{{Name: "Alice", JQL: `assignee = "a"`}, {Name: "Unassigned", JQL: "assignee is EMPTY"}}
	m := newTestModel(t, "TEST", fb)
	return drain(t, m, m.Init()), fb
}

func TestFilterPopupSwitchesBetweenTabs(t *testing.T) {
	m, _ := filterModel(t)
	m = pressDrain(t, m, "f")
	if _, ok := m.popup.Content().(*filterPopup); !ok {
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
	if m.popup.IsOpen() || m.board.filter == nil || m.board.filter.Name != "Alice" {
		t.Fatal("selected filter should be active and the popup closed")
	}
	if !strings.Contains(m.View().Content, "filter: Alice") {
		t.Error("header should show the active filter")
	}

	// The popup reopens on the tab and row of the active filter.
	m = pressDrain(t, m, "f")
	if p := m.popup.Content().(*filterPopup); p.tab != 1 || p.cursor != 1 {
		t.Fatalf("tab, cursor = %d, %d", p.tab, p.cursor)
	}
	m = pressDrain(t, m, "k", "enter")
	if got := fb.filterCalls[len(fb.filterCalls)-1]; got != "" || m.board.filter != nil {
		t.Fatalf("All issues should clear the filter, got %q", got)
	}
}

func TestFilterPopupSearchesBothLists(t *testing.T) {
	for _, tt := range []struct {
		name, query string
		tab         int
	}{
		{name: "Quick hotfix", query: "quick h", tab: 0},
		{name: "Jacqueline", query: "jacqueline", tab: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fb := &fakeBackend{board: testBoard()}
			want := jira.Filter{Name: tt.name, JQL: "test filter"}
			fb.board.QuickFilters = []jira.Filter{{Name: "Other filter"}, want}
			fb.board.Assignees = []jira.Filter{{Name: "Other assignee"}, want}
			m := newTestModel(t, "TEST", fb)
			m = drain(t, m, m.Init())
			m = pressDrain(t, m, "f")
			if tt.tab == 1 {
				m = pressDrain(t, m, "tab")
			}
			m = pressDrain(t, m, "/")
			for _, r := range strings.ToUpper(tt.query[:1]) + tt.query[1:] {
				m = pressDrain(t, m, string(r))
			}
			p := m.popup.Content().(*filterPopup)
			if p.tab != tt.tab || !p.selectField.GetFiltering() {
				t.Fatal("typing a query should search without switching tabs or closing")
			}
			v := ansi.Strip(m.View().Content)
			if !strings.Contains(v, tt.name) || strings.Contains(v, "Other") || strings.Contains(v, "All issues") {
				t.Fatalf("search should show only the matching option:\n%s", v)
			}
			m = pressDrain(t, m, "enter")
			if m.popup.IsOpen() || m.board.filter == nil || *m.board.filter != want {
				t.Fatal("enter should apply the matching filter and close")
			}
			if got := fb.filterCalls[len(fb.filterCalls)-1]; got != want.JQL {
				t.Fatalf("filterJQL = %q, want %q", got, want.JQL)
			}
		})
	}
}

func TestFilterPopupSearchNoMatchesAndCancel(t *testing.T) {
	m, fb := filterModel(t)
	m = pressDrain(t, m, "f", "j", "enter", "f", "/")
	calls := len(fb.filterCalls)
	m = pressDrain(t, m, "z", "z", "z", "enter")
	if !m.popup.IsOpen() || len(fb.filterCalls) != calls {
		t.Fatal("enter with no matches should not apply a stale selection")
	}
	m = pressDrain(t, m, "esc")
	if m.popup.IsOpen() || m.board.filter.Name != "Only Bugs" || len(fb.filterCalls) != calls {
		t.Fatal("esc should cancel search without changing the active filter")
	}
}

func TestFilterPopupSwitchTabResetsSearch(t *testing.T) {
	m, _ := filterModel(t)
	m = pressDrain(t, m, "f", "/", "b", "u", "g", "tab", "shift+tab")
	p := m.popup.Content().(*filterPopup)
	v := ansi.Strip(m.View().Content)
	if p.selectField.GetFiltering() || !strings.Contains(v, "Recent") || !strings.Contains(v, "All issues") {
		t.Fatal("switching tabs should reset the search and show all options")
	}
}

func TestFilterPopupEmptyListCanClearActiveFilter(t *testing.T) {
	fb := &fakeBackend{board: testBoard()}
	fb.board.QuickFilters = []jira.Filter{{Name: "Only Bugs", JQL: "type = Bug"}}
	m := newTestModel(t, "TEST", fb)
	m = drain(t, m, m.Init())
	m = pressDrain(t, m, "f", "j", "enter", "f", "tab")
	v := ansi.Strip(m.View().Content)
	if !strings.Contains(v, "No assignees on this board.") || !strings.Contains(v, "All issues") {
		t.Fatalf("an empty list should explain why it is empty and allow clearing:\n%s", v)
	}
	m = pressDrain(t, m, "enter")
	if m.popup.IsOpen() || m.board.filter != nil {
		t.Fatal("All issues on an empty tab should clear the active filter")
	}
}
