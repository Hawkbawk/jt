package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/hawkbawk/jt/internal/jira"
)

var searchNow = time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)

func newTestSearch(t *testing.T, fb *fakeBackend) SearchModel {
	t.Helper()
	m := NewSearch(fb, &fakeSettings{})
	m.now = func() time.Time { return searchNow }
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return next.(SearchModel)
}

// searchSend feeds msgs to the model one at a time, then returns it and the
// last command.
func searchSend(m SearchModel, msgs ...tea.Msg) (SearchModel, tea.Cmd) {
	var cmd tea.Cmd
	for _, msg := range msgs {
		var next tea.Model
		next, cmd = m.Update(msg)
		m = next.(SearchModel)
	}
	return m, cmd
}

func searchPress(m SearchModel, ks ...string) SearchModel {
	for _, k := range ks {
		m, _ = searchSend(m, keyMsg(k))
	}
	return m
}

// focusField presses tab until the field has focus.
func focusField(m SearchModel, field int) SearchModel {
	for m.form.focus != field {
		m = searchPress(m, "tab")
	}
	return m
}

func typeInto(m SearchModel, field int, text string) SearchModel {
	return searchType(focusField(m, field), text)
}

func searchType(m SearchModel, text string) SearchModel {
	for _, r := range text {
		m, _ = searchSend(m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	return m
}

func TestSearchStartsOnDefaultProject(t *testing.T) {
	m := newTestSearch(t, &fakeBackend{defaultProject: "DEMO"})
	q, err := m.form.query(searchNow)
	if err != nil {
		t.Fatal(err)
	}
	want := jira.SearchQuery{Project: "DEMO", Sort: jira.SortUpdated, Descending: true}
	if q.JQL() != want.JQL() {
		t.Errorf("JQL = %q, want %q", q.JQL(), want.JQL())
	}
	if !strings.Contains(m.render(), `JQL: project = "DEMO" ORDER BY updated DESC`) {
		t.Errorf("preview missing from view:\n%s", m.render())
	}
}

func TestSearchLoadsIssueTypes(t *testing.T) {
	fb := &fakeBackend{issueTypes: []string{"Bug", "Task"}}
	m := newTestSearch(t, fb)
	msg := loadIssueTypes(fb)()
	m, _ = searchSend(m, msg)

	m = focusField(m, fieldType)
	m = searchPress(m, "right", "right")
	q, _ := m.form.query(searchNow)
	if q.IssueType != "Task" {
		t.Errorf("IssueType = %q, want Task", q.IssueType)
	}
	m = searchPress(m, "right")
	if q, _ = m.form.query(searchNow); q.IssueType != "" {
		t.Errorf("IssueType = %q after wrapping to Any", q.IssueType)
	}
}

func TestSearchFormFields(t *testing.T) {
	m := newTestSearch(t, &fakeBackend{})
	m = focusField(m, fieldSprint)
	m = searchPress(m, "right")
	m = focusField(m, fieldOrder)
	m = searchPress(m, "left")
	m = typeInto(m, fieldKey, "demo-1, demo-2")
	m = typeInto(m, fieldCreatedFrom, "2026-03-01")
	m = typeInto(m, fieldCreatedTo, "2026-03-02")
	m = typeInto(m, fieldUpdatedFrom, "-1d")

	q, err := m.form.query(searchNow)
	if err != nil {
		t.Fatal(err)
	}
	want := `sprint in openSprints() AND key in (DEMO-1, DEMO-2) AND created >= "2026-03-01 00:00" AND created <= "2026-03-02 23:59" AND updated >= "2026-03-09 12:00" ORDER BY updated ASC`
	if q.JQL() != want {
		t.Errorf("JQL = %q, want %q", q.JQL(), want)
	}
}

func TestSearchFormErrors(t *testing.T) {
	m := newTestSearch(t, &fakeBackend{})
	m = typeInto(m, fieldKey, "nope")
	if _, err := m.form.query(searchNow); err == nil || !strings.Contains(err.Error(), "Issue key") {
		t.Errorf("err = %v, want an issue key error", err)
	}
	if !strings.Contains(m.render(), "Error: Issue key") {
		t.Error("error missing from view")
	}

	m = newTestSearch(t, &fakeBackend{})
	m = typeInto(m, fieldUpdatedTo, "soon")
	if _, err := m.form.query(searchNow); err == nil || !strings.Contains(err.Error(), "Updated to") {
		t.Errorf("err = %v, want an updated to error", err)
	}
}

func TestSearchPicksProject(t *testing.T) {
	fb := &fakeBackend{defaultProject: "DEMO", projects: []jira.Project{{Key: "DEMO"}, {Key: "OPS"}}}
	m := newTestSearch(t, fb)

	m, cmd := searchSend(m, keyMsg("enter"))
	m, _ = searchSend(m, cmd())
	if !m.picker.open {
		t.Fatal("project picker didn't open")
	}
	m, _ = searchSend(m, projectsLoadedMsg{projects: fb.projects})
	m = searchPress(m, "down")
	m, cmd = searchSend(m, keyMsg("enter"))
	m, _ = searchSend(m, cmd())
	if q, _ := m.form.query(searchNow); q.Project != "OPS" {
		t.Errorf("Project = %q, want OPS", q.Project)
	}

	m = searchPress(m, "backspace")
	if q, _ := m.form.query(searchNow); q.Project != "" {
		t.Errorf("Project = %q after clearing", q.Project)
	}
}

func TestSearchQuitKeys(t *testing.T) {
	m := newTestSearch(t, &fakeBackend{})
	if _, cmd := searchSend(m, keyMsg("ctrl+d")); cmd == nil {
		t.Error("ctrl+d should quit")
	}
	m, _ = searchSend(m, keyMsg("ctrl+c"))
	if !m.quit.confirming {
		t.Error("ctrl+c should ask to confirm")
	}
	// Typing q in a text field must not quit.
	m = newTestSearch(t, &fakeBackend{})
	m = typeInto(m, fieldKey, "q")
	if m.quit.confirming || m.form.fields[fieldKey].value() != "q" {
		t.Error("q should be typed into the field")
	}
}

func TestSearchAdvancedMode(t *testing.T) {
	m := newTestSearch(t, &fakeBackend{defaultProject: "DEMO"})
	m = searchPress(m, "ctrl+t")
	if !m.advanced || m.jql.value() != `project = "DEMO" ORDER BY updated DESC` {
		t.Fatalf("advanced = %v, jql = %q", m.advanced, m.jql.value())
	}
	if !strings.Contains(m.render(), "advanced") {
		t.Error("header doesn't show the mode")
	}

	// Keys go to the editor, so q is typed rather than quitting.
	m = searchType(m, " q")
	if !strings.HasSuffix(m.jql.value(), " q") || m.quit.confirming {
		t.Errorf("jql = %q", m.jql.value())
	}

	// What was written survives a trip through the form.
	m = searchPress(m, "ctrl+t", "ctrl+t")
	if m.jql.value() != `project = "DEMO" ORDER BY updated DESC q` {
		t.Errorf("jql = %q after switching back", m.jql.value())
	}
	m = searchPress(m, "ctrl+t")
	if m.advanced {
		t.Error("should be back in basic mode")
	}
}

func TestSearchAdvancedStartsEmptyWhenFormInvalid(t *testing.T) {
	m := newTestSearch(t, &fakeBackend{})
	m = typeInto(m, fieldKey, "nope")
	m = searchPress(m, "ctrl+t")
	if m.jql.value() != "" {
		t.Errorf("jql = %q, want empty", m.jql.value())
	}
}

func searchDrain(t *testing.T, m SearchModel, cmd tea.Cmd) SearchModel {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for steps := 0; len(queue) > 0; steps++ {
		if steps > maxSteps {
			t.Fatal("too many commands, likely a message loop")
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
		var next tea.Model
		next, c = m.Update(msg)
		m = next.(SearchModel)
		queue = append(queue, c)
	}
	return m
}

func searchPressDrain(t *testing.T, m SearchModel, ks ...string) SearchModel {
	t.Helper()
	for _, k := range ks {
		next, cmd := m.Update(keyMsg(k))
		m = searchDrain(t, next.(SearchModel), cmd)
	}
	return m
}

func foundIssues() []jira.Issue {
	return []jira.Issue{
		{Key: "DEMO-1", Summary: "First", Status: "To Do", Assignee: jira.Option{ID: "u1", Name: "Ada"}},
		{Key: "DEMO-2", Summary: "Second", Status: "Done"},
		{Key: "DEMO-3", Summary: "Third", Status: "In Progress"},
	}
}

func searched(t *testing.T) (SearchModel, *fakeBackend) {
	t.Helper()
	fb := &fakeBackend{defaultProject: "DEMO", found: foundIssues()}
	m := newTestSearch(t, fb)
	return searchPressDrain(t, m, "ctrl+s"), fb
}

func TestSearchRunsFormAndFocusesResults(t *testing.T) {
	m, fb := searched(t)
	if len(fb.findCalls) != 1 || fb.findCalls[0] != `project = "DEMO" ORDER BY updated DESC` {
		t.Fatalf("searches = %q", fb.findCalls)
	}
	if !m.resultsFocused || len(m.results.issues) != 3 {
		t.Fatalf("resultsFocused = %v, issues = %d", m.resultsFocused, len(m.results.issues))
	}
	view := m.render()
	for _, want := range []string{"Results (3)", "DEMO-1", "Second", "› DEMO-1"} {
		if !strings.Contains(ansi.Strip(view), want) {
			t.Errorf("view is missing %q:\n%s", want, ansi.Strip(view))
		}
	}

	m = searchPress(m, "j", "j", "j")
	if got := m.results.selected().Key; got != "DEMO-3" {
		t.Errorf("selected = %s, want DEMO-3", got)
	}
	m = searchPress(m, "g")
	if got := m.results.selected().Key; got != "DEMO-1" {
		t.Errorf("selected = %s after g, want DEMO-1", got)
	}
}

func TestSearchResultsBackToForm(t *testing.T) {
	m, _ := searched(t)
	m = searchPress(m, "q")
	if m.quit.confirming {
		t.Fatal("q in results should quit, not confirm")
	}
	m = searchPress(m, "esc")
	if m.resultsFocused {
		t.Fatal("esc should return to the form")
	}
	// Tab from the last field goes back down to the results.
	m = focusField(m, fieldCount-1)
	m = searchPress(m, "tab")
	if !m.resultsFocused {
		t.Error("tab from the last field should focus the results")
	}
}

func TestSearchResultsQuit(t *testing.T) {
	m, _ := searched(t)
	if _, cmd := m.Update(keyMsg("q")); !isQuit(cmd) {
		t.Error("q should quit from the results")
	}
}

func TestSearchAdvancedSubmit(t *testing.T) {
	fb := &fakeBackend{found: foundIssues()}
	m := newTestSearch(t, fb)
	m = searchPress(m, "ctrl+t")
	m.jql.setValue("")
	m = searchPressDrain(t, m, "enter")
	if len(fb.findCalls) != 0 || !strings.Contains(m.status, "write some JQL") {
		t.Fatalf("searches = %q, status = %q", fb.findCalls, m.status)
	}

	m = searchType(m, "assignee = currentUser()")
	m = searchPressDrain(t, m, "enter")
	if len(fb.findCalls) != 1 || !strings.Contains(fb.findCalls[0], "currentUser()") {
		t.Errorf("searches = %q", fb.findCalls)
	}
	if !m.resultsFocused {
		t.Error("results should have focus")
	}
}

func TestSearchShowsErrorsAndEmptyResults(t *testing.T) {
	fb := &fakeBackend{findErr: errors.New("bad JQL")}
	m := searchPressDrain(t, newTestSearch(t, fb), "ctrl+s")
	if !strings.Contains(ansi.Strip(m.render()), "Error: bad JQL") || m.resultsFocused {
		t.Errorf("view:\n%s", ansi.Strip(m.render()))
	}

	fb.findErr = nil
	m = searchPressDrain(t, m, "ctrl+s")
	if !strings.Contains(ansi.Strip(m.render()), "No issues found.") || m.resultsFocused {
		t.Errorf("view:\n%s", ansi.Strip(m.render()))
	}
}

func TestSearchIgnoresStaleResults(t *testing.T) {
	m, _ := searched(t)
	m.searchID = 5
	m, _ = searchSend(m, searchLoadedMsg{id: 4, issues: nil})
	if len(m.results.issues) != 3 {
		t.Error("an older search replaced the newer results")
	}
}

func TestSearchRefreshKeepsFocusedIssue(t *testing.T) {
	m, fb := searched(t)
	m = searchPress(m, "j")
	m = searchPressDrain(t, m, "r")
	if len(fb.findCalls) != 2 || fb.findCalls[0] != fb.findCalls[1] {
		t.Errorf("searches = %q", fb.findCalls)
	}
	if got := m.results.selected().Key; got != "DEMO-2" {
		t.Errorf("selected = %s, want DEMO-2", got)
	}
}

func TestSearchIssueActions(t *testing.T) {
	m, fb := searched(t)
	fb.detail = &jira.IssueDetail{Issue: foundIssues()[0], Description: "Details here"}
	fb.transitions = []jira.Transition{{ID: "31", Name: "Done", To: "Done"}}

	m = searchPressDrain(t, m, "v")
	if m.popup == nil || !strings.Contains(ansi.Strip(m.render()), "Details here") {
		t.Fatalf("details popup didn't open:\n%s", ansi.Strip(m.render()))
	}
	m = searchPress(m, "esc")
	if m.popup != nil {
		t.Fatal("esc should close the popup")
	}

	// Moving an issue searches again and keeps the cursor on it.
	m = searchPress(m, "j")
	m = searchPressDrain(t, m, "m", "enter")
	if len(fb.moved) != 1 || fb.moved[0] != "DEMO-2:31" {
		t.Fatalf("moved = %q", fb.moved)
	}
	if m.popup != nil || len(fb.findCalls) != 2 || m.results.selected().Key != "DEMO-2" {
		t.Errorf("popup = %v, searches = %q, selected = %v", m.popup, fb.findCalls, m.results.selected())
	}
	if !strings.Contains(m.status, "Moved DEMO-2 to Done") {
		t.Errorf("status = %q", m.status)
	}
}

func TestSearchYankUsesSelectedIssue(t *testing.T) {
	m, _ := searched(t)
	var copied string
	m.actions.copyText = func(s string) error { copied = s; return nil }
	m = searchPress(m, "j")
	_, cmd := searchSend(m, keyMsg("y"))
	cmd()
	if copied != "https://jira.example.com/browse/DEMO-2" {
		t.Errorf("copied = %q", copied)
	}
}
