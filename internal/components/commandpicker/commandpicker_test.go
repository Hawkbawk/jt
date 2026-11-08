package commandpicker

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/config"
	"github.com/hawkbawk/jt/internal/tui/popup"
)

func pickerKey(s string) tea.KeyPressMsg {
	if s == "enter" {
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	}
	if s == "esc" {
		return tea.KeyPressMsg{Code: tea.KeyEsc}
	}
	return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
}

func startedPicker(commands []config.Command) *Picker {
	p := New(commands)
	p.Init()
	p.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	return p
}

func settle(p *Picker, cmd tea.Cmd) tea.Msg {
	queue := []tea.Cmd{cmd}
	for steps := 0; len(queue) > 0 && steps < 100; steps++ {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		switch msg := c().(type) {
		case tea.BatchMsg:
			queue = append(queue, msg...)
		case popup.CloseMsg:
			return msg
		default:
			_, next := p.Update(msg)
			queue = append(queue, next)
		}
	}
	return nil
}

func TestPickerListsAndSelectsCommand(t *testing.T) {
	p := startedPicker([]config.Command{
		{Name: "Open ticket", Executable: "code"},
		{Executable: "gh"},
	})
	if v := p.View().Content; !strings.Contains(v, "Open ticket") || !strings.Contains(v, "gh") {
		t.Fatalf("picker view = %q", v)
	}
	_, cmd := p.Update(pickerKey("enter"))
	if cmd == nil {
		t.Fatal("enter did not select a command")
	}
	result := settle(p, cmd)
	msg, ok := result.(popup.CloseMsg)
	if !ok {
		t.Fatalf("selection returned %T, want popup.CloseMsg", result)
	}
	selected, ok := msg.Then.(SelectedMsg)
	if !ok || selected.Command.Executable != "code" {
		t.Fatalf("selected = %#v, want the first command", msg.Then)
	}
}

func TestPickerExplainsWhenNoCommandsAreConfigured(t *testing.T) {
	p := startedPicker(nil)
	if !strings.Contains(p.View().Content, "No commands configured.") {
		t.Fatal("empty picker should explain that no commands are configured")
	}
	_, cmd := p.Update(pickerKey("esc"))
	if cmd == nil {
		t.Fatal("esc should close the empty picker")
	}
	if _, ok := cmd().(popup.CloseMsg); !ok {
		t.Fatalf("esc returned %T, want popup.CloseMsg", cmd())
	}
}
