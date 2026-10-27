package tui

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/hawkbawk/jt/internal/jira"
)

func TestMain(m *testing.M) {
	searchDelay = 0
	os.Exit(m.Run())
}

func testFields() []jira.EditField {
	return []jira.EditField{
		{ID: "summary", Name: "Summary", Kind: jira.FieldText, Required: true, Text: "Fix it"},
		{ID: "description", Name: "Description", Kind: jira.FieldTextArea, Text: "Steps"},
		{
			ID: "priority", Name: "Priority", Kind: jira.FieldSelect, Chosen: []string{"3"},
			Options: []jira.Option{{ID: "1", Name: "High"}, {ID: "3", Name: "Medium"}},
		},
		{ID: "labels", Name: "Labels", Kind: jira.FieldLabels, Chosen: []string{"api"}},
		{
			ID: "components", Name: "Components", Kind: jira.FieldMultiSelect,
			Options: []jira.Option{{ID: "c1", Name: "API"}, {ID: "c2", Name: "UI"}},
		},
		{
			ID: "owner", Name: "Owner", Kind: jira.FieldUser, Chosen: []string{"u1"},
			Options: []jira.Option{{ID: "u1", Name: "Ada"}},
		},
		{
			ID: "reviewers", Name: "Reviewers", Kind: jira.FieldMultiUser, Chosen: []string{"u1"},
			Options: []jira.Option{{ID: "u1", Name: "Ada"}},
		},
	}
}

// fieldsModel opens the edit popup.
func fieldsModel(t *testing.T) (Model, *fakeBackend) {
	t.Helper()
	m, fb := issueModel(t)
	return pressDrain(t, m, "e"), fb
}

func typeText(t *testing.T, m Model, s string) Model {
	t.Helper()
	for _, r := range s {
		m = pressDrain(t, m, string(r))
	}
	return m
}

// savedField returns the field with id from the only edit saved.
func savedField(t *testing.T, fb *fakeBackend, id string) jira.EditField {
	t.Helper()
	if len(fb.edited) != 1 {
		t.Fatalf("expected one save, got %d", len(fb.edited))
	}
	i := slices.IndexFunc(fb.edited[0], func(f jira.EditField) bool { return f.ID == id })
	if i < 0 {
		t.Fatalf("%s not saved", id)
	}
	return fb.edited[0][i]
}

func TestFieldsSaveChangesTogether(t *testing.T) {
	m, fb := fieldsModel(t)
	m = pressDrain(t, m, "enter")
	m = typeText(t, m, " now")
	m = pressDrain(t, m, "enter", "j", "j", "enter", "k", "enter", "j", "enter")
	m = typeText(t, m, " ui")
	m = pressDrain(t, m, "enter", "j", "enter", " ", "enter")
	if len(fb.edited) != 0 {
		t.Fatal("nothing should be saved before ctrl+s")
	}
	m = pressDrain(t, m, "ctrl+s")

	if len(fb.edited[0]) != 4 {
		t.Errorf("only changed fields should be sent, got %d", len(fb.edited[0]))
	}
	if got := savedField(t, fb, "summary").Text; got != "Fix it now" {
		t.Errorf("summary = %q", got)
	}
	if got := savedField(t, fb, "priority").Chosen; !slices.Equal(got, []string{"1"}) {
		t.Errorf("priority = %v", got)
	}
	if got := savedField(t, fb, "labels").Chosen; !slices.Equal(got, []string{"api", "ui"}) {
		t.Errorf("labels = %v", got)
	}
	if got := savedField(t, fb, "components").Chosen; !slices.Equal(got, []string{"c1"}) {
		t.Errorf("components = %v", got)
	}
	if m.popup != nil || len(fb.boardCalls) != 2 || !strings.Contains(m.status, "Updated 4 fields on T-2") {
		t.Fatalf("popup should close and the board reload, status = %q", m.status)
	}
}

func TestFieldsFormKeepsKeysThatWouldClose(t *testing.T) {
	m, fb := fieldsModel(t)
	m = pressDrain(t, m, "enter")
	m = typeText(t, m, "qhl/")
	if m.popup == nil {
		t.Fatal("typing should stay in the form")
	}
	pressDrain(t, m, "enter", "ctrl+s")
	if got := savedField(t, fb, "summary").Text; got != "Fix itqhl/" {
		t.Errorf("summary = %q", got)
	}
}

func TestFieldsEscRevertsEditEnterKeepsIt(t *testing.T) {
	m, fb := fieldsModel(t)
	m = pressDrain(t, m, "enter")
	m = typeText(t, m, " now")
	m = pressDrain(t, m, "esc", "ctrl+s")
	if len(fb.edited) != 0 {
		t.Fatalf("esc should drop the edit, got %v", fb.edited)
	}
	m = pressDrain(t, m, "enter")
	m = typeText(t, m, " now")
	m = pressDrain(t, m, "enter", "enter")
	m = typeText(t, m, " more")
	pressDrain(t, m, "esc", "ctrl+s")
	if got := savedField(t, fb, "summary").Text; got != "Fix it now" {
		t.Errorf("summary = %q", got)
	}
}

func TestFieldsCtrlSDoesNothingInsideForm(t *testing.T) {
	m, fb := fieldsModel(t)
	m = pressDrain(t, m, "enter")
	m = typeText(t, m, "!")
	m = pressDrain(t, m, "ctrl+s")
	if len(fb.edited) != 0 || m.popup == nil {
		t.Fatalf("ctrl+s should only save from the list, got %v", fb.edited)
	}
}

func TestFieldsEscGoesBackToListThenCloses(t *testing.T) {
	m, _ := fieldsModel(t)
	m = pressDrain(t, m, "enter")
	if v := m.View().Content; strings.Contains(v, "Description") {
		t.Fatal("only the selected field should show")
	}
	m = pressDrain(t, m, "esc")
	if v := m.View().Content; m.popup == nil || !strings.Contains(v, "Description") || !strings.Contains(v, "Components") {
		t.Fatal("esc should go back to the list of fields")
	}
	if m = pressDrain(t, m, "esc"); m.popup != nil {
		t.Fatal("esc on the list should close the popup")
	}
}

func TestFieldsListShowsEnteredValues(t *testing.T) {
	m, _ := fieldsModel(t)
	m = pressDrain(t, m, "enter")
	m = typeText(t, m, "!")
	m = pressDrain(t, m, "enter")
	if v := m.View().Content; !strings.Contains(v, "Fix it!") || !strings.Contains(v, "Summary * ●") {
		t.Fatal("list should show the new value and mark the field as changed")
	}
}

func TestFieldsUndoResetsSelectedField(t *testing.T) {
	m, fb := fieldsModel(t)
	m = pressDrain(t, m, "enter")
	m = typeText(t, m, "!")
	m = pressDrain(t, m, "esc", "j", "j", "enter", "k", "enter", "k", "k", "u")
	if v := m.View().Content; !strings.Contains(v, "1 unsaved change") || strings.Contains(v, "Summary * ●") {
		t.Fatal("u should reset the selected field and leave the other change")
	}
	pressDrain(t, m, "j", "j", "u", "ctrl+s")
	if len(fb.edited) != 0 {
		t.Fatalf("reset fields shouldn't be saved, got %v", fb.edited)
	}
}

func TestFieldsFilterNarrowsList(t *testing.T) {
	m, fb := fieldsModel(t)
	m = pressDrain(t, m, "/")
	m = typeText(t, m, "lab")
	m = pressDrain(t, m, "enter")
	if v := ansi.Strip(m.View().Content); strings.Contains(v, "Summary") || !strings.Contains(v, "Labels") {
		t.Fatal("filter should leave only matching fields")
	}
	m = pressDrain(t, m, "enter")
	m = typeText(t, m, " ui")
	pressDrain(t, m, "enter", "ctrl+s")
	if got := savedField(t, fb, "labels").Chosen; !slices.Equal(got, []string{"api", "ui"}) {
		t.Errorf("labels = %v", got)
	}
}

func TestFieldsEscClearsFilterBeforeClosing(t *testing.T) {
	m, _ := fieldsModel(t)
	m = pressDrain(t, m, "/")
	m = typeText(t, m, "q")
	m = pressDrain(t, m, "esc")
	if m.popup == nil || !strings.Contains(m.View().Content, "Summary") {
		t.Fatal("esc should clear the filter and keep the popup open")
	}
}

func TestFieldsTextAreaTakesNewlines(t *testing.T) {
	m, fb := fieldsModel(t)
	m = pressDrain(t, m, "j", "enter")
	m = typeText(t, m, " one")
	m = pressDrain(t, m, "ctrl+j")
	m = typeText(t, m, "two")
	pressDrain(t, m, "enter", "ctrl+s")
	if got := savedField(t, fb, "description").Text; got != "Steps one\ntwo" {
		t.Errorf("description = %q", got)
	}
}

func TestFieldsSelectCanClearOptionalField(t *testing.T) {
	m, fb := fieldsModel(t)
	pressDrain(t, m, "j", "j", "enter", "k", "k", "enter", "ctrl+s")
	if got := savedField(t, fb, "priority").Chosen; len(got) != 0 {
		t.Errorf("priority = %v", got)
	}
}

func TestFieldsSaveErrorKeepsChangesForRetry(t *testing.T) {
	m, fb := fieldsModel(t)
	fb.editErr = errors.New("summary: too long")
	m = pressDrain(t, m, "enter")
	m = typeText(t, m, "!")
	m = pressDrain(t, m, "enter", "ctrl+s")
	if m.popup == nil || !strings.Contains(m.View().Content, "summary: too long") || len(fb.boardCalls) != 1 {
		t.Fatal("failed save should leave the popup open with the error")
	}
	fb.editErr = nil
	m = pressDrain(t, m, "ctrl+s")
	if got := savedField(t, fb, "summary").Text; got != "Fix it!" || m.popup != nil {
		t.Errorf("retry should save the kept change, got %q", got)
	}
}

func TestFieldsListScrolls(t *testing.T) {
	m, fb := issueModel(t)
	fb.editFields = nil
	for i := range 40 {
		fb.editFields = append(fb.editFields, jira.EditField{ID: fmt.Sprint(i), Name: fmt.Sprintf("Field %02d", i), Kind: jira.FieldText})
	}
	m = pressDrain(t, m, "e")
	if v := m.View().Content; !strings.Contains(v, "Field 00") || strings.Contains(v, "Field 39") {
		t.Fatal("list should start at the top")
	}
	m = pressDrain(t, m, "G")
	if v := m.View().Content; !strings.Contains(v, "Field 39") || strings.Contains(v, "Field 00") {
		t.Fatal("list should scroll to the last field")
	}
}

func TestFieldsCloseWithoutChangesNeedsNoConfirm(t *testing.T) {
	m, _ := fieldsModel(t)
	if m, _ = press(t, m, "esc"); m.popup != nil {
		t.Fatal("esc should close a popup with nothing to save")
	}
}

func TestFieldsCloseWithChangesAsksFirst(t *testing.T) {
	m, fb := fieldsModel(t)
	m = pressDrain(t, m, "enter")
	m = typeText(t, m, "!")
	if v := m.View().Content; strings.Contains(v, "unsaved") || strings.Contains(v, "ctrl+s") {
		t.Fatal("what is still being entered shouldn't count or offer saving")
	}
	m = pressDrain(t, m, "enter")
	if !strings.Contains(m.View().Content, "1 unsaved change · ctrl+s to save") {
		t.Fatal("kept changes should be called out")
	}
	m, _ = press(t, m, "esc")
	if m.popup == nil || !strings.Contains(m.View().Content, "Discard 1 unsaved change?") {
		t.Fatal("esc should ask before dropping changes")
	}
	m, _ = press(t, m, "n")
	if m.popup == nil || m.popup.(*panePopup).unsaved() != 1 {
		t.Fatal("n should keep editing with the changes intact")
	}
	m, _ = press(t, m, "esc", "y")
	if m.popup != nil || len(fb.edited) != 0 {
		t.Fatal("y should close without saving")
	}
}

func TestFieldsCtrlDQuitsWithoutAsking(t *testing.T) {
	m, _ := fieldsModel(t)
	m = pressDrain(t, m, "enter")
	m = typeText(t, m, "!")
	if _, cmd := press(t, m, "ctrl+d"); !isQuit(cmd) {
		t.Fatal("ctrl+d must quit immediately, unsaved changes or not")
	}
}

// userModel opens the user field that is field rows down the list.
func userModel(t *testing.T, field int) (Model, *fakeBackend) {
	t.Helper()
	m, fb := fieldsModel(t)
	fb.users = []jira.Option{{ID: "u2", Name: "Bo"}, {ID: "u3", Name: "Cy"}}
	for range field {
		m = pressDrain(t, m, "down")
	}
	return pressDrain(t, m, "enter"), fb
}

func TestUserFieldSearchesAndPicksOne(t *testing.T) {
	m, fb := userModel(t, 5)
	m = typeText(t, m, "bo")
	if !slices.Equal(fb.searches, []string{"b", "bo"}) {
		t.Fatalf("searches = %v", fb.searches)
	}
	m = pressDrain(t, m, "enter", "enter")
	if !strings.Contains(m.View().Content, "Cy") {
		t.Fatalf("expected the picked person in the list:\n%s", m.View().Content)
	}
	pressDrain(t, m, "ctrl+s")
	if got := savedField(t, fb, "owner").Chosen; !slices.Equal(got, []string{"u3"}) {
		t.Errorf("owner = %v", got)
	}
}

func TestUserFieldShowsCurrentPeopleAndCanClear(t *testing.T) {
	m, fb := userModel(t, 5)
	if !strings.Contains(m.View().Content, "(none)") || !strings.Contains(m.View().Content, "Ada") {
		t.Fatalf("expected the current person and a way to clear:\n%s", m.View().Content)
	}
	pressDrain(t, m, "enter", "up", "enter", "ctrl+s")
	if got := savedField(t, fb, "owner").Chosen; len(got) != 0 {
		t.Errorf("owner = %v", got)
	}
}

func TestUserFieldEscKeepsCurrentPerson(t *testing.T) {
	m, fb := userModel(t, 5)
	m = typeText(t, m, "bo")
	m = pressDrain(t, m, "enter", "esc", "ctrl+s")
	if len(fb.edited) != 0 || m.popup == nil {
		t.Fatalf("esc should leave the field unchanged, got %v", fb.edited)
	}
}

func TestUserFieldMultiKeepsPicksAcrossSearches(t *testing.T) {
	m, fb := userModel(t, 6)
	m = typeText(t, m, "c")
	m = pressDrain(t, m, "enter", "down", "down", " ")
	m = pressDrain(t, m, "shift+tab", "backspace")
	m = typeText(t, m, "b")
	m = pressDrain(t, m, "enter", "enter")
	if !strings.Contains(m.View().Content, "Ada, Cy") {
		t.Fatalf("expected the picked person's name in the list:\n%s", m.View().Content)
	}
	pressDrain(t, m, "ctrl+s")
	if got := savedField(t, fb, "reviewers").Chosen; !slices.Equal(got, []string{"u1", "u3"}) {
		t.Errorf("reviewers = %v", got)
	}
}

func TestUserFieldMultiTogglesTwiceCleanly(t *testing.T) {
	m, fb := userModel(t, 6)
	m = pressDrain(t, m, "enter", " ", " ", " ", " ")
	m = pressDrain(t, m, "enter", "ctrl+s")
	if len(fb.edited) != 0 {
		t.Errorf("toggling twice should leave the field unchanged, got %v", fb.edited)
	}
	pressDrain(t, m, "enter", "enter", " ", " ", " ", "enter", "ctrl+s")
	if got := savedField(t, fb, "reviewers").Chosen; len(got) != 0 {
		t.Errorf("reviewers = %v", got)
	}
}

func TestUserSearchSkipsSupersededQueries(t *testing.T) {
	fb := &fakeBackend{users: []jira.Option{{ID: "u2", Name: "Bo"}}}
	s := newUserSearch(fb, "T-1")
	s.delay = 50 * time.Millisecond
	var wg sync.WaitGroup
	for _, q := range []string{"b", "bo", "bob"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.lookup(q)
		}()
		time.Sleep(5 * time.Millisecond)
	}
	wg.Wait()
	if !slices.Equal(fb.searches, []string{"bob"}) {
		t.Errorf("searches = %v", fb.searches)
	}
}

func TestUserFieldMultiRemovesCurrentPerson(t *testing.T) {
	m, fb := userModel(t, 6)
	pressDrain(t, m, "enter", " ", "enter", "ctrl+s")
	if got := savedField(t, fb, "reviewers").Chosen; len(got) != 0 {
		t.Errorf("reviewers = %v", got)
	}
}

func TestUserFieldShowsSearchError(t *testing.T) {
	m, fb := userModel(t, 6)
	fb.searchErr = errors.New("forbidden")
	m = typeText(t, m, "x")
	if !strings.Contains(m.View().Content, "forbidden") {
		t.Fatalf("expected the error:\n%s", m.View().Content)
	}
}

func TestUserFieldShowsNoMatches(t *testing.T) {
	m, fb := userModel(t, 6)
	fb.users = nil
	m = typeText(t, m, "x")
	if !strings.Contains(ansi.Strip(m.View().Content), "No matching") {
		t.Fatalf("expected a note that nobody matched:\n%s", m.View().Content)
	}
}

func TestFieldValueValidate(t *testing.T) {
	tests := []struct {
		kind     jira.FieldKind
		required bool
		text     string
		ok       bool
	}{
		{jira.FieldNumber, false, "3.5", true},
		{jira.FieldNumber, false, " 8 ", true},
		{jira.FieldNumber, false, "abc", false},
		{jira.FieldNumber, false, "", true},
		{jira.FieldNumber, true, "", false},
		{jira.FieldDate, false, "2026-10-26", true},
		{jira.FieldDate, false, "10/26/2026", false},
		{jira.FieldDate, false, "-2d", true},
		{jira.FieldDateTime, false, "2026-10-26T09:30", true},
		{jira.FieldDateTime, false, "1d12h", true},
		{jira.FieldDateTime, false, "soon", false},
		{jira.FieldDate, false, "2026-02-30", false},
		{jira.FieldDate, false, "", true},
	}
	for _, tt := range tests {
		v := &fieldValue{EditField: jira.EditField{Kind: tt.kind, Required: tt.required}}
		if err := v.validate(tt.text); (err == nil) != tt.ok {
			t.Errorf("validate(%v, required=%v, %q) = %v", tt.kind, tt.required, tt.text, err)
		}
	}
}

func TestNormalizeTime(t *testing.T) {
	now := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		kind jira.FieldKind
		in   string
		want string
	}{
		{jira.FieldDate, "-2d", "2026-10-25"},
		{jira.FieldDate, "2026-11-01", "2026-11-01"},
		{jira.FieldDateTime, "1d12h", "2026-10-28T21:00:00.000+0000"},
		{jira.FieldDateTime, "2026-10-26T09:30", "2026-10-26T09:30:00.000+0000"},
		{jira.FieldDateTime, "", ""},
	}
	for _, tt := range tests {
		v := &fieldValue{EditField: jira.EditField{Kind: tt.kind}, text: tt.in}
		v.normalizeTime(now)
		if v.text != tt.want {
			t.Errorf("normalizeTime(%v, %q) = %q, want %q", tt.kind, tt.in, v.text, tt.want)
		}
	}
}
