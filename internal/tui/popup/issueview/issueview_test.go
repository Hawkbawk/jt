package issueview

import (
	"errors"
	"strings"
	"testing"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/hawkbawk/jt/internal/jira"
)

type fakeBackend struct {
	detail *jira.IssueDetail
	err    error
}

func (f fakeBackend) Issue(string) (*jira.IssueDetail, error) { return f.detail, f.err }

func plain(md string, _ int) string { return md }

func detail() *jira.IssueDetail {
	return &jira.IssueDetail{
		Issue:       jira.Issue{Key: "T-1", Summary: "one", Status: "Open"},
		Description: strings.Repeat("line\n", 100),
		Comments: []jira.Comment{
			{Author: "Ada", Created: "2026-10-01 09:00", Body: "first comment"},
			{Author: "Grace", Created: "2026-10-02 10:00", Body: "second comment"},
		},
	}
}

func sized(m *Model, w, h int) *Model {
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m
}

// loaded runs the model's load and hands the response back to it.
func loaded(m *Model) *Model {
	m.Update(m.Reload(false)())
	return m
}

func press(m *Model, k string) {
	msg := tea.KeyPressMsg{Code: rune(k[0]), Text: k}
	m.Update(msg)
}

func view(m *Model) string { return ansi.Strip(m.View().Content) }

func TestStartsLoading(t *testing.T) {
	m := sized(Details(fakeBackend{detail: detail()}, "T-1", plain), 40, 10)
	if !m.Loading() || m.Ready() || !strings.Contains(view(m), "Loading") {
		t.Fatal("a new model should be loading")
	}
}

func TestLoadShowsDetailsAndRerendersOnResize(t *testing.T) {
	var widths []int
	md := func(s string, w int) string { widths = append(widths, w); return s }
	m := sized(Details(fakeBackend{detail: detail()}, "T-1", md), 40, 10)
	loaded(m)
	sized(m, 30, 10)
	if m.Loading() || !m.Ready() || len(widths) != 2 || widths[0] != 40 || widths[1] != 30 {
		t.Fatalf("text should render at each width, got %v", widths)
	}
	if v := view(m); !strings.Contains(v, "Details · T-1") || !strings.Contains(v, "one") {
		t.Fatalf("missing title or summary:\n%s", v)
	}
}

func TestCommentsShowEveryComment(t *testing.T) {
	m := sized(Comments(fakeBackend{detail: detail()}, "T-1", plain), 60, 20)
	loaded(m)
	v := view(m)
	for _, want := range []string{"Comments · T-1", "Ada", "first comment", "Grace", "second comment"} {
		if !strings.Contains(v, want) {
			t.Errorf("comments missing %q:\n%s", want, v)
		}
	}
}

func TestNoComments(t *testing.T) {
	if got := renderComments(&jira.IssueDetail{}, 40, plain); !strings.Contains(got, "No comments") {
		t.Fatalf("got %q", got)
	}
}

func TestLoadErrorIsShown(t *testing.T) {
	m := sized(Details(fakeBackend{err: errors.New("boom")}, "T-1", plain), 40, 10)
	loaded(m)
	if v := view(m); m.Ready() || !strings.Contains(v, "Error: boom") {
		t.Fatalf("unexpected error view:\n%s", v)
	}
}

func TestLoadForAnotherModelIsIgnored(t *testing.T) {
	m := sized(Details(fakeBackend{detail: detail()}, "T-1", plain), 40, 10)
	m.Update(loadedMsg{id: m.id - 1, issue: &jira.IssueDetail{}})
	m.Update(loadedMsg{id: m.id - 1, err: errors.New("boom")})
	if !m.Loading() || m.err != nil {
		t.Fatal("model accepted an issue it did not request")
	}
}

func TestScrollKeys(t *testing.T) {
	m := sized(Details(fakeBackend{detail: detail()}, "T-1", plain), 40, 10)
	loaded(m)
	press(m, "G")
	if m.ScrollPercent() != 1 {
		t.Fatalf("G should scroll to the end, at %v", m.ScrollPercent())
	}
	press(m, "g")
	if m.ScrollPercent() != 0 {
		t.Fatalf("g should scroll to the top, at %v", m.ScrollPercent())
	}
}

func TestReloadScrollsToEndOnRequest(t *testing.T) {
	m := sized(Details(fakeBackend{detail: detail()}, "T-1", plain), 40, 10)
	loaded(m)
	m.Update(m.Reload(false)())
	if m.ScrollPercent() != 0 {
		t.Fatalf("a plain reload should stay at the top, at %v", m.ScrollPercent())
	}
	m.Update(m.Reload(true)())
	if m.ScrollPercent() != 1 {
		t.Fatalf("reload with toEnd should scroll to the end, at %v", m.ScrollPercent())
	}
}

func TestViewFillsTheAreaItIsSizedFor(t *testing.T) {
	m := sized(Details(fakeBackend{detail: detail()}, "T-1", plain), 40, 20)
	loaded(m)
	if h := strings.Count(m.View().Content, "\n") + 1; h != 20 {
		t.Fatalf("view should fill the area, got %d lines", h)
	}
}

func TestSpinnerStopsWhenIdleAndRestarts(t *testing.T) {
	m := loaded(Details(fakeBackend{detail: detail()}, "T-1", plain))
	if cmd := m.SetBusy(true); cmd == nil {
		t.Fatal("becoming busy should restart the spinner")
	}
	if cmd := m.SetBusy(true); cmd != nil {
		t.Fatal("already busy, so nothing to restart")
	}
	m.SetBusy(false)
	if _, cmd := m.Update(spinner.TickMsg{}); cmd != nil {
		t.Fatal("the tick chain should end when nothing is busy")
	}
}
