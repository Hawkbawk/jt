package filterpicker

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/tui/modal"
	"github.com/hawkbawk/jt/internal/tui/tuitest"
)

func keyMsg(s string) tea.KeyPressMsg {
	switch s {
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEsc}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	}
	return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
}

// newPicker returns a sized picker that has started, as a modal host does.
func newPicker(quick, assignees []jira.Filter, active *jira.Filter) *Picker {
	p := New(quick, assignees, active)
	p.Init()
	p.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return p
}

// press sends each key to p and returns the command from the last one.
func press(t *testing.T, p *Picker, keys ...string) tea.Cmd {
	t.Helper()
	var cmd tea.Cmd
	for _, k := range keys {
		var m tea.Model
		m, cmd = p.Update(keyMsg(k))
		*p = *m.(*Picker)
	}
	return cmd
}

// settle runs cmd and the commands it leads to, feeding their messages back to
// p, until the modal closes. Forms finish a step after the key that completes
// them, so the close can arrive a message or two later. ok is false if nothing
// closed.
func settle(p *Picker, cmd tea.Cmd) (tea.Msg, bool) {
	queue := []tea.Cmd{cmd}
	for steps := 0; len(queue) > 0 && steps < 100; steps++ {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		switch msg := c().(type) {
		case tea.BatchMsg:
			queue = append(queue, msg...)
		case modal.Closes:
			return msg, true
		default:
			_, next := p.Update(msg)
			queue = append(queue, next)
		}
	}
	return nil, false
}

func view(p *Picker) string { return tuitest.ModalView(p, p.width) }

func testFilters() ([]jira.Filter, []jira.Filter) {
	quick := []jira.Filter{{Name: "Only Bugs", JQL: "type = Bug"}, {Name: "Recent", JQL: "updated >= -7d"}}
	assignees := []jira.Filter{{Name: "Alice", JQL: `assignee = "a"`}, {Name: "Unassigned", JQL: "assignee is EMPTY"}}
	return quick, assignees
}

func TestOpensOnActiveFilter(t *testing.T) {
	quick, assignees := testFilters()
	p := newPicker(quick, assignees, &assignees[0])
	if p.tab != 1 || p.cursor != 1 {
		t.Fatalf("tab, cursor = %d, %d; want the assignees tab on Alice", p.tab, p.cursor)
	}
}

func TestOpensOnAssigneesWithoutQuickFilters(t *testing.T) {
	p := newPicker(nil, []jira.Filter{{Name: "Alice"}}, nil)
	if p.tab != 1 {
		t.Fatalf("tab = %d, want the assignees tab", p.tab)
	}
}

func TestSwitchesBetweenTabs(t *testing.T) {
	quick, assignees := testFilters()
	p := newPicker(quick, assignees, nil)
	v := view(p)
	for _, want := range []string{"Quick filters", "Assignees", "All issues", "Only Bugs", "Recent"} {
		if !strings.Contains(v, want) {
			t.Errorf("quick filter tab missing %q", want)
		}
	}
	if strings.Contains(v, "Alice") {
		t.Error("assignees should not show on the quick filters tab")
	}

	press(t, p, "tab")
	if v := view(p); !strings.Contains(v, "Alice") || strings.Contains(v, "Only Bugs") {
		t.Error("tab should switch to the assignees list")
	}
	press(t, p, "shift+tab")
	if !strings.Contains(view(p), "Only Bugs") {
		t.Error("shift+tab should switch back")
	}
}

func TestApplyReportsFilterAndClears(t *testing.T) {
	quick, assignees := testFilters()
	p := newPicker(quick, assignees, nil)
	closed, ok := settle(p, press(t, p, "tab", "j", "enter"))
	if !ok {
		t.Fatal("enter should close the modal with the selection")
	}
	msg := closed.(SelectedMsg)
	if msg.Filter == nil || *msg.Filter != assignees[0] {
		t.Fatalf("selected = %+v, want Alice", msg.Filter)
	}

	p = newPicker(quick, assignees, &assignees[0])
	closed, _ = settle(p, press(t, p, "k", "enter"))
	if msg := closed.(SelectedMsg); msg.Filter != nil {
		t.Fatalf("All issues should clear the filter, got %+v", msg.Filter)
	}
}

func TestSearchesBothLists(t *testing.T) {
	for _, tt := range []struct {
		name, query string
		tab         int
	}{
		{name: "Quick hotfix", query: "quick h", tab: 0},
		{name: "Jacqueline", query: "jacqueline", tab: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			want := jira.Filter{Name: tt.name, JQL: "test filter"}
			quick := []jira.Filter{{Name: "Other filter"}, want}
			assignees := []jira.Filter{{Name: "Other assignee"}, want}
			p := newPicker(quick, assignees, nil)
			if tt.tab == 1 {
				press(t, p, "tab")
			}
			press(t, p, "/")
			for _, r := range strings.ToUpper(tt.query[:1]) + tt.query[1:] {
				press(t, p, string(r))
			}
			if p.tab != tt.tab || !p.Filtering() {
				t.Fatal("typing a query should search without switching tabs or closing")
			}
			v := view(p)
			if !strings.Contains(v, tt.name) || strings.Contains(v, "Other") || strings.Contains(v, "All issues") {
				t.Fatalf("search should show only the matching option:\n%s", v)
			}
			closed, ok := settle(p, press(t, p, "enter"))
			if !ok {
				t.Fatal("enter should apply the matching filter and close")
			}
			if msg := closed.(SelectedMsg); msg.Filter == nil || *msg.Filter != want {
				t.Fatalf("selected = %+v, want %+v", msg.Filter, want)
			}
		})
	}
}

func TestSearchNoMatchesAndCancel(t *testing.T) {
	quick, assignees := testFilters()
	active := &quick[0]
	p := newPicker(quick, assignees, active)
	press(t, p, "/", "z", "z", "z")
	if _, ok := settle(p, press(t, p, "enter")); ok {
		t.Fatal("enter with no matches should not apply a stale selection")
	}
	press(t, p, "esc")
	if p.Filtering() {
		t.Fatal("esc should stop searching without closing the modal")
	}
	if _, ok := settle(p, press(t, p, "esc")); !ok {
		t.Fatal("esc after an unmatched search should close")
	}
}

func TestEscapeStopsAndClearsSearch(t *testing.T) {
	quick, assignees := testFilters()
	p := newPicker(quick, assignees, nil)
	press(t, p, "/", "b", "u", "g", "esc")
	v := view(p)
	if p.Filtering() || !strings.Contains(v, "Only Bugs") || strings.Contains(v, "Recent") {
		t.Fatal("esc should stop editing the search and keep the matching options")
	}
	if !strings.Contains(v, "esc clear search") || strings.Contains(v, "esc close") {
		t.Fatal("help should show the form's clear-search binding, not close")
	}

	if _, ok := settle(p, press(t, p, "esc")); ok {
		t.Fatal("a second esc should clear the search without closing")
	}
	v = view(p)
	if !strings.Contains(v, "Recent") || !strings.Contains(v, "All issues") {
		t.Fatal("the cleared search should show every option")
	}
	if !strings.Contains(v, "esc close") {
		t.Fatal("help should show close after the search is cleared")
	}
	if _, ok := settle(p, press(t, p, "esc")); !ok {
		t.Fatal("esc without a search should close")
	}
}

func TestHelpFollowsSearchState(t *testing.T) {
	quick, assignees := testFilters()
	p := newPicker(quick, assignees, nil)
	v := view(p)
	for _, want := range []string{"tab next", "shift+tab prev", "/ search", "enter apply", "esc close"} {
		if !strings.Contains(v, want) {
			t.Errorf("help missing %q", want)
		}
	}
	press(t, p, "/")
	v = view(p)
	if !strings.Contains(v, "esc stop search") || strings.Contains(v, "esc close") || strings.Contains(v, "/ search") {
		t.Fatal("help should show stop-search while editing the query")
	}
}

func TestSwitchTabResetsSearch(t *testing.T) {
	quick, assignees := testFilters()
	p := newPicker(quick, assignees, nil)
	press(t, p, "/", "b", "u", "g", "tab", "shift+tab")
	v := view(p)
	if p.Filtering() || !strings.Contains(v, "Recent") || !strings.Contains(v, "All issues") {
		t.Fatal("switching tabs should reset the search and show all options")
	}
}

func TestEmptyTabExplainsItself(t *testing.T) {
	quick, _ := testFilters()
	p := newPicker(quick, nil, nil)
	press(t, p, "tab")
	if v := view(p); !strings.Contains(v, "No assignees on this board.") || !strings.Contains(v, "All issues") {
		t.Fatalf("an empty list should explain why it is empty and allow clearing:\n%s", v)
	}
}
