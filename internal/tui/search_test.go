package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/hawkbawk/jt/internal/jira"
)

var searchNow = time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)

func newTestSearch(t *testing.T, fb *fakeBackend) SearchModel {
	t.Helper()
	m := NewSearch(fb, &fakeSettings{})
	m.form.now = func() time.Time { return searchNow }
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = searchDrain(t, next.(SearchModel), cmd)
	return searchDrain(t, m, m.Init())
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

// focusField presses tab until the field has focus.
func focusField(t *testing.T, m SearchModel, field string) SearchModel {
	t.Helper()
	for i := 0; m.form.focused() != field; i++ {
		if i >= len(m.form.fields) {
			t.Fatalf("could not focus %s; focused %s", field, m.form.focused())
		}
		m = searchPressDrain(t, m, "tab")
	}
	return m
}

func typeInto(t *testing.T, m SearchModel, field, text string) SearchModel {
	t.Helper()
	return searchType(t, focusField(t, m, field), text)
}

func searchType(t *testing.T, m SearchModel, text string) SearchModel {
	t.Helper()
	if m.popup != nil {
		for _, r := range text {
			next, cmd := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
			m = searchDrain(t, next.(SearchModel), cmd)
		}
		return m
	}
	next, cmd := m.Update(tea.PasteMsg{Content: text})
	return searchDrain(t, next.(SearchModel), cmd)
}

func TestSearchStartsOnDefaultProject(t *testing.T) {
	t.Parallel()
	m := newTestSearch(t, &fakeBackend{defaultProject: "DEMO"})
	q, err := m.form.query(searchNow)
	if err != nil {
		t.Fatal(err)
	}
	want := jira.SearchQuery{Project: "DEMO", Sort: jira.SortUpdated, Descending: true}
	if q.String() != want.String() {
		t.Errorf("JQL = %q, want %q", q.String(), want.String())
	}
	if !strings.Contains(m.render(), `project = "DEMO" ORDER BY updated DESC`) {
		t.Errorf("preview missing from view:\n%s", m.render())
	}
}

func TestSearchLoadsIssueTypes(t *testing.T) {
	t.Parallel()
	fb := &fakeBackend{issueTypes: []string{"Bug", "Task"}}
	m := newTestSearch(t, fb)
	msg := loadIssueTypes(fb)()
	m, _ = searchSend(m, msg)

	m = focusField(t, m, fieldType)
	m = searchPressDrain(t, m, "right", "right")
	q, _ := m.form.query(searchNow)
	if q.IssueType != "Task" {
		t.Errorf("IssueType = %q, want Task", q.IssueType)
	}
	m = searchPressDrain(t, m, "right")
	if q, _ = m.form.query(searchNow); q.IssueType != "" {
		t.Errorf("IssueType = %q after wrapping to Any", q.IssueType)
	}
}

func TestSearchFormFields(t *testing.T) {
	t.Parallel()
	m := newTestSearch(t, &fakeBackend{})
	m = focusField(t, m, fieldSprint)
	m = searchPressDrain(t, m, "right")
	m = focusField(t, m, fieldOrder)
	m = searchPressDrain(t, m, "left")
	m = typeInto(t, m, fieldKey, "demo-1, demo-2")
	m = typeInto(t, m, fieldCreatedFrom, "2026-03-01")
	m = typeInto(t, m, fieldCreatedTo, "2026-03-02")
	m = typeInto(t, m, fieldUpdatedFrom, "-1d")

	q, err := m.form.query(searchNow)
	if err != nil {
		t.Fatal(err)
	}
	want := `sprint in openSprints() AND key in (DEMO-1, DEMO-2) AND created >= "2026-03-01 00:00" AND created <= "2026-03-02 23:59" AND updated >= "2026-03-09 12:00" ORDER BY updated ASC`
	if q.String() != want {
		t.Errorf("JQL = %q, want %q", q.String(), want)
	}
}

func TestSearchFormErrors(t *testing.T) {
	t.Parallel()
	m := newTestSearch(t, &fakeBackend{})
	m = typeInto(t, m, fieldKey, "nope")
	if _, err := m.form.query(searchNow); err == nil || !strings.Contains(err.Error(), "Issue key") {
		t.Errorf("err = %v, want an issue key error", err)
	}
	m = searchPressDrain(t, m, "ctrl+s")
	if !strings.Contains(m.status, "Issue key") {
		t.Error("error missing from view")
	}

	m = newTestSearch(t, &fakeBackend{})
	m = typeInto(t, m, fieldUpdatedTo, "soon")
	if _, err := m.form.query(searchNow); err == nil || !strings.Contains(err.Error(), "Updated to") {
		t.Errorf("err = %v, want an updated to error", err)
	}
}

func TestSearchPicksProjectInForm(t *testing.T) {
	t.Parallel()
	fb := &fakeBackend{defaultProject: "DEMO", projects: []jira.Project{{Key: "DEMO"}, {Key: "OPS"}}}
	m := newTestSearch(t, fb)

	m = searchPressDrain(t, m, "right")
	if q, _ := m.form.query(searchNow); q.Project != "OPS" {
		t.Errorf("Project = %q, want OPS", q.Project)
	}

	m = searchPressDrain(t, m, "right")
	if q, _ := m.form.query(searchNow); q.Project != "" {
		t.Errorf("Project = %q after clearing", q.Project)
	}
}

func TestSearchQuitKeys(t *testing.T) {
	t.Parallel()
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
	m = typeInto(t, m, fieldKey, "q")
	if m.quit.confirming || m.form.values.keys != "q" {
		t.Error("q should be typed into the field")
	}
}

func TestSearchAdvancedMode(t *testing.T) {
	t.Parallel()
	m := newTestSearch(t, &fakeBackend{defaultProject: "DEMO"})
	m = searchPressDrain(t, m, "ctrl+t")
	if !m.form.advanced || m.form.jql != `project = "DEMO" ORDER BY updated DESC` {
		t.Fatalf("advanced = %v, jql = %q", m.form.advanced, m.form.jql)
	}
	if !strings.Contains(m.render(), "advanced") {
		t.Error("header doesn't show the mode")
	}

	// Keys go to the editor, so q is typed rather than quitting.
	m = searchType(t, m, " q")
	if !strings.HasSuffix(m.form.jql, " q") || m.quit.confirming {
		t.Errorf("jql = %q", m.form.jql)
	}

	// What was written survives a trip through the form.
	m = searchPressDrain(t, m, "ctrl+t", "ctrl+t")
	if m.form.jql != `project = "DEMO" ORDER BY updated DESC q` {
		t.Errorf("jql = %q after switching back", m.form.jql)
	}
	m = searchPressDrain(t, m, "ctrl+t")
	if m.form.advanced {
		t.Error("should be back in basic mode")
	}
}

func TestSearchAdvancedStartsEmptyWhenFormInvalid(t *testing.T) {
	t.Parallel()
	m := newTestSearch(t, &fakeBackend{})
	m = typeInto(t, m, fieldKey, "nope")
	m = searchPressDrain(t, m, "ctrl+t")
	if m.form.jql != "" {
		t.Errorf("jql = %q, want empty", m.form.jql)
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
	t.Parallel()
	m, fb := searched(t)
	if len(fb.findCalls) != 1 || fb.findCalls[0] != `project = "DEMO" ORDER BY updated DESC` {
		t.Fatalf("searches = %q", fb.findCalls)
	}
	if !m.resultsFocused() || len(m.results.issues) != 3 {
		t.Fatalf("resultsFocused = %v, issues = %d", m.resultsFocused(), len(m.results.issues))
	}
	view := m.render()
	for _, want := range []string{"Results (3)", "DEMO-1", "Second"} {
		if !strings.Contains(ansi.Strip(view), want) {
			t.Errorf("view is missing %q:\n%s", want, ansi.Strip(view))
		}
	}

	m = searchPressDrain(t, m, "j", "j")
	if got := m.results.selected().Key; got != "DEMO-3" {
		t.Errorf("selected = %s, want DEMO-3", got)
	}
	m = searchPressDrain(t, m, "g")
	if got := m.results.selected().Key; got != "DEMO-1" {
		t.Errorf("selected = %s after g, want DEMO-1", got)
	}
}

func TestSearchResultsBackToForm(t *testing.T) {
	t.Parallel()
	m, _ := searched(t)
	m = searchPressDrain(t, m, "q")
	if m.quit.confirming {
		t.Fatal("q in results should quit, not confirm")
	}
	m = searchPressDrain(t, m, "esc")
	if m.resultsFocused() {
		t.Fatal("esc should return to the form")
	}
	// Tab from the last field goes back down to the results.
	m = focusField(t, m, fieldUpdatedTo)
	m = searchPressDrain(t, m, "tab")
	if !m.resultsFocused() {
		t.Error("tab from the last field should focus the results")
	}
}

func TestSearchResultsQuit(t *testing.T) {
	t.Parallel()
	m, _ := searched(t)
	if _, cmd := m.Update(keyMsg("q")); !isQuit(cmd) {
		t.Error("q should quit from the results")
	}
}

func TestSearchAdvancedSubmit(t *testing.T) {
	t.Parallel()
	fb := &fakeBackend{found: foundIssues()}
	m := newTestSearch(t, fb)
	m = searchPressDrain(t, m, "ctrl+t", "ctrl+a", "ctrl+k")
	m = searchPressDrain(t, m, "enter")
	if len(fb.findCalls) != 0 || !strings.Contains(m.status, "write some JQL") {
		t.Fatalf("searches = %q, status = %q", fb.findCalls, m.status)
	}

	m = searchType(t, m, "assignee = currentUser()")
	m = searchPressDrain(t, m, "enter")
	if len(fb.findCalls) != 1 || !strings.Contains(fb.findCalls[0], "currentUser()") {
		t.Errorf("searches = %q", fb.findCalls)
	}
	if !m.resultsFocused() {
		t.Error("results should have focus")
	}
}

func TestSearchShowsErrorsAndEmptyResults(t *testing.T) {
	t.Parallel()
	fb := &fakeBackend{findErr: errors.New("bad JQL")}
	m := searchPressDrain(t, newTestSearch(t, fb), "ctrl+s")
	if !strings.Contains(m.status, "bad JQL") || m.resultsFocused() {
		t.Errorf("view:\n%s", ansi.Strip(m.render()))
	}

	fb.findErr = nil
	m = searchPressDrain(t, m, "ctrl+s")
	if !strings.Contains(ansi.Strip(m.render()), "No issues found.") || m.resultsFocused() {
		t.Errorf("view:\n%s", ansi.Strip(m.render()))
	}
}

func TestSearchIgnoresStaleResults(t *testing.T) {
	t.Parallel()
	m, _ := searched(t)
	m.searchID = 5
	m, _ = searchSend(m, searchLoadedMsg{id: 4, issues: nil})
	if len(m.results.issues) != 3 {
		t.Error("an older search replaced the newer results")
	}
}

func TestSearchRefreshKeepsFocusedIssue(t *testing.T) {
	t.Parallel()
	m, fb := searched(t)
	m = searchPressDrain(t, m, "j")
	m = searchPressDrain(t, m, "r")
	if len(fb.findCalls) != 2 || fb.findCalls[0] != fb.findCalls[1] {
		t.Errorf("searches = %q", fb.findCalls)
	}
	if got := m.results.selected().Key; got != "DEMO-2" {
		t.Errorf("selected = %s, want DEMO-2", got)
	}
}

func TestSearchIssueActions(t *testing.T) {
	t.Parallel()
	m, fb := searched(t)
	fb.detail = &jira.IssueDetail{Issue: foundIssues()[0], Description: "Details here"}
	fb.transitions = []jira.Transition{{ID: "31", Name: "Done", To: "Done"}}

	m = searchPressDrain(t, m, "v")
	if m.popup == nil || !strings.Contains(ansi.Strip(m.render()), "Details here") {
		t.Fatalf("details popup didn't open:\n%s", ansi.Strip(m.render()))
	}
	m = searchPressDrain(t, m, "esc")
	if m.popup != nil {
		t.Fatal("esc should close the popup")
	}

	// Moving an issue searches again and keeps the cursor on it.
	m = searchPressDrain(t, m, "j")
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
	t.Parallel()
	m, _ := searched(t)
	var copied string
	m.actions.copyText = func(s string) error { copied = s; return nil }
	m = searchPressDrain(t, m, "j")
	_, cmd := searchSend(m, keyMsg("y"))
	cmd()
	if copied != "https://jira.example.com/browse/DEMO-2" {
		t.Errorf("copied = %q", copied)
	}
}

func TestSearchUsesOneHuhForm(t *testing.T) {
	t.Parallel()
	m, _ := searched(t)
	if m.form.form.GetFocusedField() != m.results.selector {
		t.Fatal("results should be a field of the search form")
	}
	m = searchPressDrain(t, m, "tab")
	if m.form.focused() != fieldProject || m.form.form.State != huh.StateNormal {
		t.Fatalf("tab should wrap without submitting: field = %s, state = %v", m.form.focused(), m.form.form.State)
	}
}

func TestSearchSelectPreviewsAndRestoresFocus(t *testing.T) {
	t.Parallel()
	m, fb := searched(t)
	fb.detail = &jira.IssueDetail{Issue: foundIssues()[1], Description: "Selected issue preview"}
	m = searchPressDrain(t, m, "j", "enter")
	if m.popup == nil || !strings.Contains(ansi.Strip(m.render()), "Selected issue preview") {
		t.Fatal("selecting a result should open its preview")
	}
	m = searchPressDrain(t, m, "esc")
	if !m.resultsFocused() || m.results.selected().Key != "DEMO-2" || m.form.form.State != huh.StateNormal {
		t.Fatal("closing the preview should restore the selected result")
	}
}

func TestSearchCommentAndEditShortcuts(t *testing.T) {
	t.Parallel()
	m, fb := searched(t)
	fb.detail = &jira.IssueDetail{Issue: foundIssues()[1]}
	m = searchPressDrain(t, m, "j", "c", "enter")
	m = searchType(t, m, "A comment")
	m = searchPressDrain(t, m, "ctrl+s")
	if len(fb.comments) != 1 || fb.comments[0] != "DEMO-2:A comment" {
		t.Fatalf("comments = %q", fb.comments)
	}
	m = searchPressDrain(t, m, "esc", "e")
	if p, ok := m.popup.(*panePopup); !ok || p.title != "Edit" || p.issueKey != "DEMO-2" {
		t.Fatalf("edit shortcut should use the hovered result: %T", m.popup)
	}
}

func TestSearchOpenUsesHoveredIssueAfterResize(t *testing.T) {
	t.Parallel()
	m, _ := searched(t)
	var opened string
	m.actions.openURL = func(s string) error { opened = s; return nil }
	m = searchPressDrain(t, m, "G")
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 50, Height: 20})
	m = searchDrain(t, next.(SearchModel), cmd)
	m = searchPressDrain(t, m, "o")
	if opened != "https://jira.example.com/browse/DEMO-3" {
		t.Fatalf("opened = %q", opened)
	}
}

func TestSearchReplacesResultsWithEmptyAndError(t *testing.T) {
	t.Parallel()
	m, fb := searched(t)
	fb.found = nil
	m = searchPressDrain(t, m, "r")
	if m.resultsFocused() || len(m.results.issues) != 0 || m.results.selected() != nil {
		t.Fatal("empty results should remove the old select and return to the input")
	}
	m = focusField(t, m, fieldUpdatedTo)
	m = searchPressDrain(t, m, "tab")
	if m.form.focused() != fieldProject || m.form.form.State != huh.StateNormal {
		t.Fatal("tab without results should wrap, not complete the form")
	}
	fb.findErr = errors.New("unavailable")
	m = searchPressDrain(t, m, "ctrl+s")
	if m.resultsFocused() || !strings.Contains(m.status, "unavailable") {
		t.Fatal("failed search should show its error and keep the input usable")
	}
}

func TestSearchCancelsReplacedRequests(t *testing.T) {
	t.Parallel()
	fb := &fakeBackend{found: foundIssues()}
	m := newTestSearch(t, fb)
	first := m.submit()().(tea.BatchMsg)[0]()
	original := fb.findContexts[0]
	if original.Err() != nil {
		t.Fatal("search should start with a live context")
	}
	second := m.submit()().(tea.BatchMsg)[0]()
	if !errors.Is(original.Err(), context.Canceled) {
		t.Fatal("replacing a search should cancel its context")
	}
	m, _ = searchSend(m, first)
	if m.searched || !m.searching {
		t.Fatal("a reply queued before cancellation should be ignored")
	}
	m, cmd := searchSend(m, second)
	m = searchDrain(t, m, cmd)
	if !m.resultsFocused() {
		t.Fatal("the latest reply should populate the result select")
	}
	_ = m.submit()().(tea.BatchMsg)[0]()
	active := fb.findContexts[len(fb.findContexts)-1]
	_, cmd = m.Update(keyMsg("ctrl+d"))
	if !isQuit(cmd) || !errors.Is(active.Err(), context.Canceled) {
		t.Fatal("quitting should cancel the active search")
	}
}

func TestSearchResizesWithinWindow(t *testing.T) {
	t.Parallel()
	m, _ := searched(t)
	for _, size := range []tea.WindowSizeMsg{
		{Width: 50, Height: 20}, {Width: 24, Height: 10}, {Width: 8, Height: 4}, {Width: 120, Height: 40},
	} {
		next, cmd := m.Update(size)
		m = searchDrain(t, next.(SearchModel), cmd)
		content := m.View().Content
		if lipgloss.Width(content) > size.Width || lipgloss.Height(content) > size.Height {
			t.Errorf("view %dx%d exceeds window %dx%d", lipgloss.Width(content), lipgloss.Height(content), size.Width, size.Height)
		}
		if !m.resultsFocused() || m.results.selected().Key != "DEMO-1" {
			t.Fatal("resizing should preserve the selected result")
		}
	}
}
