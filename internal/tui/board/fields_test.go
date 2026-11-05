package board

import (
	"os"
	"strings"
	"testing"

	"github.com/hawkbawk/jt/internal/components/usersearch"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/tui/tuitest"
)

func TestMain(m *testing.M) {
	usersearch.Delay = 0
	os.Exit(m.Run())
}

func testFields() []jira.EditField {
	return []jira.EditField{{ID: "summary", Name: "Summary", Kind: jira.FieldText, Required: true, Text: "Fix it"}}
}

func typeText(t *testing.T, m Model, s string) Model {
	t.Helper()
	for _, r := range s {
		m = tuitest.PressDrain(t, m, string(r))
	}
	return m
}

func TestEditSaveClosesAndReloadsBoard(t *testing.T) {
	m, fb := issueModel(t)
	m = tuitest.PressDrain(t, m, "e", "enter")
	m = typeText(t, m, "!")
	m = tuitest.PressDrain(t, m, "enter", "ctrl+s")
	if len(fb.Edited) != 1 || m.Popup.IsOpen() || len(fb.BoardCalls) != 2 || !strings.Contains(m.Status, "Updated 1 field on T-2") {
		t.Fatalf("popup should close and the board reload, status = %q", m.Status)
	}
}

func TestEditCtrlDQuitsWithoutAsking(t *testing.T) {
	m, _ := issueModel(t)
	m = tuitest.PressDrain(t, m, "e", "enter")
	m = typeText(t, m, "!")
	if _, cmd := press(t, m, "ctrl+d"); !tuitest.IsQuit(cmd) {
		t.Fatal("ctrl+d must quit immediately, unsaved changes or not")
	}
}
