package edit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/components/issueedit"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
	"github.com/hawkbawk/jt/internal/tui/tuitest"
)

func newModel(t *testing.T) Model {
	t.Helper()
	m := New(&tuitest.Backend{}, jira.Issue{Key: "DEMO-1", Summary: "Fix the thing"})
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

func TestEditShowsTitleAndLoadsFields(t *testing.T) {
	m := newModel(t)
	if m.Init() == nil {
		t.Error("Init should load the fields")
	}
	if out := m.View().Content; !strings.Contains(out, "DEMO-1") {
		t.Errorf("view is missing the issue key:\n%s", out)
	}
	if !m.View().AltScreen {
		t.Error("view should use the alternate screen")
	}
}

func TestEditQuitsWhenContentFinishes(t *testing.T) {
	for _, msg := range []tea.Msg{tuimsg.Closed{}, issueedit.Edited{Key: "DEMO-1", Count: 1}} {
		if _, cmd := newModel(t).Update(msg); !isQuit(cmd) {
			t.Errorf("%T should quit", msg)
		}
	}
	if _, cmd := newModel(t).Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl}); !isQuit(cmd) {
		t.Error("ctrl+d should quit")
	}
}
