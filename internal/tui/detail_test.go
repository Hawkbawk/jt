package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/hawkbawk/jt/internal/components/issuecomments"
	"github.com/hawkbawk/jt/internal/components/issuedetail"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/tui/tuitest"
)

func plainMarkdown(md string, _ int) string { return md }

func detailBackend() *tuitest.Backend {
	return &tuitest.Backend{Detail: &jira.IssueDetail{
		Key:         "T-1",
		Summary:     "one",
		Status:      "Open",
		Description: strings.Repeat("line\n", 100) + strings.Repeat("wide", 100),
	}, CommentList: &jira.CommentList{Total: 2, Items: []jira.Comment{
		{Author: "Ada", Created: "2026-10-01 09:00", Body: "first comment"},
		{Author: "Grace", Created: "2026-10-02 10:00", Body: "second comment"},
	}}}
}

func loadedDetailModel(t *testing.T) BoardModel {
	t.Helper()
	fb := detailBackend()
	m := newTestModel(t, "TEST", fb)
	m.actions.MD = plainMarkdown
	return tuitest.Drain(t, m, m.Init())
}

func TestViewPopupLoadsAndScrolls(t *testing.T) {
	m := loadedDetailModel(t)
	m, cmd := pressOpen(t, m, "v")
	p, ok := m.Popup.Content().(*issuedetail.Model)
	if !ok || !p.Issue.Loading() {
		t.Fatal("v should open a loading detail popup")
	}
	m = tuitest.Drain(t, m, cmd)
	p = m.Popup.Content().(*issuedetail.Model)
	if p.Issue.Loading() || !strings.Contains(m.View().Content, "T-1") {
		t.Fatal("detail not loaded into popup")
	}

	m, _ = press(t, m, "down", "down")
	if p := m.Popup.Content().(*issuedetail.Model); p.Issue.ScrollPercent() == 0 {
		t.Fatalf("down arrow should scroll, scrolled to %v", p.Issue.ScrollPercent())
	}

	m, _ = press(t, m, "j")
	if m.board.nav.row != 0 {
		t.Fatal("board moved while popup was open")
	}
	m = tuitest.PressDrain(t, m, "esc")
	if m.Popup.IsOpen() {
		t.Fatal("esc should close the popup")
	}
}

func TestViewPopupTopAndBottom(t *testing.T) {
	m := loadedDetailModel(t)
	m = tuitest.PressDrain(t, m, "v")

	m, _ = press(t, m, "G")
	p := m.Popup.Content().(*issuedetail.Model)
	if p.Issue.ScrollPercent() != 1 {
		t.Fatalf("G should jump to the bottom, scrolled to %v", p.Issue.ScrollPercent())
	}
	if m.board.nav.row != 0 {
		t.Fatal("G moved the board while the popup was open")
	}

	m, _ = press(t, m, "g")
	if p := m.Popup.Content().(*issuedetail.Model); p.Issue.ScrollPercent() != 0 {
		t.Fatalf("g should jump to the top, scrolled to %v", p.Issue.ScrollPercent())
	}
	if v := ansi.Strip(m.Popup.View()); !strings.Contains(v, "g/G top/bottom") {
		t.Fatalf("footer is missing g/G:\n%s", v)
	}
}

func TestViewPopupHorizontalScroll(t *testing.T) {
	m := loadedDetailModel(t)
	m = tuitest.PressDrain(t, m, "v")
	before := m.Popup.Content().(*issuedetail.Model).View().Content
	m, _ = press(t, m, "right")
	if m.Popup.Content().(*issuedetail.Model).View().Content == before {
		t.Fatal("right arrow should scroll wide content horizontally")
	}
}

func TestViewIgnoredOnEmptyColumn(t *testing.T) {
	m := loadedDetailModel(t)
	m = tuitest.PressDrain(t, m, "l", "v")
	if m.Popup.IsOpen() {
		t.Fatal("popup opened without a selected issue")
	}
}

func TestCtrlPIgnoredWhilePopupOpen(t *testing.T) {
	m := loadedDetailModel(t)
	m = tuitest.PressDrain(t, m, "v", "ctrl+p")
	if _, ok := m.Popup.Content().(*issuedetail.Model); !ok {
		t.Fatal("ctrl+p should do nothing while a popup is open")
	}
}

func TestCommentsPopupShowsAllComments(t *testing.T) {
	m := loadedDetailModel(t)
	m = tuitest.PressDrain(t, m, "c")
	_, ok := m.Popup.Content().(*issuecomments.Model)
	if !ok {
		t.Fatal("c should open the comments popup")
	}
	content := ansi.Strip(m.Popup.View())
	for _, want := range []string{"Ada", "first comment", "Grace", "second comment"} {
		if !strings.Contains(content, want) {
			t.Errorf("comments missing %q", want)
		}
	}
	m = tuitest.PressDrain(t, m, "q")
	if m.Popup.IsOpen() {
		t.Fatal("q should close the popup")
	}
}
