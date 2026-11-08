package board

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/config"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/tui/tuitest"
)

func newTestModel(t *testing.T, defaultProject string, fb *tuitest.Backend) Model {
	t.Helper()
	fb.Project = defaultProject
	if fb.BoardData == nil {
		fb.BoardData = testBoard()
	}
	m := New(fb, &tuitest.Settings{})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return next.(Model)
}

// press sends keys in order and returns the final model and last command.
func press(t *testing.T, m Model, keys ...string) (Model, tea.Cmd) {
	t.Helper()
	var cmd tea.Cmd
	for _, k := range keys {
		var next tea.Model
		next, cmd = m.Update(tuitest.Key(k))
		m = next.(Model)
	}
	return m, cmd
}

// pressOpen presses k, which opens a popup, and applies the message that opens
// it. It returns the model with the popup showing and the popup's own commands,
// which haven't run yet.
func pressOpen(t *testing.T, m Model, k string) (Model, tea.Cmd) {
	t.Helper()
	m, cmd := press(t, m, k)
	if cmd == nil {
		t.Fatalf("%s should open a popup", k)
	}
	next, cmd := m.Update(cmd())
	return next.(Model), cmd
}

func loadedModel(t *testing.T) (Model, *tuitest.Backend) {
	t.Helper()
	fb := &tuitest.Backend{}
	m := newTestModel(t, "TEST", fb)
	return tuitest.Drain(t, m, m.Init()), fb
}

func TestCtrlDQuitsImmediately(t *testing.T) {
	m, _ := loadedModel(t)
	if _, cmd := press(t, m, "ctrl+d"); !tuitest.IsQuit(cmd) {
		t.Fatal("ctrl+d did not quit")
	}
}

func TestQQuitsFromBoard(t *testing.T) {
	m, _ := loadedModel(t)
	if _, cmd := press(t, m, "q"); !tuitest.IsQuit(cmd) {
		t.Fatal("q did not quit from the board")
	}
}

func TestQClosesPopupInsteadOfQuitting(t *testing.T) {
	m, _ := loadedModel(t)
	m = tuitest.PressDrain(t, m, "v")
	if !m.Popup.IsOpen() {
		t.Fatal("v should open a popup")
	}
	m, cmd := press(t, m, "q")
	if tuitest.IsQuit(cmd) {
		t.Fatal("q in a popup should not quit")
	}
	if m = tuitest.Drain(t, m, cmd); m.Popup.IsOpen() {
		t.Fatal("q in a popup should close it")
	}
}

func TestYankCopiesIssueURL(t *testing.T) {
	m, _ := loadedModel(t)
	var copied string
	m.actions.CopyText = func(s string) error { copied = s; return nil }
	m, cmd := press(t, m, "j", "y")
	m = tuitest.Drain(t, m, cmd)
	if copied != "https://jira.example.com/browse/T-2" {
		t.Fatalf("copied %q", copied)
	}
	if !strings.Contains(m.Status, "T-2") {
		t.Fatalf("status %q", m.Status)
	}
}

func TestOpenOpensIssueURL(t *testing.T) {
	m, _ := loadedModel(t)
	var opened string
	m.actions.OpenURL = func(s string) error { opened = s; return nil }
	_, cmd := press(t, m, "o")
	tuitest.Drain(t, m, cmd)
	if opened != "https://jira.example.com/browse/T-1" {
		t.Fatalf("opened %q", opened)
	}
}

func TestActionErrorShowsInStatus(t *testing.T) {
	m, _ := loadedModel(t)
	m.actions.CopyText = func(string) error { return errors.New("no clipboard") }
	m, cmd := press(t, m, "y")
	m = tuitest.Drain(t, m, cmd)
	if !strings.Contains(m.Status, "no clipboard") {
		t.Fatalf("status %q", m.Status)
	}
}

func TestYankAndOpenIgnoreEmptyColumn(t *testing.T) {
	m, _ := loadedModel(t)
	m.actions.CopyText = func(string) error { t.Fatal("copy called"); return nil }
	m.actions.OpenURL = func(string) error { t.Fatal("open called"); return nil }
	if _, cmd := press(t, m, "l", "y", "o"); cmd != nil {
		t.Fatal("expected no command in an empty column")
	}
}

func TestCtrlCPromptsThenQuitsOnSecondPress(t *testing.T) {
	m, _ := loadedModel(t)
	m, cmd := press(t, m, "ctrl+c")
	if tuitest.IsQuit(cmd) || !m.Quit.Confirming {
		t.Fatal("first ctrl+c should prompt, not quit")
	}
	if _, cmd = press(t, m, "ctrl+c"); !tuitest.IsQuit(cmd) {
		t.Fatal("second ctrl+c should quit")
	}
}

func TestQuitPromptAnswers(t *testing.T) {
	m, _ := loadedModel(t)
	m, _ = press(t, m, "ctrl+c", "j")
	if !m.Quit.Confirming || m.board.nav.row != 0 {
		t.Fatal("keys other than y/n should be ignored while prompting")
	}
	m, _ = press(t, m, "n")
	if m.Quit.Confirming {
		t.Fatal("n should dismiss the prompt")
	}
	m, _ = press(t, m, "ctrl+c")
	if _, cmd := press(t, m, "y"); !tuitest.IsQuit(cmd) {
		t.Fatal("y should quit")
	}
}

func TestStartupLoadsDefaultProjectBoard(t *testing.T) {
	m, fb := loadedModel(t)
	if m.board.nav == nil || m.board.loading || len(fb.BoardCalls) != 1 || fb.BoardCalls[0] != "TEST" {
		t.Fatalf("board not loaded: board=%v calls=%v", m.board.nav != nil, fb.BoardCalls)
	}
}

func TestBoardKeysMoveFocus(t *testing.T) {
	m, _ := loadedModel(t)
	m, _ = press(t, m, "j", "j", "l", "l")
	if got := m.board.nav.selected(); got == nil || got.Key != "T-4" {
		t.Fatalf("selected %v", got)
	}
	m, _ = press(t, m, "h", "h", "j")
	if got := m.board.nav.selected().Key; got != "T-2" {
		t.Fatalf("selected %s", got)
	}
}

func TestCommandsKeyOpensConfiguredCommandPicker(t *testing.T) {
	m, _ := loadedModel(t)
	m.commands = []config.Command{{Name: "Open issue", Executable: "code"}}
	m = tuitest.PressDrain(t, m, "c")
	if !m.Popup.IsOpen() || !strings.Contains(m.Popup.View(), "Open issue") {
		t.Fatalf("c should open the configured command picker: %s", m.Popup.View())
	}
}

func TestCommandsKeyDoesNothingWithoutSelectedIssue(t *testing.T) {
	m, _ := loadedModel(t)
	m.commands = []config.Command{{Name: "Open issue", Executable: "code"}}
	m, cmd := press(t, m, "l", "c")
	if m.Popup.IsOpen() || cmd != nil {
		t.Fatal("c opened a command picker without a selected issue")
	}
}

func TestCommandLoadsTicketContextBeforeRunning(t *testing.T) {
	m, fb := loadedModel(t)
	fb.Detail = &jira.IssueDetail{
		Issue:       jira.Issue{Key: "T-1", Summary: "ticket title"},
		Description: "ticket description",
	}
	fb.CommentList = &jira.CommentList{Items: []jira.Comment{{Author: "Ada", Body: "comment"}}}
	result := m.loadCommand(config.Command{Executable: "true"})().(commandLoadedMsg)
	if result.err != nil {
		t.Fatal(result.err)
	}
	if result.issue.Key != "T-1" || result.project != "TEST" || result.board != "Test Board" ||
		result.url != "https://jira.example.com/browse/T-1" || result.title != "ticket title" ||
		result.description != "ticket description" || len(result.comments) != 1 || result.comments[0].Body != "comment" {
		t.Fatalf("loaded command context = %#v", result)
	}
	if _, cmd := m.Update(result); cmd == nil {
		t.Fatal("loaded command did not produce an executable process")
	}
}

func TestBoardErrorIsShown(t *testing.T) {
	fb := &tuitest.Backend{BoardErr: errors.New("boom")}
	m := newTestModel(t, "TEST", fb)
	m = tuitest.Drain(t, m, m.Init())
	if m.board.err == nil || m.board.nav != nil {
		t.Fatal("expected error state")
	}
}

func TestStaleBoardIsIgnored(t *testing.T) {
	m, _ := loadedModel(t)
	next, _ := m.Update(boardLoadedMsg{project: "OTHER", board: &jira.Board{Name: "stale"}})
	if next.(Model).board.nav.board.Name == "stale" {
		t.Fatal("board for a different project replaced the current one")
	}
}

func TestViewRendersColumnsAndCards(t *testing.T) {
	m, _ := loadedModel(t)
	v := m.View().Content
	for _, want := range []string{"Open (3)", "In Progress (0)", "T-1", "Test Board"} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q", want)
		}
	}
}

func TestNarrowTerminalShowsScrollArrowsAsFocusMoves(t *testing.T) {
	fb := &tuitest.Backend{}
	m := newTestModel(t, "TEST", fb)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 40})
	m = tuitest.Drain(t, next.(Model), m.Init())

	v := m.View().Content
	if strings.Contains(v, "Closed") || !strings.Contains(v, "▶") || strings.Contains(v, "◀") {
		t.Fatalf("first page should hide Closed and only point right:\n%s", v)
	}

	m, _ = press(t, m, "l", "l")
	v = m.View().Content
	if !strings.Contains(v, "Closed") || !strings.Contains(v, "◀") || strings.Contains(v, "▶") {
		t.Fatalf("last page should show Closed and only point left:\n%s", v)
	}
}

func TestRefreshReloadsBoardAndKeepsFocus(t *testing.T) {
	m, fb := loadedModel(t)
	m, cmd := press(t, m, "j", "r")
	if !m.board.loading || m.board.nav == nil {
		t.Fatal("r should reload while keeping the current board visible")
	}
	fb.BoardData.Columns[0].Issues = append([]jira.Issue{{Key: "T-0", Summary: "new"}}, fb.BoardData.Columns[0].Issues...)
	m = tuitest.Drain(t, m, cmd)
	if len(fb.BoardCalls) != 2 {
		t.Fatalf("expected 2 board fetches, got %v", fb.BoardCalls)
	}
	if got := m.board.nav.selected().Key; got != "T-2" {
		t.Fatalf("focus should stay on T-2 after refresh, got %s", got)
	}
}

func TestRefreshIgnoredWithoutProject(t *testing.T) {
	m := newTestModel(t, "", &tuitest.Backend{})
	if _, cmd := press(t, m, "r"); cmd != nil {
		t.Fatal("r without a project should do nothing")
	}
}
