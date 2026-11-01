package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

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
	m = focusField(m, field)
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
