package board

import (
	"errors"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/tui/modal"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
	"github.com/hawkbawk/jt/internal/tui/tuitest"
)

// issueModel returns a loaded board with T-2 selected.
func issueModel(t *testing.T) (Model, *tuitest.Backend) {
	t.Helper()
	fb := &tuitest.Backend{
		BoardData:      testBoard(),
		TransitionList: []jira.Transition{{ID: "11", Name: "Start Work", To: "In Progress"}},
		EditFieldList:  testFields(),
	}
	fb.BoardData.Columns[0].Issues[1].Assignee = jira.Option{ID: "b", Name: "Bob"}
	m := newTestModel(t, "TEST", fb)
	m = tuitest.Drain(t, m, m.Init())
	m, _ = press(t, m, "j")
	return m, fb
}

func TestIssueKeysOpenSeparateModals(t *testing.T) {
	m, _ := issueModel(t)
	for key, title := range map[string]string{"m": "Move · T-2", "a": "Assign · T-2", "e": "Edit · T-2"} {
		opened := tuitest.PressDrain(t, m, key)
		if !opened.Modal.IsOpen() || !strings.Contains(opened.View().Content, title) {
			t.Errorf("%s should open the %q modal", key, title)
		}
	}
}

func TestMove(t *testing.T) {
	m, fb := issueModel(t)
	m = tuitest.PressDrain(t, m, "m")
	if !strings.Contains(m.View().Content, "Move to In Progress") {
		t.Fatal("move modal should list transitions")
	}
	m = tuitest.PressDrain(t, m, "enter")
	if len(fb.Moved) != 1 || fb.Moved[0] != "T-2:11" || m.Modal.IsOpen() {
		t.Fatalf("moved = %v, modal = %v", fb.Moved, m.Modal.Content())
	}
}

func TestMoveErrorKeepsModalOpen(t *testing.T) {
	m, fb := issueModel(t)
	fb.MoveErr = errors.New("workflow says no")
	m = tuitest.PressDrain(t, m, "m", "enter")
	if !m.Modal.IsOpen() || !strings.Contains(m.View().Content, "workflow says no") || len(fb.BoardCalls) != 1 {
		t.Fatal("failed move should leave the modal open with the error and skip the reload")
	}
}

func TestAssignSearchesForPeople(t *testing.T) {
	m, fb := issueModel(t)
	fb.Users = []jira.Option{{ID: "a", Name: "Alice"}}
	m = tuitest.PressDrain(t, m, "a")
	if v := m.View().Content; !strings.Contains(v, "Bob") || !strings.Contains(v, "(none)") {
		t.Fatalf("expected the current assignee and a way to unassign:\n%s", v)
	}
	m = typeText(t, m, "al")
	if !slices.Equal(fb.UserQueries, []string{"a", "al"}) {
		t.Fatalf("searches = %v", fb.UserQueries)
	}
	m = tuitest.PressDrain(t, m, "enter", "enter")
	if len(fb.Assigned) != 1 || fb.Assigned[0] != "T-2:a" {
		t.Fatalf("assigned = %v", fb.Assigned)
	}
	if m.Modal.IsOpen() || len(fb.BoardCalls) != 2 {
		t.Fatal("modal should close and the board reload")
	}
	if !strings.Contains(m.Status, "Assigned T-2 to Alice") {
		t.Fatalf("status = %q", m.Status)
	}
}

func TestAssignKeysTypeIntoSearch(t *testing.T) {
	m, fb := issueModel(t)
	m = tuitest.PressDrain(t, m, "a")
	m = typeText(t, m, "qjk")
	if !m.Modal.IsOpen() || fb.UserQueries[len(fb.UserQueries)-1] != "qjk" {
		t.Fatalf("typing should go to the search, searches = %v", fb.UserQueries)
	}
}

func TestUnassign(t *testing.T) {
	m, fb := issueModel(t)
	m = tuitest.PressDrain(t, m, "a", "enter", "up", "enter")
	if len(fb.Assigned) != 1 || fb.Assigned[0] != "T-2:" || !strings.Contains(m.Status, "Unassigned T-2") {
		t.Fatalf("assigned = %v, status = %q", fb.Assigned, m.Status)
	}
}

func TestAssignErrorKeepsModalOpenForRetry(t *testing.T) {
	m, fb := issueModel(t)
	fb.AssignErr = errors.New("no permission")
	m = tuitest.PressDrain(t, m, "a", "enter", "enter")
	if !m.Modal.IsOpen() || !strings.Contains(m.View().Content, "no permission") {
		t.Fatal("failed assign should leave the modal open with the error")
	}
	fb.AssignErr = nil
	m = tuitest.PressDrain(t, m, "enter", "enter")
	if len(fb.Assigned) != 1 || fb.Assigned[0] != "T-2:b" || m.Modal.IsOpen() {
		t.Fatalf("retry should assign, got %v", fb.Assigned)
	}
}

func TestCreateOpensOnTheBoardsProject(t *testing.T) {
	m, fb := issueModel(t)
	fb.ProjectList = []jira.Project{{Key: "TEST", Name: "Test"}}
	fb.CreateTypes = map[string][]jira.IssueType{"TEST": {{ID: "1", Name: "Task"}}}
	fb.CreateFieldList = []jira.IssueField{{ID: "summary", Name: "Summary", Kind: jira.FieldText, Required: true}}
	fb.CreatedKey = "TEST-9"

	m = tuitest.PressDrain(t, m, "n")
	if !m.Modal.IsOpen() || !strings.Contains(m.View().Content, "Select a project") {
		t.Fatalf("n should open the create modal on the project list")
	}
	m = tuitest.PressDrain(t, m, "enter")
	if !strings.Contains(m.View().Content, "Issue type") || !slices.Equal(fb.CreateTypeCalls, []string{"TEST"}) {
		t.Fatalf("n should open the create modal for TEST, calls = %v", fb.CreateTypeCalls)
	}

	m = tuitest.PressDrain(t, m, "enter", "enter", "N", "e", "w", "enter", "ctrl+s")
	if m.Modal.IsOpen() || len(fb.Creates) != 1 || fb.Creates[0].Fields[0].Text != "New" {
		t.Fatalf("creates = %+v, modal open = %v", fb.Creates, m.Modal.IsOpen())
	}
	if !strings.Contains(m.Status, "Created TEST-9") {
		t.Errorf("status = %q", m.Status)
	}
}

type recorder struct{ got []tea.Msg }

func (r *recorder) Init() tea.Cmd { return nil }

func (r *recorder) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	r.got = append(r.got, msg)
	return r, nil
}

func (r *recorder) View() tea.View { return tea.NewView("") }

type stray struct{}

func TestMessagesTheScreenDoesNotHandleReachTheModal(t *testing.T) {
	m, _ := issueModel(t)
	rec := &recorder{}
	m = tuitest.Drain(t, m, modal.Open(rec))
	tuitest.Drain(t, m, tuimsg.Send(stray{}))
	if !slices.Contains(rec.got, tea.Msg(stray{})) {
		t.Fatalf("modal got %v", rec.got)
	}
}
