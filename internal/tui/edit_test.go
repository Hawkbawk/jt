package tui

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/hawkbawk/jt/internal/jira"
)

// issueModel returns a loaded board with T-2 selected.
func issueModel(t *testing.T) (Model, *fakeBackend) {
	t.Helper()
	fb := &fakeBackend{
		board:       testBoard(),
		transitions: []jira.Transition{{ID: "11", Name: "Start Work", To: "In Progress"}},
		editFields:  testFields(),
	}
	fb.board.Columns[0].Issues[1].Assignee = "Bob"
	fb.board.Columns[0].Issues[1].AssigneeID = "b"
	m := newTestModel(t, "TEST", fb)
	m = drain(t, m, m.Init())
	m, _ = press(t, m, "j")
	return m, fb
}

func TestIssueKeysOpenSeparatePopups(t *testing.T) {
	m, _ := issueModel(t)
	for key, title := range map[string]string{"m": "Move · T-2", "a": "Assign · T-2", "e": "Edit · T-2"} {
		opened := pressDrain(t, m, key)
		if _, ok := opened.popup.(*panePopup); !ok || !strings.Contains(opened.View().Content, title) {
			t.Errorf("%s should open the %q popup", key, title)
		}
	}
}

func TestMove(t *testing.T) {
	m, fb := issueModel(t)
	m = pressDrain(t, m, "m")
	if !strings.Contains(m.View().Content, "Move to In Progress") {
		t.Fatal("move popup should list transitions")
	}
	m = pressDrain(t, m, "enter")
	if len(fb.moved) != 1 || fb.moved[0] != "T-2:11" || m.popup != nil {
		t.Fatalf("moved = %v, popup = %v", fb.moved, m.popup)
	}
}

func TestMoveErrorKeepsPopupOpen(t *testing.T) {
	m, fb := issueModel(t)
	fb.moveErr = errors.New("workflow says no")
	m = pressDrain(t, m, "m", "enter")
	if m.popup == nil || !strings.Contains(m.View().Content, "workflow says no") || len(fb.boardCalls) != 1 {
		t.Fatal("failed move should leave the popup open with the error and skip the reload")
	}
}

func TestAssignSearchesForPeople(t *testing.T) {
	m, fb := issueModel(t)
	fb.users = []jira.Option{{ID: "a", Name: "Alice"}}
	m = pressDrain(t, m, "a")
	if v := m.View().Content; !strings.Contains(v, "Bob") || !strings.Contains(v, "(none)") {
		t.Fatalf("expected the current assignee and a way to unassign:\n%s", v)
	}
	m = typeText(t, m, "al")
	if !slices.Equal(fb.searches, []string{"a", "al"}) {
		t.Fatalf("searches = %v", fb.searches)
	}
	m = pressDrain(t, m, "enter", "enter")
	if len(fb.assigned) != 1 || fb.assigned[0] != "T-2:a" {
		t.Fatalf("assigned = %v", fb.assigned)
	}
	if m.popup != nil || len(fb.boardCalls) != 2 {
		t.Fatal("popup should close and the board reload")
	}
	if !strings.Contains(m.status, "Assigned T-2 to Alice") {
		t.Fatalf("status = %q", m.status)
	}
}

func TestAssignKeysTypeIntoSearch(t *testing.T) {
	m, fb := issueModel(t)
	m = pressDrain(t, m, "a")
	m = typeText(t, m, "qjk")
	if m.popup == nil || fb.searches[len(fb.searches)-1] != "qjk" {
		t.Fatalf("typing should go to the search, searches = %v", fb.searches)
	}
}

func TestUnassign(t *testing.T) {
	m, fb := issueModel(t)
	m = pressDrain(t, m, "a", "enter", "up", "enter")
	if len(fb.assigned) != 1 || fb.assigned[0] != "T-2:" || !strings.Contains(m.status, "Unassigned T-2") {
		t.Fatalf("assigned = %v, status = %q", fb.assigned, m.status)
	}
}

func TestAssignErrorKeepsPopupOpenForRetry(t *testing.T) {
	m, fb := issueModel(t)
	fb.assignErr = errors.New("no permission")
	m = pressDrain(t, m, "a", "enter", "enter")
	if m.popup == nil || !strings.Contains(m.View().Content, "no permission") {
		t.Fatal("failed assign should leave the popup open with the error")
	}
	fb.assignErr = nil
	m = pressDrain(t, m, "enter", "enter")
	if len(fb.assigned) != 1 || fb.assigned[0] != "T-2:b" || m.popup != nil {
		t.Fatalf("retry should assign, got %v", fb.assigned)
	}
}
