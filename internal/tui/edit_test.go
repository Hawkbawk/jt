package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/hawkbawk/jt/internal/jira"
)

func editModel(t *testing.T) (Model, *fakeBackend) {
	t.Helper()
	fb := &fakeBackend{
		board:       testBoard(),
		transitions: []jira.Transition{{ID: "11", Name: "Start Work", To: "In Progress"}},
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
	m, cmd := press(t, m, "e")
	return drain(t, m, cmd), fb
}

func TestEditPopupSwitchesBetweenMoveAndAssignee(t *testing.T) {
	m, _ := editModel(t)
	if _, ok := m.popup.(*editPopup); !ok {
		t.Fatal("e should open the edit popup")
	}
	v := m.View()
	if !strings.Contains(v, "Move to In Progress") || strings.Contains(v, "Alice") {
		t.Error("move tab should show first")
	}
	m, _ = press(t, m, "tab")
	v = m.View()
	if !strings.Contains(v, "Alice") || strings.Contains(v, "Move to In Progress") {
		t.Error("tab should switch to assignees")
	}
	if c := m.popup.(*editPopup).tabs[1].pane.(*assignPane).cursor; c != 1 {
		t.Errorf("cursor should start on the current assignee, got %d", c)
	}
}

func TestEditMoveTab(t *testing.T) {
	m, fb := editModel(t)
	m = pressDrain(t, m, "enter")
	if len(fb.moved) != 1 || fb.moved[0] != "T-2:11" || m.popup != nil {
		t.Fatalf("moved = %v, popup = %v", fb.moved, m.popup)
	}
}

func TestEditAssignReloadsBoard(t *testing.T) {
	m, fb := editModel(t)
	m = pressDrain(t, m, "tab", "k", "enter")
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

func TestEditUnassign(t *testing.T) {
	m, fb := editModel(t)
	m = pressDrain(t, m, "tab", "j", "enter")
	if fb.assigned[0] != "T-2:" || !strings.Contains(m.status, "Unassigned T-2") {
		t.Fatalf("assigned = %v, status = %q", fb.assigned, m.status)
	}
}

func TestEditAssignErrorKeepsPopupOpen(t *testing.T) {
	m, fb := editModel(t)
	fb.assignErr = errors.New("no permission")
	m = pressDrain(t, m, "tab", "enter")
	if m.popup == nil || !strings.Contains(m.View(), "no permission") {
		t.Fatal("failed assign should leave the popup open with the error")
	}
}
