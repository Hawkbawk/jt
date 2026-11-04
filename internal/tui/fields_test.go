package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/hawkbawk/jt/internal/components/usersearch"
	"github.com/hawkbawk/jt/internal/jira"
)

func TestMain(m *testing.M) {
	usersearch.Delay = 0
	os.Exit(m.Run())
}

func testFields() []jira.EditField {
	return []jira.EditField{{ID: "summary", Name: "Summary", Kind: jira.FieldText, Required: true, Text: "Fix it"}}
}

func typeText(t *testing.T, m BoardModel, s string) BoardModel {
	t.Helper()
	for _, r := range s {
		m = pressDrain(t, m, string(r))
	}
	return m
}

func TestEditSaveClosesAndReloadsBoard(t *testing.T) {
	m, fb := issueModel(t)
	m = pressDrain(t, m, "e", "enter")
	m = typeText(t, m, "!")
	m = pressDrain(t, m, "enter", "ctrl+s")
	if len(fb.edited) != 1 || m.popup.IsOpen() || len(fb.boardCalls) != 2 || !strings.Contains(m.status, "Updated 1 field on T-2") {
		t.Fatalf("popup should close and the board reload, status = %q", m.status)
	}
}

func TestEditCtrlDQuitsWithoutAsking(t *testing.T) {
	m, _ := issueModel(t)
	m = pressDrain(t, m, "e", "enter")
	m = typeText(t, m, "!")
	if _, cmd := press(t, m, "ctrl+d"); !isQuit(cmd) {
		t.Fatal("ctrl+d must quit immediately, unsaved changes or not")
	}
}
