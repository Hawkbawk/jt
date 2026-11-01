package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/jira"
)

type fakeBackend struct {
	defaultProject string
	projects       []jira.Project
	board          *jira.Board
	boardErr       error
	detail         *jira.IssueDetail
	transitions    []jira.Transition
	moveErr        error
	moved          []string
	assignErr      error
	assigned       []string
	comments       []string
	commentErr     error
	boardCalls     []string
	filterCalls    []string
	editFields     []jira.EditField
	editErr        error
	edited         [][]jira.EditField
	issueTypes     []string
	users          []jira.Option
	searches       []string
	searchErr      error
}

func (f *fakeBackend) DefaultProject() string { return f.defaultProject }

func (f *fakeBackend) Projects() ([]jira.Project, error) { return f.projects, nil }

func (f *fakeBackend) IssueTypes() ([]string, error) { return f.issueTypes, nil }

func (f *fakeBackend) Board(project, filterJQL string) (*jira.Board, error) {
	f.boardCalls = append(f.boardCalls, project)
	f.filterCalls = append(f.filterCalls, filterJQL)
	if f.boardErr != nil {
		return nil, f.boardErr
	}
	b := *f.board
	b.Project = project
	return &b, nil
}

func (f *fakeBackend) Issue(string) (*jira.IssueDetail, error) { return f.detail, nil }

func (f *fakeBackend) Transitions(jira.Issue) ([]jira.Transition, error) { return f.transitions, nil }

func (f *fakeBackend) Transition(key, id string) error {
	if f.moveErr != nil {
		return f.moveErr
	}
	f.moved = append(f.moved, key+":"+id)
	return nil
}

func (f *fakeBackend) Assign(key, id string) error {
	if f.assignErr != nil {
		return f.assignErr
	}
	f.assigned = append(f.assigned, key+":"+id)
	return nil
}

func (f *fakeBackend) AddComment(key, body string) error {
	if f.commentErr != nil {
		return f.commentErr
	}
	f.comments = append(f.comments, key+":"+body)
	return nil
}

func (f *fakeBackend) EditFields(jira.Issue) ([]jira.EditField, error) { return f.editFields, nil }

func (f *fakeBackend) Edit(_ string, fields []jira.EditField) error {
	if f.editErr != nil {
		return f.editErr
	}
	f.edited = append(f.edited, fields)
	return nil
}

func (f *fakeBackend) SearchUsers(_, query string) ([]jira.Option, error) {
	f.searches = append(f.searches, query)
	return f.users, f.searchErr
}

func (f *fakeBackend) IssueURL(key string) string { return "https://jira.example.com/browse/" + key }

func keyMsg(s string) tea.KeyPressMsg {
	switch s {
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	case "ctrl+d":
		return tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl}
	case "ctrl+p":
		return tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl}
	case "ctrl+s":
		return tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
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
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	default:
		return tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
	}
}

func newTestModel(t *testing.T, defaultProject string, fb *fakeBackend) Model {
	t.Helper()
	fb.defaultProject = defaultProject
	if fb.board == nil {
		fb.board = testBoard()
	}
	m := New(fb, &fakeSettings{})
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

func loadedModel(t *testing.T) (Model, *fakeBackend) {
	t.Helper()
	fb := &fakeBackend{}
	m := newTestModel(t, "TEST", fb)
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
	m.actions.copyText = func(s string) error { copied = s; return nil }
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
	m.actions.openURL = func(s string) error { opened = s; return nil }
	_, cmd := press(t, m, "o")
	drain(t, m, cmd)
	if opened != "https://jira.example.com/browse/T-1" {
		t.Fatalf("opened %q", opened)
	}
}

func TestActionErrorShowsInStatus(t *testing.T) {
	m, _ := loadedModel(t)
	m.actions.copyText = func(string) error { return errors.New("no clipboard") }
	m, cmd := press(t, m, "y")
	m = drain(t, m, cmd)
	if !strings.Contains(m.status, "no clipboard") {
		t.Fatalf("status %q", m.status)
	}
}

func TestYankAndOpenIgnoreEmptyColumn(t *testing.T) {
	m, _ := loadedModel(t)
	m.actions.copyText = func(string) error { t.Fatal("copy called"); return nil }
	m.actions.openURL = func(string) error { t.Fatal("open called"); return nil }
	if _, cmd := press(t, m, "l", "y", "o"); cmd != nil {
		t.Fatal("expected no command in an empty column")
	}
}

func TestCtrlCPromptsThenQuitsOnSecondPress(t *testing.T) {
	m, _ := loadedModel(t)
	m, cmd := press(t, m, "ctrl+c")
	if isQuit(cmd) || !m.quit.confirming {
		t.Fatal("first ctrl+c should prompt, not quit")
	}
	if _, cmd = press(t, m, "ctrl+c"); !isQuit(cmd) {
		t.Fatal("second ctrl+c should quit")
	}
}

func TestQuitPromptAnswers(t *testing.T) {
	m, _ := loadedModel(t)
	m, _ = press(t, m, "ctrl+c", "j")
	if !m.quit.confirming || m.board.nav.row != 0 {
		t.Fatal("keys other than y/n should be ignored while prompting")
	}
	m, _ = press(t, m, "n")
	if m.quit.confirming {
		t.Fatal("n should dismiss the prompt")
	}
	m, _ = press(t, m, "ctrl+c")
	if _, cmd := press(t, m, "y"); !isQuit(cmd) {
		t.Fatal("y should quit")
	}
}

func TestStartupLoadsDefaultProjectBoard(t *testing.T) {
	m, fb := loadedModel(t)
	if m.board.nav == nil || m.board.loading || len(fb.boardCalls) != 1 || fb.boardCalls[0] != "TEST" {
		t.Fatalf("board not loaded: board=%v calls=%v", m.board.nav != nil, fb.boardCalls)
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

func TestBoardErrorIsShown(t *testing.T) {
	fb := &fakeBackend{boardErr: errors.New("boom")}
	m := newTestModel(t, "TEST", fb)
	m = drain(t, m, m.Init())
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
	fb := &fakeBackend{}
	m := newTestModel(t, "TEST", fb)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 40})
	m = drain(t, next.(Model), m.Init())

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
	fb.board.Columns[0].Issues = append([]jira.Issue{{Key: "T-0", Summary: "new"}}, fb.board.Columns[0].Issues...)
	m = drain(t, m, cmd)
	if len(fb.boardCalls) != 2 {
		t.Fatalf("expected 2 board fetches, got %v", fb.boardCalls)
	}
	if got := m.board.nav.selected().Key; got != "T-2" {
		t.Fatalf("focus should stay on T-2 after refresh, got %s", got)
	}
}

func TestRefreshIgnoredWithoutProject(t *testing.T) {
	m := newTestModel(t, "", &fakeBackend{})
	m.picker.open = false
	if _, cmd := press(t, m, "r"); cmd != nil {
		t.Fatal("r without a project should do nothing")
	}
}

type fakeSettings struct {
	saved []string
	err   error
}

func (f *fakeSettings) SaveTheme(id string) error {
	f.saved = append(f.saved, id)
	return f.err
}
