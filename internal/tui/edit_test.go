package tui

import (
	"errors"
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
	fb.board.Assignees = []jira.Filter{
		{Name: "Alice", AssigneeID: "a"},
		{Name: "Bob", AssigneeID: "b"},
		{Name: "Unassigned"},
	}
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

func TestAssignReloadsBoard(t *testing.T) {
	m, fb := issueModel(t)
	m = pressDrain(t, m, "a")
	if c := m.popup.(*panePopup).pane.(*assignPane).cursor; c != 1 {
		t.Errorf("cursor should start on the current assignee, got %d", c)
	}
	m = pressDrain(t, m, "k", "enter")
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

func TestUnassign(t *testing.T) {
	m, fb := issueModel(t)
	m = pressDrain(t, m, "a", "j", "enter")
	if fb.assigned[0] != "T-2:" || !strings.Contains(m.status, "Unassigned T-2") {
		t.Fatalf("assigned = %v, status = %q", fb.assigned, m.status)
	}
}

func TestAssignErrorKeepsPopupOpen(t *testing.T) {
	m, fb := issueModel(t)
	fb.assignErr = errors.New("no permission")
	m = pressDrain(t, m, "a", "enter")
	if m.popup == nil || !strings.Contains(m.View().Content, "no permission") {
		t.Fatal("failed assign should leave the popup open with the error")
	}
}
