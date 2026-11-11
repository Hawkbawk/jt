package view

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
	"github.com/hawkbawk/jt/internal/tui/tuitest"
)

func newModel(t *testing.T) Model {
	t.Helper()
	detail := &jira.IssueDetail{Key: "DEMO-1", Summary: "Fix the thing"}
	m := New(&tuitest.Backend{Detail: detail}, detail, func(md string, _ int) string { return md })
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return next.(Model)
}

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestViewShowsLoadedIssue(t *testing.T) {
	m := newModel(t)
	if m.Init() != nil {
		t.Error("Init should not reload an issue that's already loaded")
	}
	out := m.View().Content
	for _, want := range []string{"DEMO-1", "Fix the thing"} {
		if !strings.Contains(out, want) {
			t.Errorf("view is missing %q:\n%s", want, out)
		}
	}
	if !m.View().AltScreen {
		t.Error("view should use the alternate screen")
	}
}

func TestViewQuitsWhenContentCloses(t *testing.T) {
	m := newModel(t)
	_, cmd := m.Update(tuimsg.Closed{})
	if !isQuit(cmd) {
		t.Error("Closed should quit")
	}
	_, cmd = m.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	if !isQuit(cmd) {
		t.Error("ctrl+d should quit")
	}
}

func TestViewCloseKeyQuits(t *testing.T) {
	m := newModel(t)
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("esc should close")
	}
	next, cmd := m.Update(cmd())
	_ = next
	if !isQuit(cmd) {
		t.Error("closing the content should quit")
	}
}
