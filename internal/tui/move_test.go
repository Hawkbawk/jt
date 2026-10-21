package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/ryanhawkins/jt/internal/jira"
)

func moveModel(t *testing.T) (Model, *fakeBackend) {
	t.Helper()
	fb := &fakeBackend{transitions: []jira.Transition{
		{ID: "11", Name: "Start Work", To: "In Progress"},
		{ID: "12", Name: "Close - Won't Do", To: "Closed"},
	}}
	m := newTestModel(t, testConfig(), fb)
	m = drain(t, m, m.Init())
	m, _ = press(t, m, "j")
	m, cmd := press(t, m, "m")
	return drain(t, m, cmd), fb
}

func TestMovePopupListsTransitions(t *testing.T) {
	m, _ := moveModel(t)
	p, ok := m.popup.(*movePopup)
	if !ok || !p.loaded || p.key != "T-2" {
		t.Fatal("m should open the move popup for the selected issue")
	}
	v := m.View()
	for _, want := range []string{"Move to In Progress (Start Work)", "Move to Closed (Close - Won't Do)"} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q", want)
		}
	}
}

func TestMoveAppliesTransitionAndReloadsBoard(t *testing.T) {
	m, fb := moveModel(t)
	m, cmd := press(t, m, "j", "enter")
	if !m.popup.(*movePopup).applying {
		t.Fatal("enter should start applying the transition")
	}
	m = drain(t, m, cmd)

	if len(fb.moved) != 1 || fb.moved[0] != "T-2:12" {
		t.Fatalf("moved = %v", fb.moved)
	}
	if m.popup != nil {
		t.Fatal("popup should close after a successful move")
	}
	if len(fb.boardCalls) != 2 {
		t.Fatalf("board should reload, calls = %v", fb.boardCalls)
	}
	if sel := m.nav.selected(); sel == nil || sel.Key != "T-2" {
		t.Fatalf("focus should follow the moved issue, got %v", sel)
	}
	if !strings.Contains(m.status, "Moved T-2 to Closed") {
		t.Fatalf("status = %q", m.status)
	}
}

func TestMoveErrorKeepsPopupOpen(t *testing.T) {
	m, fb := moveModel(t)
	fb.moveErr = errors.New("workflow says no")
	m, cmd := press(t, m, "enter")
	m = drain(t, m, cmd)

	p, ok := m.popup.(*movePopup)
	if !ok || p.err == nil || p.applying {
		t.Fatal("failed move should leave the popup open with the error")
	}
	if !strings.Contains(m.View(), "workflow says no") {
		t.Fatal("error not shown")
	}
	if len(fb.boardCalls) != 1 {
		t.Fatal("board should not reload after a failed move")
	}
}

func TestMoveCursorClampsAndEscCloses(t *testing.T) {
	m, _ := moveModel(t)
	m, _ = press(t, m, "k", "j", "j", "j")
	if c := m.popup.(*movePopup).cursor; c != 1 {
		t.Fatalf("cursor = %d", c)
	}
	m, _ = press(t, m, "esc")
	if m.popup != nil {
		t.Fatal("esc should close the popup")
	}
}
