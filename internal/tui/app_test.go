package tui

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/hawkbawk/jt/internal/config"
	"github.com/hawkbawk/jt/internal/jira"
)

type fakeBackend struct {
	projects    []jira.Project
	board       *jira.Board
	boardErr    error
	detail      *jira.IssueDetail
	transitions []jira.Transition
	moveErr     error
	moved       []string
	boardCalls  []string
}

func (f *fakeBackend) Projects() ([]jira.Project, error) { return f.projects, nil }

func (f *fakeBackend) Board(project string, _ int) (*jira.Board, error) {
	f.boardCalls = append(f.boardCalls, project)
	if f.boardErr != nil {
		return nil, f.boardErr
	}
	b := *f.board
	b.Project = project
	return &b, nil
}

func (f *fakeBackend) Issue(string) (*jira.IssueDetail, error) { return f.detail, nil }

func (f *fakeBackend) Transitions(string) ([]jira.Transition, error) { return f.transitions, nil }

func (f *fakeBackend) Transition(key, id string) error {
	if f.moveErr != nil {
		return f.moveErr
	}
	f.moved = append(f.moved, key+":"+id)
	return nil
}

func (f *fakeBackend) IssueURL(key string) string { return "https://jira.example.com/browse/" + key }

func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	case "ctrl+d":
		return tea.KeyMsg{Type: tea.KeyCtrlD}
	case "ctrl+p":
		return tea.KeyMsg{Type: tea.KeyCtrlP}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func newTestModel(t *testing.T, cfg *config.Config, fb *fakeBackend) Model {
	t.Helper()
	if fb.board == nil {
		fb.board = testBoard()
	}
	m := New(fb, cfg, filepath.Join(t.TempDir(), "config.json"))
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return next.(Model)
}

// press sends keys in order and returns the final model and last command.
func press(t *testing.T, m Model, keys ...string) (Model, tea.Cmd) {
	t.Helper()
	var cmd tea.Cmd
	for _, k := range keys {
		var next tea.Model
		next, cmd = m.Update(keyMsg(k))
		m = next.(Model)
	}
	return m, cmd
}

// pressDrain sends keys in order, running each resulting command to completion.
func pressDrain(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, k := range keys {
		next, cmd := m.Update(keyMsg(k))
		m = drain(t, next.(Model), cmd)
	}
	return m
}

// cmdDeadline bounds how long drain waits on a single command. Timers such as
// spinner ticks and cursor blinks take longer than this and are dropped, which
// keeps self-rescheduling commands from looping forever.
const (
	cmdDeadline = 50 * time.Millisecond
	maxSteps    = 1000
)

func runCmd(c tea.Cmd) (tea.Msg, bool) {
	done := make(chan tea.Msg, 1)
	go func() { done <- c() }()
	select {
	case msg := <-done:
		return msg, true
	case <-time.After(cmdDeadline):
		return nil, false
	}
}

// drain runs cmd and feeds resulting messages back into the model until
// no more immediate work is produced.
func drain(t *testing.T, m Model, cmd tea.Cmd) Model {
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
		msg, ok := runCmd(c)
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
		m = next.(Model)
		queue = append(queue, nc)
	}
	return m
}

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func testConfig() *config.Config {
	return &config.Config{DefaultProject: "TEST"}
}

func loadedModel(t *testing.T) (Model, *fakeBackend) {
	t.Helper()
	fb := &fakeBackend{}
	m := newTestModel(t, testConfig(), fb)
	return drain(t, m, m.Init()), fb
}

func TestCtrlDQuitsImmediately(t *testing.T) {
	m, _ := loadedModel(t)
	if _, cmd := press(t, m, "ctrl+d"); !isQuit(cmd) {
		t.Fatal("ctrl+d did not quit")
	}
}

func TestQQuitsFromBoard(t *testing.T) {
	m, _ := loadedModel(t)
	if _, cmd := press(t, m, "q"); !isQuit(cmd) {
		t.Fatal("q did not quit from the board")
	}
}

func TestQClosesPopupInsteadOfQuitting(t *testing.T) {
	m, _ := loadedModel(t)
	m, _ = press(t, m, "v")
	if m.popup == nil {
		t.Fatal("v should open a popup")
	}
	m, cmd := press(t, m, "q")
	if isQuit(cmd) || m.popup != nil {
		t.Fatal("q in a popup should close it, not quit")
	}
}

func TestYankCopiesIssueURL(t *testing.T) {
	m, _ := loadedModel(t)
	var copied string
	m.copyText = func(s string) error { copied = s; return nil }
	m, cmd := press(t, m, "j", "y")
	m = drain(t, m, cmd)
	if copied != "https://jira.example.com/browse/T-2" {
		t.Fatalf("copied %q", copied)
	}
	if !strings.Contains(m.status, "T-2") {
		t.Fatalf("status %q", m.status)
	}
}

func TestOpenOpensIssueURL(t *testing.T) {
	m, _ := loadedModel(t)
	var opened string
	m.openURL = func(s string) error { opened = s; return nil }
	_, cmd := press(t, m, "o")
	drain(t, m, cmd)
	if opened != "https://jira.example.com/browse/T-1" {
		t.Fatalf("opened %q", opened)
	}
}

func TestActionErrorShowsInStatus(t *testing.T) {
	m, _ := loadedModel(t)
	m.copyText = func(string) error { return errors.New("no clipboard") }
	m, cmd := press(t, m, "y")
	m = drain(t, m, cmd)
	if !strings.Contains(m.status, "no clipboard") {
		t.Fatalf("status %q", m.status)
	}
}

func TestYankAndOpenIgnoreEmptyColumn(t *testing.T) {
	m, _ := loadedModel(t)
	m.copyText = func(string) error { t.Fatal("copy called"); return nil }
	m.openURL = func(string) error { t.Fatal("open called"); return nil }
	if _, cmd := press(t, m, "l", "y", "o"); cmd != nil {
		t.Fatal("expected no command in an empty column")
	}
}

func TestCtrlCPromptsThenQuitsOnSecondPress(t *testing.T) {
	m, _ := loadedModel(t)
	m, cmd := press(t, m, "ctrl+c")
	if isQuit(cmd) || !m.confirmQuit {
		t.Fatal("first ctrl+c should prompt, not quit")
	}
	if _, cmd = press(t, m, "ctrl+c"); !isQuit(cmd) {
		t.Fatal("second ctrl+c should quit")
	}
}

func TestQuitPromptAnswers(t *testing.T) {
	m, _ := loadedModel(t)
	m, _ = press(t, m, "ctrl+c", "j")
	if !m.confirmQuit || m.nav.row != 0 {
		t.Fatal("keys other than y/n should be ignored while prompting")
	}
	m, _ = press(t, m, "n")
	if m.confirmQuit {
		t.Fatal("n should dismiss the prompt")
	}
	m, _ = press(t, m, "ctrl+c")
	if _, cmd := press(t, m, "y"); !isQuit(cmd) {
		t.Fatal("y should quit")
	}
}

func TestStartupLoadsDefaultProjectBoard(t *testing.T) {
	m, fb := loadedModel(t)
	if !m.hasBoard || m.loading || len(fb.boardCalls) != 1 || fb.boardCalls[0] != "TEST" {
		t.Fatalf("board not loaded: hasBoard=%v calls=%v", m.hasBoard, fb.boardCalls)
	}
}

func TestBoardKeysMoveFocus(t *testing.T) {
	m, _ := loadedModel(t)
	m, _ = press(t, m, "j", "j", "l", "l")
	if got := m.nav.selected(); got == nil || got.Key != "T-4" {
		t.Fatalf("selected %v", got)
	}
	m, _ = press(t, m, "h", "h", "j")
	if got := m.nav.selected().Key; got != "T-2" {
		t.Fatalf("selected %s", got)
	}
}

func TestBoardErrorIsShown(t *testing.T) {
	fb := &fakeBackend{boardErr: errors.New("boom")}
	m := newTestModel(t, &config.Config{DefaultProject: "TEST"}, fb)
	m = drain(t, m, m.Init())
	if m.err == nil || m.hasBoard {
		t.Fatal("expected error state")
	}
}

func TestStaleBoardIsIgnored(t *testing.T) {
	m, _ := loadedModel(t)
	next, _ := m.Update(boardLoadedMsg{project: "OTHER", board: &jira.Board{Name: "stale"}})
	if next.(Model).nav.board.Name == "stale" {
		t.Fatal("board for a different project replaced the current one")
	}
}

func TestViewRendersColumnsAndCards(t *testing.T) {
	m, _ := loadedModel(t)
	v := m.View()
	for _, want := range []string{"Open (3)", "In Progress (0)", "T-1", "Test Board"} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q", want)
		}
	}
}
