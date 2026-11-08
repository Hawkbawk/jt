package commandpicker

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/hawkbawk/jt/internal/config"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/tui/popup"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
	"github.com/hawkbawk/jt/internal/tui/tuitest"
)

func startedPicker(t *testing.T, commands []config.Command) *Picker {
	t.Helper()
	p := New(commands, jira.Issue{Key: "ABC-1"}, "ABC", "Team")
	p = tuitest.Drain(t, p, p.Init())
	next, _ := p.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	return next.(*Picker)
}

func TestPickerListsAndSelectsCommand(t *testing.T) {
	p := startedPicker(t, []config.Command{
		{Name: "Open ticket", Command: "code"},
		{Command: "gh"},
	})
	if v := ansi.Strip(p.View().Content); !strings.Contains(v, "Open ticket") || !strings.Contains(v, "gh") ||
		!strings.Contains(v, "enter run") || !strings.Contains(v, "esc close") {
		t.Fatalf("picker view = %q", v)
	}
	next, cmd := p.Update(tuitest.Key("enter"))
	if cmd == nil {
		t.Fatal("enter did not select a command")
	}
	_, messages := tuitest.DrainUntil(t, next, cmd, func(msg tea.Msg) bool {
		_, ok := msg.(popup.CloseMsg)
		return ok
	})
	var msg popup.CloseMsg
	found := false
	for _, result := range messages {
		if closed, ok := result.(popup.CloseMsg); ok {
			msg = closed
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no popup close message in %v", messages)
	}
	selected, ok := msg.Then.(tuimsg.CommandSelected)
	if !ok || selected.Command.Command != "code" || selected.Issue.Key != "ABC-1" ||
		selected.Project != "ABC" || selected.Board != "Team" {
		t.Fatalf("selected = %#v, want the first command", msg.Then)
	}
}

func TestPickerExplainsWhenNoCommandsAreConfigured(t *testing.T) {
	p := startedPicker(t, nil)
	if !strings.Contains(ansi.Strip(p.View().Content), "No commands configured.") {
		t.Fatal("empty picker should explain that no commands are configured")
	}
	next, cmd := p.Update(tuitest.Key("esc"))
	if cmd == nil {
		t.Fatal("esc should close the empty picker")
	}
	_, messages := tuitest.DrainUntil(t, next, cmd, func(msg tea.Msg) bool {
		_, ok := msg.(popup.CloseMsg)
		return ok
	})
	if len(messages) == 0 {
		t.Fatal("esc returned no message, want popup.CloseMsg")
	}
	if _, ok := messages[0].(popup.CloseMsg); !ok {
		t.Fatalf("esc returned %T, want popup.CloseMsg", messages[0])
	}
}
