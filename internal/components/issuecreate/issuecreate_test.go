package issuecreate

import (
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/tui/modal"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
	"github.com/hawkbawk/jt/internal/tui/tuitest"
)

var (
	typeTask = jira.IssueType{ID: "1", Name: "Task"}
	typeBug  = jira.IssueType{ID: "2", Name: "Bug"}
	typeOps  = jira.IssueType{ID: "3", Name: "Incident"}
)

func newBackend() *tuitest.Backend {
	return &tuitest.Backend{
		ProjectList: []jira.Project{{Key: "DEMO", Name: "Demo"}, {Key: "OPS", Name: "Operations"}},
		CreateTypes: map[string][]jira.IssueType{
			"DEMO": {typeTask, typeBug},
			"OPS":  {typeOps},
		},
		CreateFieldList: []jira.IssueField{
			{ID: "summary", Name: "Summary", Kind: jira.FieldText, Required: true},
			{ID: "description", Name: "Description", Kind: jira.FieldTextArea},
		},
		CreatedKey: "DEMO-9",
	}
}

type host struct {
	t      *testing.T
	m      *Model
	closed bool
	then   tea.Msg
}

func newHost(t *testing.T, b *tuitest.Backend, project string) *host {
	t.Helper()
	h := &host{t: t, m: New(b, project)}
	h.send(tea.WindowSizeMsg{Width: 96, Height: 28})
	h.drain(h.m.Init())
	return h
}

func (h *host) view() string { return ansi.Strip(tuitest.ModalView(h.m, h.m.width)) }

func (h *host) send(msg tea.Msg) {
	h.t.Helper()
	_, cmd := h.m.Update(msg)
	h.drain(cmd)
}

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
		case <-time.After(50 * time.Millisecond):
			continue
		}
		switch msg := msg.(type) {
		case tea.BatchMsg:
			queue = append(queue, msg...)
		case spinner.TickMsg:
		case modal.Closes:
			h.closed = true
			if _, ok := msg.(modal.ForParent); ok {
				h.then = msg
			}
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
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "ctrl+s":
		return tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
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

// fillSummary opens the summary field and types text into it.
func (h *host) fillSummary(text string) {
	h.t.Helper()
	h.press("enter")
	h.typeText(text)
	h.press("enter")
}

func TestPickerStartsOnTheGivenProject(t *testing.T) {
	b := newBackend()
	h := newHost(t, b, "OPS")
	if v := h.view(); !strings.Contains(v, "Select a project") {
		t.Fatalf("expected the project list:\n%s", v)
	}
	h.press("enter")
	v := h.view()
	if !strings.Contains(v, "Issue type") || !strings.Contains(v, "Incident") || strings.Contains(v, "Bug") {
		t.Fatalf("expected only the OPS issue types:\n%s", v)
	}
	if h.m.project != "OPS" || h.m.itype != typeOps || h.m.stage != stageType {
		t.Errorf("project = %v, type = %v, stage = %v", h.m.project, h.m.itype, h.m.stage)
	}
}

func TestPickerFallsBackToFirstProject(t *testing.T) {
	h := newHost(t, newBackend(), "NOPE")
	h.press("enter")
	if h.m.project != "DEMO" || h.m.itype != typeTask {
		t.Fatalf("project = %v, type = %v", h.m.project, h.m.itype)
	}
}

func TestPickingProjectLoadsOnlyItsIssueTypes(t *testing.T) {
	b := newBackend()
	h := newHost(t, b, "DEMO")
	h.press("j", "enter")
	if h.m.project != "OPS" || h.m.itype != typeOps {
		t.Fatalf("project = %v, type = %v", h.m.project, h.m.itype)
	}
	if got := strings.Join(b.CreateTypeCalls, ","); got != "OPS" {
		t.Errorf("issue types should load for the picked project, got %q", got)
	}
}

func TestCreateSendsEnteredFields(t *testing.T) {
	b := newBackend()
	h := newHost(t, b, "DEMO")
	h.press("enter", "enter")
	if !strings.Contains(h.m.Title(), "Create · DEMO · Task") {
		t.Fatalf("title = %q", h.m.Title())
	}
	h.fillSummary("Broken login")
	h.press("ctrl+s")

	if len(b.Creates) != 1 {
		t.Fatalf("creates = %v", b.Creates)
	}
	c := b.Creates[0]
	if c.Project != "DEMO" || c.IssueType != "1" || len(c.Fields) != 1 || c.Fields[0].Text != "Broken login" {
		t.Errorf("created %+v", c)
	}
	done, ok := h.then.(tuimsg.Finished)
	if !h.closed || !ok || done.IssueKey() != "DEMO-9" || done.Status() != "Created DEMO-9" {
		t.Fatalf("modal should close with a finished message, got closed=%v then=%v", h.closed, h.then)
	}
}

func TestCreateNeedsRequiredFields(t *testing.T) {
	b := newBackend()
	h := newHost(t, b, "DEMO")
	h.press("enter", "enter", "ctrl+s")
	if len(b.Creates) != 0 || h.closed {
		t.Fatal("nothing should be created without the required fields")
	}
	if v := h.view(); !strings.Contains(v, "required: Summary") {
		t.Fatalf("expected the missing field to be named:\n%s", v)
	}
}

func TestCreateErrorKeepsValuesForRetry(t *testing.T) {
	b := newBackend()
	b.CreateErr = errNope
	h := newHost(t, b, "DEMO")
	h.press("enter", "enter")
	h.fillSummary("Broken")
	h.press("ctrl+s")
	if h.closed || !strings.Contains(h.view(), "nope") {
		t.Fatalf("expected the error with the modal still open:\n%s", h.view())
	}
	b.CreateErr = nil
	h.press("ctrl+s")
	if !h.closed || len(b.Creates) != 1 {
		t.Fatalf("retry should create, closed=%v creates=%v", h.closed, b.Creates)
	}
}

func TestBackReturnsToIssueTypeThenProject(t *testing.T) {
	h := newHost(t, newBackend(), "DEMO")
	h.press("enter", "enter", "shift+tab")
	if h.m.stage != stageType || h.closed {
		t.Fatalf("stage = %v closed = %v", h.m.stage, h.closed)
	}
	if v := h.view(); !strings.Contains(v, "Issue type") {
		t.Fatalf("expected the issue type form:\n%s", v)
	}
	h.press("shift+tab")
	if h.m.stage != stageProject || !strings.Contains(h.view(), "Select a project") {
		t.Fatalf("expected the project list, stage = %v\n%s", h.m.stage, h.view())
	}
}

func TestBackWithEnteredValuesAsksFirst(t *testing.T) {
	h := newHost(t, newBackend(), "DEMO")
	h.press("enter", "enter")
	h.fillSummary("Draft")
	h.press("shift+tab")
	if h.m.stage != stageFields || h.m.confirm != confirmBack {
		t.Fatalf("expected a confirmation, stage = %v confirm = %v", h.m.stage, h.m.confirm)
	}
	h.press("n")
	if h.m.stage != stageFields || h.m.confirm != confirmNone {
		t.Fatal("n should keep editing")
	}
	h.press("shift+tab", "y")
	if h.m.stage != stageType {
		t.Fatal("y should go back")
	}
}

func TestEscGoesBackOneStageAndClosesFromTheProjectList(t *testing.T) {
	h := newHost(t, newBackend(), "DEMO")
	h.press("enter", "enter", "esc")
	if h.closed || h.m.stage != stageType {
		t.Fatalf("esc on the fields should go back to the type, closed=%v stage=%v", h.closed, h.m.stage)
	}
	h.press("esc")
	if h.closed || h.m.stage != stageProject {
		t.Fatalf("esc on the type should go back to the project, closed=%v stage=%v", h.closed, h.m.stage)
	}
	h.press("esc")
	if !h.closed {
		t.Fatal("esc on the project list should close")
	}
}

func TestEscAsksBeforeDroppingEnteredValues(t *testing.T) {
	h := newHost(t, newBackend(), "DEMO")
	h.press("enter", "enter")
	h.fillSummary("Draft")
	h.press("esc")
	if h.closed || h.m.confirm != confirmBack {
		t.Fatal("esc should ask before dropping what was entered")
	}
	h.press("y")
	if h.closed || h.m.stage != stageType {
		t.Fatalf("y should go back, closed=%v stage=%v", h.closed, h.m.stage)
	}
}

func TestQuitKeyClosesFromTheFields(t *testing.T) {
	h := newHost(t, newBackend(), "DEMO")
	h.press("enter", "enter", "q")
	if !h.closed {
		t.Fatal("q should close")
	}
}

func TestHelpIsShownOnce(t *testing.T) {
	h := newHost(t, newBackend(), "DEMO")
	h.press("enter")
	if n := strings.Count(h.view(), "back"); n != 1 {
		t.Fatalf("expected the help once, found %d:\n%s", n, h.view())
	}
}

func TestProjectWithoutIssueTypesCannotContinue(t *testing.T) {
	b := newBackend()
	b.CreateTypes["DEMO"] = nil
	h := newHost(t, b, "DEMO")
	h.press("enter", "enter")
	if h.m.stage != stageProject || !strings.Contains(h.view(), "no issue types") {
		t.Fatalf("expected an error on the project list, stage = %v\n%s", h.m.stage, h.view())
	}
}

var errNope = errString("nope")

type errString string

func (e errString) Error() string { return string(e) }
