package issueedit

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/hawkbawk/jt/internal/components/usersearch"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/tui/popup"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
)

func TestMain(m *testing.M) {
	usersearch.Delay = 0
	os.Exit(m.Run())
}

type fakeBackend struct {
	fields    []jira.EditField
	editErr   error
	edited    [][]jira.EditField
	users     []jira.Option
	searches  []string
	searchErr error
}

func (f *fakeBackend) EditFields(jira.Issue) ([]jira.EditField, error) { return f.fields, nil }

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

func testFields() []jira.EditField {
	return []jira.EditField{
		{ID: "summary", Name: "Summary", Kind: jira.FieldText, Required: true, Text: "Fix it"},
		{ID: "description", Name: "Description", Kind: jira.FieldTextArea, Text: "Steps"},
		{
			ID: "priority", Name: "Priority", Kind: jira.FieldSelect, Chosen: []jira.Option{{ID: "3", Name: "Medium"}},
			Options: []jira.Option{{ID: "1", Name: "High"}, {ID: "3", Name: "Medium"}},
		},
		{ID: "labels", Name: "Labels", Kind: jira.FieldLabels, Chosen: []jira.Option{{ID: "api", Name: "api"}}},
		{
			ID: "components", Name: "Components", Kind: jira.FieldMultiSelect,
			Options: []jira.Option{{ID: "c1", Name: "API"}, {ID: "c2", Name: "UI"}},
		},
		{
			ID: "owner", Name: "Owner", Kind: jira.FieldUser, Chosen: []jira.Option{{ID: "u1", Name: "Ada"}},
		},
		{
			ID: "reviewers", Name: "Reviewers", Kind: jira.FieldMultiUser, Chosen: []jira.Option{{ID: "u1", Name: "Ada"}},
		},
	}
}

// host stands in for the popup host: it sizes the model, feeds back the
// messages its commands produce, and notes when it closes.
type host struct {
	t      *testing.T
	m      *Model
	closed bool
	then   tea.Msg
}

const cmdDeadline = 50 * time.Millisecond

func newHost(t *testing.T, fb *fakeBackend) *host {
	t.Helper()
	h := &host{t: t, m: New(fb, jira.Issue{Key: "T-2"})}
	h.send(tea.WindowSizeMsg{Width: 96, Height: 28})
	h.drain(h.m.Init())
	return h
}

func fieldsHost(t *testing.T) (*host, *fakeBackend) {
	t.Helper()
	fb := &fakeBackend{fields: testFields()}
	return newHost(t, fb), fb
}

func (h *host) open() bool { return !h.closed }

func (h *host) view() string { return h.m.View().Content }

func (h *host) send(msg tea.Msg) {
	h.t.Helper()
	_, cmd := h.m.Update(msg)
	h.drain(cmd)
}

// drain runs cmd and feeds the messages it produces back to the model. Timers
// such as spinner ticks and cursor blinks outlast the deadline and are dropped.
func (h *host) drain(cmd tea.Cmd) {
	h.t.Helper()
	queue := []tea.Cmd{cmd}
	for steps := 0; len(queue) > 0; steps++ {
		if steps > 1000 {
			h.t.Fatal("drain: too many commands, likely a message loop")
		}
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		done := make(chan tea.Msg, 1)
		go func() { done <- c() }()
		var msg tea.Msg
		select {
		case msg = <-done:
		case <-time.After(cmdDeadline):
			continue
		}
		switch msg := msg.(type) {
		case tea.BatchMsg:
			queue = append(queue, msg...)
		case spinner.TickMsg:
		case popup.CloseMsg:
			h.closed, h.then = true, msg.Then
		default:
			_, next := h.m.Update(msg)
			queue = append(queue, next)
		}
	}
}

func keyMsg(s string) tea.KeyPressMsg {
	switch s {
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEsc}
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
	case "ctrl+s", "ctrl+j":
		return tea.KeyPressMsg{Code: rune(s[len(s)-1]), Mod: tea.ModCtrl}
	}
	return tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
}

func (h *host) press(keys ...string) {
	h.t.Helper()
	for _, k := range keys {
		h.send(keyMsg(k))
	}
}

func (h *host) typeText(s string) {
	h.t.Helper()
	for _, r := range s {
		h.press(string(r))
	}
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

// chosen is the ids of the options saved for the field with id.
func chosen(t *testing.T, fb *fakeBackend, id string) []string {
	t.Helper()
	var ids []string
	for _, o := range savedField(t, fb, id).Chosen {
		ids = append(ids, o.ID)
	}
	return ids
}

func TestFieldsSaveChangesTogether(t *testing.T) {
	h, fb := fieldsHost(t)
	h.press("enter")
	h.typeText(" now")
	h.press("enter", "j", "j", "enter", "k", "enter", "j", "enter")
	h.typeText(" ui")
	h.press("enter", "j", "enter", " ", "enter")
	if len(fb.edited) != 0 {
		t.Fatal("nothing should be saved before ctrl+s")
	}
	h.press("ctrl+s")

	if len(fb.edited[0]) != 4 {
		t.Errorf("only changed fields should be sent, got %d", len(fb.edited[0]))
	}
	if got := savedField(t, fb, "summary").Text; got != "Fix it now" {
		t.Errorf("summary = %q", got)
	}
	if got := chosen(t, fb, "priority"); !slices.Equal(got, []string{"1"}) {
		t.Errorf("priority = %v", got)
	}
	if got := chosen(t, fb, "labels"); !slices.Equal(got, []string{"api", "ui"}) {
		t.Errorf("labels = %v", got)
	}
	if got := chosen(t, fb, "components"); !slices.Equal(got, []string{"c1"}) {
		t.Errorf("components = %v", got)
	}
	done, ok := h.then.(tuimsg.Finished)
	if h.open() || !ok || done.IssueKey() != "T-2" || done.Status() != "Updated 4 fields on T-2" {
		t.Fatalf("popup should close with a finished message, got open=%v then=%v", h.open(), h.then)
	}
}

func TestFieldsFormKeepsKeysThatWouldClose(t *testing.T) {
	h, fb := fieldsHost(t)
	h.press("enter")
	h.typeText("qhl/")
	if !h.open() {
		t.Fatal("typing should stay in the form")
	}
	h.press("enter", "ctrl+s")
	if got := savedField(t, fb, "summary").Text; got != "Fix itqhl/" {
		t.Errorf("summary = %q", got)
	}
}

func TestFieldsEscRevertsEditEnterKeepsIt(t *testing.T) {
	h, fb := fieldsHost(t)
	h.press("enter")
	h.typeText(" now")
	h.press("esc", "ctrl+s")
	if len(fb.edited) != 0 {
		t.Fatalf("esc should drop the edit, got %v", fb.edited)
	}
	h.press("enter")
	h.typeText(" now")
	h.press("enter", "enter")
	h.typeText(" more")
	h.press("esc", "ctrl+s")
	if got := savedField(t, fb, "summary").Text; got != "Fix it now" {
		t.Errorf("summary = %q", got)
	}
}

func TestFieldsCtrlSDoesNothingInsideForm(t *testing.T) {
	h, fb := fieldsHost(t)
	h.press("enter")
	h.typeText("!")
	h.press("ctrl+s")
	if len(fb.edited) != 0 || !h.open() {
		t.Fatalf("ctrl+s should only save from the list, got %v", fb.edited)
	}
}

func TestFieldsEscGoesBackToListThenCloses(t *testing.T) {
	h, _ := fieldsHost(t)
	h.press("enter")
	if v := h.view(); strings.Contains(v, "Description") {
		t.Fatal("only the selected field should show")
	}
	h.press("esc")
	if v := h.view(); !h.open() || !strings.Contains(v, "Description") || !strings.Contains(v, "Components") {
		t.Fatal("esc should go back to the list of fields")
	}
	if h.press("esc"); h.open() {
		t.Fatal("esc on the list should close the popup")
	}
}

func TestFieldsListShowsEnteredValues(t *testing.T) {
	h, _ := fieldsHost(t)
	h.press("enter")
	h.typeText("!")
	h.press("enter")
	if v := h.view(); !strings.Contains(v, "Fix it!") || !strings.Contains(v, "Summary * ●") {
		t.Fatal("list should show the new value and mark the field as changed")
	}
}

func TestFieldsUndoResetsSelectedField(t *testing.T) {
	h, fb := fieldsHost(t)
	h.press("enter")
	h.typeText("!")
	h.press("esc", "j", "j", "enter", "k", "enter", "k", "k", "u")
	if v := h.view(); !strings.Contains(v, "1 unsaved change") || strings.Contains(v, "Summary * ●") {
		t.Fatal("u should reset the selected field and leave the other change")
	}
	h.press("j", "j", "u", "ctrl+s")
	if len(fb.edited) != 0 {
		t.Fatalf("reset fields shouldn't be saved, got %v", fb.edited)
	}
}

func TestFieldsFilterNarrowsList(t *testing.T) {
	h, fb := fieldsHost(t)
	h.press("/")
	h.typeText("lab")
	h.press("enter")
	if v := ansi.Strip(h.view()); strings.Contains(v, "Summary") || !strings.Contains(v, "Labels") {
		t.Fatal("filter should leave only matching fields")
	}
	h.press("enter")
	h.typeText(" ui")
	h.press("enter", "ctrl+s")
	if got := chosen(t, fb, "labels"); !slices.Equal(got, []string{"api", "ui"}) {
		t.Errorf("labels = %v", got)
	}
}

func TestFieldsEscClearsFilterBeforeClosing(t *testing.T) {
	h, _ := fieldsHost(t)
	h.press("/")
	h.typeText("q")
	h.press("esc")
	if !h.open() || !strings.Contains(h.view(), "Summary") {
		t.Fatal("esc should clear the filter and keep the popup open")
	}
}

func TestFieldsTextAreaTakesNewlines(t *testing.T) {
	h, fb := fieldsHost(t)
	h.press("j", "enter")
	h.typeText(" one")
	h.press("ctrl+j")
	h.typeText("two")
	h.press("enter", "ctrl+s")
	if got := savedField(t, fb, "description").Text; got != "Steps one\ntwo" {
		t.Errorf("description = %q", got)
	}
}

func TestFieldsSelectCanClearOptionalField(t *testing.T) {
	h, fb := fieldsHost(t)
	h.press("j", "j", "enter", "k", "k", "enter", "ctrl+s")
	if got := chosen(t, fb, "priority"); len(got) != 0 {
		t.Errorf("priority = %v", got)
	}
}

func TestFieldsSaveErrorKeepsChangesForRetry(t *testing.T) {
	h, fb := fieldsHost(t)
	fb.editErr = errors.New("summary: too long")
	h.press("enter")
	h.typeText("!")
	h.press("enter", "ctrl+s")
	if !h.open() || !strings.Contains(h.view(), "summary: too long") || len(fb.edited) != 0 {
		t.Fatal("failed save should leave the popup open with the error")
	}
	fb.editErr = nil
	h.press("ctrl+s")
	if got := savedField(t, fb, "summary").Text; got != "Fix it!" || h.open() {
		t.Errorf("retry should save the kept change, got %q", got)
	}
}

func TestFieldsListScrolls(t *testing.T) {
	fb := &fakeBackend{}
	for i := range 40 {
		fb.fields = append(fb.fields, jira.EditField{ID: fmt.Sprint(i), Name: fmt.Sprintf("Field %02d", i), Kind: jira.FieldText})
	}
	h := newHost(t, fb)
	if v := h.view(); !strings.Contains(v, "Field 00") || strings.Contains(v, "Field 39") {
		t.Fatal("list should start at the top")
	}
	h.press("G")
	if v := h.view(); !strings.Contains(v, "Field 39") || strings.Contains(v, "Field 00") {
		t.Fatal("list should scroll to the last field")
	}
}

func TestFieldsCloseWithoutChangesNeedsNoConfirm(t *testing.T) {
	h, _ := fieldsHost(t)
	if h.press("esc"); h.open() {
		t.Fatal("esc should close a popup with nothing to save")
	}
}

func TestFieldsCloseWithChangesAsksFirst(t *testing.T) {
	h, fb := fieldsHost(t)
	h.press("enter")
	h.typeText("!")
	if v := h.view(); strings.Contains(v, "unsaved") || strings.Contains(v, "ctrl+s") {
		t.Fatal("what is still being entered shouldn't count or offer saving")
	}
	h.press("enter")
	if v := h.view(); !strings.Contains(v, "1 unsaved change") || !strings.Contains(v, "ctrl+s") {
		t.Fatal("kept changes should be called out")
	}
	h.press("esc")
	if !h.open() || !strings.Contains(h.view(), "Discard 1 unsaved change?") {
		t.Fatal("esc should ask before dropping changes")
	}
	h.press("n")
	if !h.open() || h.m.unsaved() != 1 {
		t.Fatal("n should keep editing with the changes intact")
	}
	h.press("esc", "y")
	if h.open() || len(fb.edited) != 0 {
		t.Fatal("y should close without saving")
	}
}

// userModel opens the user field that is field rows down the list.
func userModel(t *testing.T, field int) (*host, *fakeBackend) {
	t.Helper()
	h, fb := fieldsHost(t)
	fb.users = []jira.Option{{ID: "u2", Name: "Bo"}, {ID: "u3", Name: "Cy"}}
	for range field {
		h.press("down")
	}
	h.press("enter")
	return h, fb
}

func TestUserFieldSearchesAndPicksOne(t *testing.T) {
	h, fb := userModel(t, 5)
	h.typeText("bo")
	if !slices.Equal(fb.searches, []string{"b", "bo"}) {
		t.Fatalf("searches = %v", fb.searches)
	}
	h.press("enter", "down", "enter")
	if !strings.Contains(h.view(), "Cy") {
		t.Fatalf("expected the picked person in the list:\n%s", h.view())
	}
	h.press("ctrl+s")
	if got := chosen(t, fb, "owner"); !slices.Equal(got, []string{"u3"}) {
		t.Errorf("owner = %v", got)
	}
}

func TestUserFieldShowsCurrentPeopleAndCanClear(t *testing.T) {
	h, fb := userModel(t, 5)
	if !strings.Contains(h.view(), "(none)") || !strings.Contains(h.view(), "Ada") {
		t.Fatalf("expected the current person and a way to clear:\n%s", h.view())
	}
	h.press("enter", "up", "enter", "ctrl+s")
	if got := chosen(t, fb, "owner"); len(got) != 0 {
		t.Errorf("owner = %v", got)
	}
}

func TestUserFieldEscKeepsCurrentPerson(t *testing.T) {
	h, fb := userModel(t, 5)
	h.typeText("bo")
	h.press("enter", "esc", "ctrl+s")
	if len(fb.edited) != 0 || !h.open() {
		t.Fatalf("esc should leave the field unchanged, got %v", fb.edited)
	}
}

func TestUserFieldMultiKeepsPicksAcrossSearches(t *testing.T) {
	h, fb := userModel(t, 6)
	h.typeText("c")
	h.press("enter", "down", "down", " ")
	h.press("shift+tab", "backspace")
	h.typeText("b")
	h.press("enter", "enter")
	if !strings.Contains(h.view(), "Ada, Cy") {
		t.Fatalf("expected the picked person's name in the list:\n%s", h.view())
	}
	h.press("ctrl+s")
	if got := chosen(t, fb, "reviewers"); !slices.Equal(got, []string{"u1", "u3"}) {
		t.Errorf("reviewers = %v", got)
	}
}

func TestUserFieldMultiTogglesTwiceCleanly(t *testing.T) {
	h, fb := userModel(t, 6)
	h.press("enter", " ", " ", " ", " ")
	h.press("enter", "ctrl+s")
	if len(fb.edited) != 0 {
		t.Errorf("toggling twice should leave the field unchanged, got %v", fb.edited)
	}
	h.press("enter", "enter", " ", " ", " ", "enter", "ctrl+s")
	if got := chosen(t, fb, "reviewers"); len(got) != 0 {
		t.Errorf("reviewers = %v", got)
	}
}

func TestUserFieldMultiRemovesCurrentPerson(t *testing.T) {
	h, fb := userModel(t, 6)
	h.press("enter", " ", "enter", "ctrl+s")
	if got := chosen(t, fb, "reviewers"); len(got) != 0 {
		t.Errorf("reviewers = %v", got)
	}
}

func TestUserFieldShowsSearchError(t *testing.T) {
	h, fb := userModel(t, 6)
	fb.searchErr = errors.New("forbidden")
	h.typeText("x")
	if !strings.Contains(h.view(), "forbidden") {
		t.Fatalf("expected the error:\n%s", h.view())
	}
}

func TestUserFieldRetriesFailedSearch(t *testing.T) {
	h, fb := userModel(t, 6)
	fb.searchErr = errors.New("timeout")
	h.typeText("x")
	fb.searchErr = nil
	h.press("backspace")
	h.typeText("x")
	if !slices.Equal(fb.searches, []string{"x", "x"}) || strings.Contains(h.view(), "timeout") {
		t.Fatalf("searches = %v, view:\n%s", fb.searches, h.view())
	}
}

func TestUserFieldShowsNoMatches(t *testing.T) {
	h, fb := userModel(t, 6)
	fb.users = nil
	h.typeText("x")
	if !strings.Contains(ansi.Strip(h.view()), "No matching") {
		t.Fatalf("expected a note that nobody matched:\n%s", h.view())
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
