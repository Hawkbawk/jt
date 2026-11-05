package tuitest

import (
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

// cmdDeadline bounds how long Drain waits on a single command. Timers such as
// spinner ticks and cursor blinks take longer than this and are dropped, which
// keeps self-rescheduling commands from looping forever.
const (
	cmdDeadline = 50 * time.Millisecond
	maxSteps    = 1000
)

// Key returns the key press for s, such as "ctrl+s", "esc" or "j".
func Key(s string) tea.KeyPressMsg {
	if strings.HasPrefix(s, "ctrl+") && len(s) == len("ctrl+x") {
		return tea.KeyPressMsg{Code: rune(s[len(s)-1]), Mod: tea.ModCtrl}
	}
	switch s {
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEsc}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case " ":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	default:
		return tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
	}
}

// RunCmd runs c and reports its message, or false when it takes longer than
// the deadline.
func RunCmd(c tea.Cmd) (tea.Msg, bool) {
	done := make(chan tea.Msg, 1)
	go func() { done <- c() }()
	select {
	case msg := <-done:
		return msg, true
	case <-time.After(cmdDeadline):
		return nil, false
	}
}

// IsQuit reports whether cmd quits the program.
func IsQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

// Drain runs cmd and feeds resulting messages back into the model until no
// more immediate work is produced.
func Drain[M tea.Model](t *testing.T, m M, cmd tea.Cmd) M {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for steps := 0; len(queue) > 0; steps++ {
		if steps > maxSteps {
			t.Fatal("drain: too many commands, likely a message loop")
		}
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		msg, ok := RunCmd(c)
		if !ok {
			continue
		}
		switch msg := msg.(type) {
		case tea.BatchMsg:
			queue = append(queue, msg...)
			continue
		case spinner.TickMsg:
			continue
		}
		next, nc := m.Update(msg)
		m = next.(M)
		queue = append(queue, nc)
	}
	return m
}

// PressDrain sends keys in order, running each resulting command to completion.
func PressDrain[M tea.Model](t *testing.T, m M, keys ...string) M {
	t.Helper()
	for _, k := range keys {
		next, cmd := m.Update(Key(k))
		m = Drain(t, next.(M), cmd)
	}
	return m
}
