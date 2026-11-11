package standalone

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/components/issuedetail"
	"github.com/hawkbawk/jt/internal/components/issueedit"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
	"github.com/hawkbawk/jt/internal/tui/tuitest"
)

func sized(t *testing.T, content tea.Model) Model {
	t.Helper()
	next, _ := New(content).Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return next.(Model)
}

func detailModel(t *testing.T) Model {
	t.Helper()
	detail := &jira.IssueDetail{Key: "DEMO-1", Summary: "Fix the thing"}
	b := &tuitest.Backend{Detail: detail}
	return sized(t, issuedetail.NewLoaded(b, detail, func(md string, _ int) string { return md }))
}

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestShowsLoadedContentWithTitle(t *testing.T) {
	m := detailModel(t)
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

func TestQuitsWhenContentCloses(t *testing.T) {
	m := detailModel(t)
	if _, cmd := m.Update(tuimsg.Closed{}); !isQuit(cmd) {
		t.Error("Closed should quit")
	}
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl}); !isQuit(cmd) {
		t.Error("ctrl+d should quit")
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("esc should close")
	}
	if _, cmd = m.Update(cmd()); !isQuit(cmd) {
		t.Error("closing the content should quit")
	}
}

func TestQuitsWhenContentFinishes(t *testing.T) {
	m := sized(t, issueedit.New(&tuitest.Backend{}, jira.Issue{Key: "DEMO-1"}))
	if _, cmd := m.Update(issueedit.Edited{Key: "DEMO-1", Count: 1}); !isQuit(cmd) {
		t.Error("Edited should quit")
	}
	if !strings.Contains(m.View().Content, "DEMO-1") {
		t.Error("view should show the edit title")
	}
}

func TestCtrlCAsksBeforeQuitting(t *testing.T) {
	m := detailModel(t)
	next, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if isQuit(cmd) {
		t.Fatal("first ctrl+c should ask")
	}
	if _, cmd = next.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}); !isQuit(cmd) {
		t.Error("second ctrl+c should quit")
	}
}
