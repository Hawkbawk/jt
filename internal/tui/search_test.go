package tui

import (
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
	if strings.Contains(m.render(), "ORDER BY") {
		t.Error("the generated JQL shouldn't be shown")
	}
}

func TestSearchFormHasTwoColumns(t *testing.T) {
	t.Parallel()
	m := newTestSearch(t, &fakeBackend{})
	for _, line := range strings.Split(ansi.Strip(m.render()), "\n") {
		if strings.Contains(line, "Issue key") && strings.Contains(line, "Order") {
			return
		}
	}
	t.Errorf("fields aren't side by side:\n%s", ansi.Strip(m.render()))
}

func TestSearchLoadsIssueTypes(t *testing.T) {
	t.Parallel()
	fb := &fakeBackend{issueTypes: []string{"Bug", "Task"}}
	m := newTestSearch(t, fb)
	msg := loadIssueTypes(fb)()
	m, _ = searchSend(m, msg)

	m = typeInto(t, m, fieldType, "task")
	q, err := m.form.query(searchNow)
	if err != nil || q.IssueType != "Task" {
		t.Errorf("IssueType = %q, err %v; want Task", q.IssueType, err)
	}
	m = searchPressDrain(t, m, "tab")
	if m.form.focused() == fieldType {
		t.Error("a known issue type should let focus move on")
	}
	m = searchPressDrain(t, m, "shift+tab")
	m = searchPressDrain(t, searchType(t, m, "x"), "tab")
	if m.form.focused() != fieldType || !strings.Contains(ansi.Strip(m.render()), "not an issue type") {
		t.Errorf("an unknown type should show its error:\n%s", ansi.Strip(m.render()))
	}
}

func TestSearchFormFields(t *testing.T) {
	t.Parallel()
	m := newTestSearch(t, &fakeBackend{})
	m = typeInto(t, m, fieldKey, "demo-1, demo-2")
	m = focusField(t, m, fieldSprint)
	m = searchPressDrain(t, m, "right")
	m = focusField(t, m, fieldOrder)
	m = searchPressDrain(t, m, "left")
	m = typeInto(t, m, fieldCreated, "2026-03-01..2026-03-02")
	m = typeInto(t, m, fieldUpdated, "-1d")

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
	m = searchPressDrain(t, m, "tab")
	if m.form.focused() != fieldKey || !strings.Contains(ansi.Strip(m.render()), "isn't an issue key") {
		t.Errorf("leaving the field should show its error and stay:\n%s", ansi.Strip(m.render()))
	}

	fb := &fakeBackend{}
	m = newTestSearch(t, fb)
	m = typeInto(t, m, fieldUpdated, "soon")
	m = searchPressDrain(t, m, "ctrl+s")
	if len(fb.findCalls) != 0 || !strings.Contains(ansi.Strip(m.render()), "must be a date") {
		t.Errorf("searches = %q, view:\n%s", fb.findCalls, ansi.Strip(m.render()))
	}
}

func TestSearchCtrlSValidatesFocusedField(t *testing.T) {
	t.Parallel()
	fb := &fakeBackend{}
	m := typeInto(t, newTestSearch(t, fb), fieldCreated, "soon")
	m = searchPressDrain(t, m, "ctrl+s")
	if len(fb.findCalls) != 0 || m.form.focused() != fieldCreated || !strings.Contains(ansi.Strip(m.render()), "must be a date") {
		t.Errorf("searches = %q, view:\n%s", fb.findCalls, ansi.Strip(m.render()))
	}
}

func TestSearchPicksProjectInForm(t *testing.T) {
	t.Parallel()
	fb := &fakeBackend{defaultProject: "DEMO", projects: []jira.Project{{Key: "DEMO"}, {Key: "OPS", Name: "Operations"}}}
	m := newTestSearch(t, fb)

	m = typeInto(t, m, fieldProject, "")
	m = searchPressDrain(t, m, "ctrl+a", "ctrl+k")
	m = searchType(t, m, "operations")
	if q, _ := m.form.query(searchNow); q.Project != "OPS" {
		t.Errorf("Project = %q, want OPS", q.Project)
	}

	m = searchPressDrain(t, m, "ctrl+a", "ctrl+k")
	m = searchType(t, m, "nope")
	m = searchPressDrain(t, m, "tab")
	if m.form.focused() != fieldProject || !strings.Contains(ansi.Strip(m.render()), "not a project") {
		t.Errorf("an unknown project should show its error:\n%s", ansi.Strip(m.render()))
	}

	m = searchPressDrain(t, m, "ctrl+a", "ctrl+k")
	if q, _ := m.form.query(searchNow); q.Project != "" {
		t.Errorf("Project = %q after clearing", q.Project)
	}
}

func TestParseTimeRange(t *testing.T) {
	t.Parallel()
	day := func(d int, h, m int) time.Time { return time.Date(2026, 3, d, h, m, 0, 0, time.UTC) }
	tests := []struct {
		text    string
		want    jira.TimeRange
		wantErr string
	}{
		{"", jira.TimeRange{}, ""},
		{"2026-03-01", jira.TimeRange{From: day(1, 0, 0)}, ""},
		{"-1d", jira.TimeRange{From: day(9, 12, 0)}, ""},
		{"2026-03-20", jira.TimeRange{From: searchNow, To: day(20, 23, 59)}, ""},
		{"2026-03-01..2026-03-02", jira.TimeRange{From: day(1, 0, 0), To: day(2, 23, 59)}, ""},
		{"2026-03-01..", jira.TimeRange{From: day(1, 0, 0)}, ""},
		{"..2026-03-01", jira.TimeRange{To: day(1, 23, 59)}, ""},
		{"-2d.. -1d", jira.TimeRange{From: day(8, 12, 0), To: day(9, 12, 0)}, ""},
		{"2026-03-05..2026-03-01", jira.TimeRange{}, "ends before"},
		{"soon", jira.TimeRange{}, "must be a date"},
		{"2026-03-01..soon", jira.TimeRange{}, "must be a date"},
	}
	for _, tt := range tests {
		got, err := parseTimeRange(tt.text, searchNow)
		if tt.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("%q: err = %v, want %q", tt.text, err, tt.wantErr)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Errorf("%q = %+v, %v; want %+v", tt.text, got, err, tt.want)
		}
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
}

func TestSearchShowsResultsBelowForm(t *testing.T) {
	t.Parallel()
	m, _ := searched(t)
	view := ansi.Strip(m.render())
	if form, results := strings.Index(view, "Project"), strings.Index(view, "Results (3)"); form < 0 || results < form {
		t.Fatalf("form should sit above the results:\n%s", view)
	}
	m = searchPressDrain(t, m, "ctrl+o")
	if m.resultsFocused() {
		t.Fatal("ctrl+o should move to the form")
	}
	m = searchPressDrain(t, m, "ctrl+o")
	if !m.resultsFocused() {
		t.Fatal("ctrl+o should move back to the results")
	}
}

func TestSearchTabOnLastFieldMovesToResults(t *testing.T) {
	t.Parallel()
	m, _ := searched(t)
	m = searchPressDrain(t, m, "esc")
	for i := 0; i < 20 && !m.form.onLastField(); i++ {
		if m.resultsFocused() {
			t.Fatal("tab left the form before its last field")
		}
		m = searchPressDrain(t, m, "tab")
	}
	m = searchPressDrain(t, m, "tab")
	if !m.resultsFocused() {
		t.Fatal("tab on the last field should move to the results")
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
	if len(fb.findCalls) != 0 || !strings.Contains(ansi.Strip(m.render()), "write some JQL") {
		t.Fatalf("searches = %q, view:\n%s", fb.findCalls, ansi.Strip(m.render()))
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
	if !strings.Contains(ansi.Strip(m.render()), "Error: bad JQL") || !m.resultsFocused() {
		t.Errorf("view:\n%s", ansi.Strip(m.render()))
	}

	fb.findErr = nil
	m = searchPressDrain(t, m, "ctrl+s")
	if !strings.Contains(ansi.Strip(m.render()), "No issues found.") || !strings.Contains(ansi.Strip(m.render()), "Results (0)") {
		t.Errorf("view:\n%s", ansi.Strip(m.render()))
	}
}

func TestSearchRefresh(t *testing.T) {
	t.Parallel()
	m, fb := searched(t)
	searchPressDrain(t, m, "r")
	if len(fb.findCalls) != 2 || fb.findCalls[0] != fb.findCalls[1] {
		t.Errorf("searches = %q", fb.findCalls)
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

	// Moving an issue searches again.
	m = searchPressDrain(t, m, "j")
	m = searchPressDrain(t, m, "m", "enter")
	if len(fb.moved) != 1 || fb.moved[0] != "DEMO-2:31" {
		t.Fatalf("moved = %q", fb.moved)
	}
	if m.popup != nil || len(fb.findCalls) != 2 {
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
	if !m.resultsFocused() || len(m.results.issues) != 0 || m.results.selected() != nil {
		t.Fatal("empty results should replace the old rows and stay in the table")
	}
	fb.findErr = errors.New("unavailable")
	m = searchPressDrain(t, m, "ctrl+s")
	if !strings.Contains(ansi.Strip(m.render()), "unavailable") || len(m.results.issues) != 0 {
		t.Fatal("failed search should show its error under an empty table")
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

func TestSearchIgnoresSubmitWhileSearching(t *testing.T) {
	t.Parallel()
	fb := &fakeBackend{found: foundIssues()}
	m := newTestSearch(t, fb)
	m, cmd := searchSend(m, keyMsg("ctrl+s"))
	if !m.searching || cmd == nil {
		t.Fatal("ctrl+s should start a search")
	}
	if _, again := searchSend(m, keyMsg("ctrl+s")); again != nil {
		t.Error("a second search should wait for the first")
	}
	m = searchDrain(t, m, cmd)
	if len(fb.findCalls) != 1 || m.searching {
		t.Errorf("searches = %q, searching = %v", fb.findCalls, m.searching)
	}
}
