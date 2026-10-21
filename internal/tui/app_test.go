package tui

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ryanhawkins/jt/internal/config"
	"github.com/ryanhawkins/jt/internal/jira"
)

type fakeBackend struct {
	projects    []jira.Project
	board       *jira.Board
	boardErr    error
	detail      *jira.IssueDetail
	transitions []jira.Transition
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
	f.moved = append(f.moved, key+":"+id)
	return nil
}

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

// drain runs cmd and feeds resulting messages back into the model until
// no more work is produced. Spinner ticks are skipped to avoid sleeping.
func drain(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		msg := c()
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

func loadedModel(t *testing.T) (Model, *fakeBackend) {
	t.Helper()
	fb := &fakeBackend{}
	m := newTestModel(t, &config.Config{DefaultProject: "TEST"}, fb)
	return drain(t, m, m.Init()), fb
}

func TestCtrlDQuitsImmediately(t *testing.T) {
	m, _ := loadedModel(t)
	if _, cmd := press(t, m, "ctrl+d"); !isQuit(cmd) {
		t.Fatal("ctrl+d did not quit")
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
	if !m.confirmQuit || m.nav.rows[0] != 0 {
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
	m, _ = press(t, m, "h", "h", "k")
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
