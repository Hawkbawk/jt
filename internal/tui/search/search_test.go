package search

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/hawkbawk/jt/internal/components/commandpicker"
	"github.com/hawkbawk/jt/internal/config"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
	"github.com/hawkbawk/jt/internal/tui/tuitest"
)

var searchNow = time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)

func newTestSearch(t *testing.T, fb *tuitest.Backend) Model {
	t.Helper()
	m := New(fb, &tuitest.Settings{})
	m.form.now = func() time.Time { return searchNow }
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = tuitest.Drain(t, next.(Model), cmd)
	return tuitest.Drain(t, m, m.Init())
}

func withFilters(t *testing.T, m Model) Model {
	t.Helper()
	return tuitest.PressDrain(t, m, "ctrl+f")
}

// searchSend feeds msgs to the model one at a time, then returns it and the
// last command.
func searchSend(m Model, msgs ...tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	for _, msg := range msgs {
		var next tea.Model
		next, cmd = m.Update(msg)
		m = next.(Model)
	}
	return m, cmd
}

// focusField presses tab until the field has focus.
func focusField(t *testing.T, m Model, field string) Model {
	t.Helper()
	for i := 0; m.form.focused() != field; i++ {
		if i >= len(m.form.fields) {
			t.Fatalf("could not focus %s; focused %s", field, m.form.focused())
		}
		m = tuitest.PressDrain(t, m, "tab")
	}
	return m
}

func typeInto(t *testing.T, m Model, field, text string) Model {
	t.Helper()
	return searchType(t, focusField(t, m, field), text)
}

func searchType(t *testing.T, m Model, text string) Model {
	t.Helper()
	if m.Modal.IsOpen() {
		for _, r := range text {
			next, cmd := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
			m = tuitest.Drain(t, next.(Model), cmd)
		}
		return m
	}
	next, cmd := m.Update(tea.PasteMsg{Content: text})
	return tuitest.Drain(t, next.(Model), cmd)
}

func TestSearchBox(t *testing.T) {
	t.Parallel()
	tests := []struct{ text, want string }{
		{"12345", "key = DEMO-12345"},
		{"ops-7", "key = OPS-7"},
		{"login page", `textfields ~ "login page" ORDER BY created DESC`},
	}
	for _, tt := range tests {
		fb := &tuitest.Backend{Project: "DEMO"}
		m := searchType(t, newTestSearch(t, fb), tt.text)
		m = tuitest.PressDrain(t, m, "ctrl+s")
		if len(fb.FindCalls) != 1 || fb.FindCalls[0] != tt.want {
			t.Errorf("%q: searches = %q, want %q", tt.text, fb.FindCalls, tt.want)
		}
		if strings.Contains(m.render(), "ORDER BY") {
			t.Error("the generated JQL shouldn't be shown")
		}
	}
}

func TestSearchBoxErrors(t *testing.T) {
	t.Parallel()
	fb := &tuitest.Backend{}
	m := tuitest.PressDrain(t, newTestSearch(t, fb), "ctrl+s")
	if len(fb.FindCalls) != 0 || !strings.Contains(ansi.Strip(m.render()), "type some text") {
		t.Errorf("an empty box shouldn't search:\n%s", ansi.Strip(m.render()))
	}
	m = tuitest.PressDrain(t, searchType(t, m, "42"), "ctrl+s")
	if len(fb.FindCalls) != 0 || !strings.Contains(ansi.Strip(m.render()), "set a default project") {
		t.Errorf("a number needs a default project:\n%s", ansi.Strip(m.render()))
	}
}

func TestSearchFiltersToggle(t *testing.T) {
	t.Parallel()
	fb := &tuitest.Backend{Project: "DEMO"}
	m := searchType(t, newTestSearch(t, fb), "login")
	if strings.Contains(ansi.Strip(m.render()), "Project") {
		t.Fatal("filters should start hidden")
	}
	m = withFilters(t, m)
	if !strings.Contains(ansi.Strip(m.render()), "Project") || !strings.Contains(m.render(), "filters") {
		t.Fatalf("ctrl+f should show the filters:\n%s", ansi.Strip(m.render()))
	}
	m = tuitest.PressDrain(t, m, "ctrl+s")
	tuitest.PressDrain(t, m, "esc", "ctrl+f", "ctrl+s")
	want := []string{`textfields ~ "login" AND project = "DEMO" ORDER BY created DESC`, `textfields ~ "login" ORDER BY created DESC`}
	if len(fb.FindCalls) != 2 || fb.FindCalls[0] != want[0] || fb.FindCalls[1] != want[1] {
		t.Errorf("searches = %q, want %q", fb.FindCalls, want)
	}
}

func TestSearchFormHasTwoColumns(t *testing.T) {
	t.Parallel()
	m := withFilters(t, newTestSearch(t, &tuitest.Backend{}))
	for line := range strings.SplitSeq(ansi.Strip(m.render()), "\n") {
		if strings.Contains(line, "Search") && strings.Contains(line, "Order") {
			return
		}
	}
	t.Errorf("fields aren't side by side:\n%s", ansi.Strip(m.render()))
}

func TestSearchLoadsIssueTypes(t *testing.T) {
	t.Parallel()
	fb := &tuitest.Backend{IssueTypeList: []string{"Bug", "Task"}}
	m := withFilters(t, newTestSearch(t, fb))
	msg := loadIssueTypes(fb)()
	m, _ = searchSend(m, msg)

	m = typeInto(t, m, fieldType, "task")
	q, err := m.form.query(searchNow)
	if err != nil || q.IssueType != "Task" {
		t.Errorf("IssueType = %q, err %v; want Task", q.IssueType, err)
	}
	m = tuitest.PressDrain(t, m, "tab")
	if m.form.focused() == fieldType {
		t.Error("a known issue type should let focus move on")
	}
	m = tuitest.PressDrain(t, m, "shift+tab")
	m = tuitest.PressDrain(t, searchType(t, m, "x"), "tab")
	if m.form.focused() != fieldType || !strings.Contains(ansi.Strip(m.render()), "not an issue type") {
		t.Errorf("an unknown type should show its error:\n%s", ansi.Strip(m.render()))
	}
}

func TestSearchFormFields(t *testing.T) {
	t.Parallel()
	m := withFilters(t, newTestSearch(t, &tuitest.Backend{}))
	m = typeInto(t, m, fieldKey, "demo-1, demo-2")
	m = focusField(t, m, fieldSprint)
	m = tuitest.PressDrain(t, m, "right")
	m = focusField(t, m, fieldOrder)
	m = tuitest.PressDrain(t, m, "left")
	m = typeInto(t, m, fieldCreated, "2026-03-01..2026-03-02")
	m = typeInto(t, m, fieldUpdated, "-1d")

	q, err := m.form.query(searchNow)
	if err != nil {
		t.Fatal(err)
	}
	want := `sprint in openSprints() AND key in (DEMO-1, DEMO-2) AND created >= "2026-03-01 00:00" AND created <= "2026-03-02 23:59" AND updated >= "2026-03-09 12:00" ORDER BY created ASC`
	if q.String() != want {
		t.Errorf("JQL = %q, want %q", q.String(), want)
	}
}

func TestSearchFormErrors(t *testing.T) {
	t.Parallel()
	m := withFilters(t, newTestSearch(t, &tuitest.Backend{}))
	m = typeInto(t, m, fieldKey, "nope")
	m = tuitest.PressDrain(t, m, "tab")
	if m.form.focused() != fieldKey || !strings.Contains(ansi.Strip(m.render()), "isn't an issue key") {
		t.Errorf("leaving the field should show its error and stay:\n%s", ansi.Strip(m.render()))
	}

	fb := &tuitest.Backend{}
	m = withFilters(t, newTestSearch(t, fb))
	m = typeInto(t, m, fieldUpdated, "soon")
	m = tuitest.PressDrain(t, m, "ctrl+s")
	if len(fb.FindCalls) != 0 || !strings.Contains(ansi.Strip(m.render()), "must be a date") {
		t.Errorf("searches = %q, view:\n%s", fb.FindCalls, ansi.Strip(m.render()))
	}
}

func TestSearchCtrlSValidatesFocusedField(t *testing.T) {
	t.Parallel()
	fb := &tuitest.Backend{}
	m := typeInto(t, withFilters(t, newTestSearch(t, fb)), fieldCreated, "soon")
	m = tuitest.PressDrain(t, m, "ctrl+s")
	if len(fb.FindCalls) != 0 || m.form.focused() != fieldCreated || !strings.Contains(ansi.Strip(m.render()), "must be a date") {
		t.Errorf("searches = %q, view:\n%s", fb.FindCalls, ansi.Strip(m.render()))
	}
}

func TestSearchPicksProjectInForm(t *testing.T) {
	t.Parallel()
	fb := &tuitest.Backend{Project: "DEMO", ProjectList: []jira.Project{{Key: "DEMO"}, {Key: "OPS", Name: "Operations"}}}
	m := withFilters(t, newTestSearch(t, fb))

	m = typeInto(t, m, fieldProject, "")
	m = tuitest.PressDrain(t, m, "ctrl+a", "ctrl+k")
	m = searchType(t, m, "operations")
	if q, _ := m.form.query(searchNow); q.Project != "OPS" {
		t.Errorf("Project = %q, want OPS", q.Project)
	}

	m = tuitest.PressDrain(t, m, "ctrl+a", "ctrl+k")
	m = searchType(t, m, "nope")
	m = tuitest.PressDrain(t, m, "tab")
	if m.form.focused() != fieldProject || !strings.Contains(ansi.Strip(m.render()), "not a project") {
		t.Errorf("an unknown project should show its error:\n%s", ansi.Strip(m.render()))
	}

	m = tuitest.PressDrain(t, m, "ctrl+a", "ctrl+k")
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
	m := newTestSearch(t, &tuitest.Backend{})
	if _, cmd := searchSend(m, tuitest.Key("ctrl+d")); cmd == nil {
		t.Error("ctrl+d should quit")
	}
	m, _ = searchSend(m, tuitest.Key("ctrl+c"))
	if !m.Quit.Confirming {
		t.Error("ctrl+c should ask to confirm")
	}
	// Typing q in a text field must not quit.
	m = newTestSearch(t, &tuitest.Backend{})
	m = typeInto(t, m, fieldText, "q")
	if m.Quit.Confirming || m.form.values.text != "q" {
		t.Error("q should be typed into the field")
	}
}

func TestSearchAdvancedMode(t *testing.T) {
	t.Parallel()
	m := searchType(t, newTestSearch(t, &tuitest.Backend{Project: "DEMO"}), "login")
	m = tuitest.PressDrain(t, m, "ctrl+t")
	if !m.form.advanced || m.form.jql != `textfields ~ "login" ORDER BY created DESC` {
		t.Fatalf("advanced = %v, jql = %q", m.form.advanced, m.form.jql)
	}
	if !strings.Contains(m.render(), "advanced") {
		t.Error("header doesn't show the mode")
	}

	// Keys go to the editor, so q is typed rather than quitting.
	m = searchType(t, m, " q")
	if !strings.HasSuffix(m.form.jql, " q") || m.Quit.Confirming {
		t.Errorf("jql = %q", m.form.jql)
	}

	// What was written survives a trip through the form.
	m = tuitest.PressDrain(t, m, "ctrl+t", "ctrl+t")
	if m.form.jql != `textfields ~ "login" ORDER BY created DESC q` {
		t.Errorf("jql = %q after switching back", m.form.jql)
	}
	m = tuitest.PressDrain(t, m, "ctrl+t")
	if m.form.advanced {
		t.Error("should be back in basic mode")
	}
}

func TestSearchAdvancedStartsEmptyWhenFormInvalid(t *testing.T) {
	t.Parallel()
	m := newTestSearch(t, &tuitest.Backend{})
	m = tuitest.PressDrain(t, m, "ctrl+t")
	if m.form.jql != "" {
		t.Errorf("jql = %q, want empty", m.form.jql)
	}
}

func foundIssues() []jira.Issue {
	return []jira.Issue{
		{Key: "DEMO-1", Summary: "First", Status: "To Do", Assignee: jira.Option{ID: "u1", Name: "Ada"}},
		{Key: "DEMO-2", Summary: "Second", Status: "Done"},
		{Key: "DEMO-3", Summary: "Third", Status: "In Progress"},
	}
}

func searched(t *testing.T) (Model, *tuitest.Backend) {
	t.Helper()
	fb := &tuitest.Backend{Project: "DEMO", Found: foundIssues()}
	m := searchType(t, newTestSearch(t, fb), "demo")
	return tuitest.PressDrain(t, m, "ctrl+s"), fb
}

func TestSearchRunsFormAndFocusesResults(t *testing.T) {
	t.Parallel()
	m, fb := searched(t)
	if len(fb.FindCalls) != 1 || fb.FindCalls[0] != `textfields ~ "demo" ORDER BY created DESC` {
		t.Fatalf("searches = %q", fb.FindCalls)
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

	m = tuitest.PressDrain(t, m, "j", "j")
	if got := m.results.selected().Key; got != "DEMO-3" {
		t.Errorf("selected = %s, want DEMO-3", got)
	}
	m = tuitest.PressDrain(t, m, "g")
	if got := m.results.selected().Key; got != "DEMO-1" {
		t.Errorf("selected = %s after g, want DEMO-1", got)
	}
}

func TestSearchResultsBackToForm(t *testing.T) {
	t.Parallel()
	m, _ := searched(t)
	m = tuitest.PressDrain(t, m, "q")
	if m.Quit.Confirming {
		t.Fatal("q in results should quit, not confirm")
	}
	m = tuitest.PressDrain(t, m, "esc")
	if m.resultsFocused() {
		t.Fatal("esc should return to the form")
	}
}

func TestSearchShowsResultsBelowForm(t *testing.T) {
	t.Parallel()
	m, _ := searched(t)
	view := ansi.Strip(m.render())
	if form, results := strings.Index(view, "Search"), strings.Index(view, "Results (3)"); form < 0 || results < form {
		t.Fatalf("form should sit above the results:\n%s", view)
	}
	m = tuitest.PressDrain(t, m, "ctrl+o")
	if m.resultsFocused() {
		t.Fatal("ctrl+o should move to the form")
	}
	m = tuitest.PressDrain(t, m, "ctrl+o")
	if !m.resultsFocused() {
		t.Fatal("ctrl+o should move back to the results")
	}
}

func TestSearchTabOnLastFieldMovesToResults(t *testing.T) {
	t.Parallel()
	m, _ := searched(t)
	m = tuitest.PressDrain(t, m, "esc")
	for i := 0; i < 20 && !m.form.onLastField(); i++ {
		if m.resultsFocused() {
			t.Fatal("tab left the form before its last field")
		}
		m = tuitest.PressDrain(t, m, "tab")
	}
	m = tuitest.PressDrain(t, m, "tab")
	if !m.resultsFocused() {
		t.Fatal("tab on the last field should move to the results")
	}
}

func TestSearchResultsQuit(t *testing.T) {
	t.Parallel()
	m, _ := searched(t)
	if _, cmd := m.Update(tuitest.Key("q")); !tuitest.IsQuit(cmd) {
		t.Error("q should quit from the results")
	}
}

func TestSearchAdvancedSubmit(t *testing.T) {
	t.Parallel()
	fb := &tuitest.Backend{Found: foundIssues()}
	m := newTestSearch(t, fb)
	m = tuitest.PressDrain(t, m, "ctrl+t", "ctrl+a", "ctrl+k")
	m = tuitest.PressDrain(t, m, "enter")
	if len(fb.FindCalls) != 0 || !strings.Contains(ansi.Strip(m.render()), "write some JQL") {
		t.Fatalf("searches = %q, view:\n%s", fb.FindCalls, ansi.Strip(m.render()))
	}

	m = searchType(t, m, "assignee = currentUser()")
	m = tuitest.PressDrain(t, m, "enter")
	if len(fb.FindCalls) != 1 || !strings.Contains(fb.FindCalls[0], "currentUser()") {
		t.Errorf("searches = %q", fb.FindCalls)
	}
	if !m.resultsFocused() {
		t.Error("results should have focus")
	}
}

func TestSearchShowsErrorsAndEmptyResults(t *testing.T) {
	t.Parallel()
	fb := &tuitest.Backend{FindErr: errors.New("bad JQL")}
	m := tuitest.PressDrain(t, searchType(t, newTestSearch(t, fb), "demo"), "ctrl+s")
	if !strings.Contains(ansi.Strip(m.render()), "Error: bad JQL") || !m.resultsFocused() {
		t.Errorf("view:\n%s", ansi.Strip(m.render()))
	}

	fb.FindErr = nil
	m = tuitest.PressDrain(t, m, "ctrl+s")
	if !strings.Contains(ansi.Strip(m.render()), "No issues found.") || !strings.Contains(ansi.Strip(m.render()), "Results (0)") {
		t.Errorf("view:\n%s", ansi.Strip(m.render()))
	}
}

func TestSearchRefresh(t *testing.T) {
	t.Parallel()
	m, fb := searched(t)
	tuitest.PressDrain(t, m, "r")
	if len(fb.FindCalls) != 2 || fb.FindCalls[0] != fb.FindCalls[1] {
		t.Errorf("searches = %q", fb.FindCalls)
	}
}

func TestSearchIssueActions(t *testing.T) {
	t.Parallel()
	m, fb := searched(t)
	fb.Detail = &jira.IssueDetail{Issue: foundIssues()[0], Description: "Details here"}
	fb.TransitionList = []jira.Transition{{ID: "31", Name: "Done", To: "Done"}}

	m = tuitest.PressDrain(t, m, "v")
	if !m.Modal.IsOpen() || !strings.Contains(ansi.Strip(m.render()), "Details here") {
		t.Fatalf("details modal didn't open:\n%s", ansi.Strip(m.render()))
	}
	m = tuitest.PressDrain(t, m, "esc")
	if m.Modal.IsOpen() {
		t.Fatal("esc should close the modal")
	}

	// Moving an issue searches again.
	m = tuitest.PressDrain(t, m, "j")
	m = tuitest.PressDrain(t, m, "m", "enter")
	if len(fb.Moved) != 1 || fb.Moved[0] != "DEMO-2:31" {
		t.Fatalf("moved = %q", fb.Moved)
	}
	if m.Modal.IsOpen() || len(fb.FindCalls) != 2 {
		t.Errorf("modal = %v, searches = %q, selected = %v", m.Modal.Content(), fb.FindCalls, m.results.selected())
	}
	if !strings.Contains(m.Status, "Moved DEMO-2 to Done") {
		t.Errorf("status = %q", m.Status)
	}
}

func TestSearchYankUsesSelectedIssue(t *testing.T) {
	t.Parallel()
	m, _ := searched(t)
	var copied string
	m.actions.CopyText = func(s string) error { copied = s; return nil }
	m = tuitest.PressDrain(t, m, "j")
	_, cmd := searchSend(m, tuitest.Key("y"))
	cmd()
	if copied != "https://jira.example.com/browse/DEMO-2" {
		t.Errorf("copied = %q", copied)
	}
}

func TestSearchSelectPreviewsAndRestoresFocus(t *testing.T) {
	t.Parallel()
	m, fb := searched(t)
	fb.Detail = &jira.IssueDetail{Issue: foundIssues()[1], Description: "Selected issue preview"}
	m = tuitest.PressDrain(t, m, "j", "enter")
	if !m.Modal.IsOpen() || !strings.Contains(ansi.Strip(m.render()), "Selected issue preview") {
		t.Fatal("selecting a result should open its preview")
	}
	m = tuitest.PressDrain(t, m, "esc")
	if !m.resultsFocused() || m.results.selected().Key != "DEMO-2" || m.form.form.State != huh.StateNormal {
		t.Fatal("closing the preview should restore the selected result")
	}
}

func TestSearchCommentAndEditShortcuts(t *testing.T) {
	t.Parallel()
	m, fb := searched(t)
	fb.CommentList = &jira.CommentList{}
	m = tuitest.PressDrain(t, m, "j", "c", "enter")
	m = searchType(t, m, "A comment")
	m = tuitest.PressDrain(t, m, "ctrl+s")
	if len(fb.Commented) != 1 || fb.Commented[0] != "DEMO-2:A comment" {
		t.Fatalf("comments = %q", fb.Commented)
	}
	m = tuitest.PressDrain(t, m, "esc", "e")
	if !m.Modal.IsOpen() || !strings.Contains(m.View().Content, "Edit · DEMO-2") {
		t.Fatalf("edit shortcut should use the hovered result: %T", m.Modal.Content())
	}
}

func TestSearchCreateOpensOnTheDefaultProject(t *testing.T) {
	t.Parallel()
	m, fb := searched(t)
	fb.ProjectList = []jira.Project{{Key: "DEMO", Name: "Demo"}}
	fb.CreateTypes = map[string][]jira.IssueType{"DEMO": {{ID: "1", Name: "Task"}}}
	m = tuitest.PressDrain(t, m, "n")
	if !m.Modal.IsOpen() || !strings.Contains(m.View().Content, "Issue type") || !slices.Equal(fb.CreateTypeCalls, []string{"DEMO"}) {
		t.Fatalf("n should open the create modal for DEMO, calls = %v", fb.CreateTypeCalls)
	}
}

func TestSearchRunsConfiguredCommandForHoveredIssue(t *testing.T) {
	t.Parallel()
	m, _ := searched(t)
	m.commands = []config.Command{{Name: "Open ticket", Command: "code"}}
	m = tuitest.PressDrain(t, m, "j", "C")
	p, ok := m.Modal.Content().(*commandpicker.Picker)
	if !ok || !strings.Contains(p.View().Content, "Open ticket") {
		t.Fatalf("C should open the command picker for the hovered issue: %T", m.Modal.Content())
	}
	if m.results.selected().Key != "DEMO-2" {
		t.Fatalf("selected issue = %q, want DEMO-2", m.results.selected().Key)
	}
	selected := tuimsg.CommandSelected{
		Command: config.Command{Command: "true"}, Issue: *m.results.selected(), Project: "DEMO",
	}
	next, cmd := m.Update(selected)
	if cmd == nil {
		t.Fatal("selecting a command did not load the hovered issue")
	}
	loaded := cmd().(tuimsg.CommandLoaded)
	if loaded.Err != nil || loaded.Data.Key != "DEMO-2" || loaded.Data.Project != "DEMO" || loaded.Data.Board != "" {
		t.Fatalf("loaded search command context = %#v", loaded)
	}
	if _, cmd = next.(Model).Update(loaded); cmd == nil {
		t.Fatal("loaded command did not start")
	}
}

func TestSearchOpenUsesHoveredIssueAfterResize(t *testing.T) {
	t.Parallel()
	m, _ := searched(t)
	var opened string
	m.actions.OpenURL = func(s string) error { opened = s; return nil }
	m = tuitest.PressDrain(t, m, "G")
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 50, Height: 20})
	m = tuitest.Drain(t, next.(Model), cmd)
	m = tuitest.PressDrain(t, m, "o")
	if opened != "https://jira.example.com/browse/DEMO-3" {
		t.Fatalf("opened = %q", opened)
	}
}

func TestSearchReplacesResultsWithEmptyAndError(t *testing.T) {
	t.Parallel()
	m, fb := searched(t)
	fb.Found = nil
	m = tuitest.PressDrain(t, m, "r")
	if !m.resultsFocused() || len(m.results.issues) != 0 || m.results.selected() != nil {
		t.Fatal("empty results should replace the old rows and stay in the table")
	}
	fb.FindErr = errors.New("unavailable")
	m = tuitest.PressDrain(t, m, "ctrl+s")
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
		m = tuitest.Drain(t, next.(Model), cmd)
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
	fb := &tuitest.Backend{Found: foundIssues()}
	m := searchType(t, newTestSearch(t, fb), "demo")
	m, cmd := searchSend(m, tuitest.Key("ctrl+s"))
	if !m.searching || cmd == nil {
		t.Fatal("ctrl+s should start a search")
	}
	if _, again := searchSend(m, tuitest.Key("ctrl+s")); again != nil {
		t.Error("a second search should wait for the first")
	}
	m = tuitest.Drain(t, m, cmd)
	if len(fb.FindCalls) != 1 || m.searching {
		t.Errorf("searches = %q, searching = %v", fb.FindCalls, m.searching)
	}
}

func TestSearchResultsHelpKeyTogglesFullHelp(t *testing.T) {
	t.Parallel()
	m, _ := searched(t)
	if strings.Contains(ansi.Strip(m.render()), "yank URL") {
		t.Fatal("short help should leave out yank")
	}
	m = tuitest.PressDrain(t, m, "?")
	if !m.Help.ShowAll || !strings.Contains(ansi.Strip(m.render()), "yank URL") {
		t.Fatal("? should show every binding")
	}
}

func TestSearchFormTypesQuestionMark(t *testing.T) {
	t.Parallel()
	m := newTestSearch(t, &tuitest.Backend{Project: "DEMO"})
	m = tuitest.PressDrain(t, m, "?")
	if m.Help.ShowAll {
		t.Fatal("? in the form must not open help")
	}
}
