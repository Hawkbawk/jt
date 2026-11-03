package tui

import (
	"testing"

	"github.com/hawkbawk/jt/internal/components/projectpicker"
	"github.com/hawkbawk/jt/internal/jira"
)

func pickerBackend() *fakeBackend {
	return &fakeBackend{projects: []jira.Project{
		{Key: "ALPHA", Name: "Alpha Project"},
		{Key: "BETA", Name: "Beta Project"},
	}}
}

func projects(t *testing.T, m BoardModel) *projectpicker.Picker {
	t.Helper()
	p, ok := m.popup.Content().(*projectpicker.Picker)
	if !ok {
		t.Fatalf("project popup not open, got %T", m.popup.Content())
	}
	return p
}

func TestStartupWithoutDefaultOpensPicker(t *testing.T) {
	fb := pickerBackend()
	m := newTestModel(t, "", fb)
	m = drain(t, m, m.Init())
	if p := projects(t, m); p.Cancellable() || p.Len() != 2 {
		t.Fatalf("picker not shown: cancellable=%v items=%d", p.Cancellable(), p.Len())
	}
	if len(fb.boardCalls) != 0 {
		t.Fatal("board loaded without a project")
	}

	m = pressDrain(t, m, "esc")
	projects(t, m)
}

func TestEnterOpensProjectForSessionOnly(t *testing.T) {
	fb := pickerBackend()
	m := newTestModel(t, "", fb)
	m = drain(t, m, m.Init())

	m, cmd := press(t, m, "j", "enter")
	m = drain(t, m, cmd)
	if m.popup.IsOpen() || m.board.project != "BETA" || m.board.nav == nil {
		t.Fatalf("project not opened: open=%v project=%s", m.popup.IsOpen(), m.board.project)
	}
}

func TestCtrlPFromBoardIsCancellable(t *testing.T) {
	fb := pickerBackend()
	fb.board = testBoard()
	m := newTestModel(t, "TEST", fb)
	m = drain(t, m, m.Init())

	m = pressDrain(t, m, "ctrl+p")
	if !projects(t, m).Cancellable() {
		t.Fatal("ctrl+p should open a cancellable picker")
	}
	m = pressDrain(t, m, "esc")
	if m.popup.IsOpen() || m.board.nav == nil || m.board.project != "TEST" {
		t.Fatal("esc should return to the current board")
	}

	m, _ = pressOpen(t, m, "ctrl+p")
	if !projects(t, m).Loaded() {
		t.Fatal("projects should be cached after the first load")
	}
}

func TestQDoesNotQuitFromPicker(t *testing.T) {
	m := newTestModel(t, "", pickerBackend())
	m = drain(t, m, m.Init())
	if _, cmd := press(t, m, "q"); isQuit(cmd) {
		t.Fatal("q quit the app from the picker")
	}
}

func TestFilterThenOpen(t *testing.T) {
	m := newTestModel(t, "", pickerBackend())
	m = drain(t, m, m.Init())

	m = pressDrain(t, m, "/", "b", "e", "t", "enter", "enter")
	if m.popup.IsOpen() || m.board.project != "BETA" {
		t.Fatalf("expected BETA to open from the filtered list, project=%q", m.board.project)
	}
}

func TestKeysIgnoredWhileProjectsLoad(t *testing.T) {
	m := newTestModel(t, "", pickerBackend())
	m, _ = pressOpen(t, m, "ctrl+p")
	m, _ = press(t, m, "/", "x")
	if projects(t, m).Filtering() {
		t.Fatal("filter started before projects loaded")
	}
}
