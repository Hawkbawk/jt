package tui

import (
	"os"
	"testing"

	"github.com/hawkbawk/jt/internal/config"
	"github.com/hawkbawk/jt/internal/jira"
)

func pickerBackend() *fakeBackend {
	return &fakeBackend{projects: []jira.Project{
		{Key: "ALPHA", Name: "Alpha Project"},
		{Key: "BETA", Name: "Beta Project"},
	}}
}

func TestStartupWithoutDefaultOpensPicker(t *testing.T) {
	fb := pickerBackend()
	m := newTestModel(t, &config.Config{}, fb)
	m = drain(t, m, m.Init())
	if !m.pickerOpen || m.picker.cancellable || len(m.picker.list.Items()) != 2 {
		t.Fatalf("picker not shown: open=%v items=%d", m.pickerOpen, len(m.picker.list.Items()))
	}
	if len(fb.boardCalls) != 0 {
		t.Fatal("board loaded without a project")
	}

	m, _ = press(t, m, "esc")
	if !m.pickerOpen {
		t.Fatal("startup picker should not be dismissable without a board")
	}
}

func TestEnterOpensProjectForSessionOnly(t *testing.T) {
	fb := pickerBackend()
	m := newTestModel(t, &config.Config{}, fb)
	m = drain(t, m, m.Init())

	m, cmd := press(t, m, "j", "enter")
	m = drain(t, m, cmd)
	if m.pickerOpen || m.project != "BETA" || !m.hasBoard {
		t.Fatalf("project not opened: open=%v project=%s", m.pickerOpen, m.project)
	}
	if m.cfg.DefaultProject != "" {
		t.Fatal("enter should not change the default project")
	}
	if _, err := os.Stat(m.cfgPath); !os.IsNotExist(err) {
		t.Fatal("config should not be written on enter")
	}
}

func TestSpaceSetsDefaultAndSaves(t *testing.T) {
	m := newTestModel(t, &config.Config{}, pickerBackend())
	m = drain(t, m, m.Init())

	m, _ = press(t, m, "j", " ")
	if !m.pickerOpen {
		t.Fatal("space should keep the picker open")
	}
	saved, err := config.Load(m.cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if saved.DefaultProject != "BETA" || m.cfg.DefaultProject != "BETA" {
		t.Fatalf("default = %q (saved %q)", m.cfg.DefaultProject, saved.DefaultProject)
	}
	if item := m.picker.list.Items()[1].(projectItem); !item.isDefault {
		t.Fatal("default marker not shown")
	}
	if item := m.picker.list.Items()[0].(projectItem); item.isDefault {
		t.Fatal("non-default project marked as default")
	}
}

func TestCtrlPFromBoardIsCancellable(t *testing.T) {
	fb := pickerBackend()
	fb.board = testBoard()
	m := newTestModel(t, &config.Config{DefaultProject: "TEST"}, fb)
	m = drain(t, m, m.Init())

	m, cmd := press(t, m, "ctrl+p")
	m = drain(t, m, cmd)
	if !m.pickerOpen || !m.picker.cancellable {
		t.Fatal("ctrl+p should open a cancellable picker")
	}
	m, _ = press(t, m, "esc")
	if m.pickerOpen || !m.hasBoard || m.project != "TEST" {
		t.Fatal("esc should return to the current board")
	}

	m, _ = press(t, m, "ctrl+p")
	if !m.picker.loaded {
		t.Fatal("projects should be cached after the first load")
	}
}

func TestSpaceWhileFilteringDoesNotSetDefault(t *testing.T) {
	m := newTestModel(t, &config.Config{}, pickerBackend())
	m = drain(t, m, m.Init())

	m, _ = press(t, m, "/", "b", " ")
	if m.cfg.DefaultProject != "" {
		t.Fatal("space typed into the filter changed the default")
	}
	if !m.picker.filtering() {
		t.Fatal("expected to still be filtering")
	}
}

func TestQDoesNotQuitFromPicker(t *testing.T) {
	m := newTestModel(t, &config.Config{}, pickerBackend())
	m = drain(t, m, m.Init())
	if _, cmd := press(t, m, "q"); isQuit(cmd) {
		t.Fatal("q quit the app from the picker")
	}
}

func TestFilterThenOpen(t *testing.T) {
	m := newTestModel(t, &config.Config{}, pickerBackend())
	m = drain(t, m, m.Init())

	m = pressDrain(t, m, "/", "b", "e", "t", "enter", "enter")
	if m.pickerOpen || m.project != "BETA" {
		t.Fatalf("expected BETA to open from the filtered list, project=%q", m.project)
	}
}

func TestKeysIgnoredWhileProjectsLoad(t *testing.T) {
	m := newTestModel(t, &config.Config{}, pickerBackend())
	m, _ = press(t, m, "/", "x")
	if m.picker.filtering() {
		t.Fatal("filter started before projects loaded")
	}
}
