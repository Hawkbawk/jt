package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/hawkbawk/jt/internal/jira"
)

func plainMarkdown(md string, _ int) string { return md }

func detailBackend() *fakeBackend {
	return &fakeBackend{detail: &jira.IssueDetail{
		Issue:       jira.Issue{Key: "T-1", Summary: "one", Status: "Open"},
		Description: strings.Repeat("line\n", 100) + strings.Repeat("wide", 100),
		Comments: []jira.Comment{
			{Author: "Ada", Created: "2026-10-01 09:00", Body: "first comment"},
			{Author: "Grace", Created: "2026-10-02 10:00", Body: "second comment"},
		},
	}}
}

func loadedDetailModel(t *testing.T) BoardModel {
	t.Helper()
	fb := detailBackend()
	m := newTestModel(t, "TEST", fb)
	m.actions.md = plainMarkdown
	return drain(t, m, m.Init())
}

func TestViewPopupLoadsAndScrolls(t *testing.T) {
	m := loadedDetailModel(t)
	m, cmd := pressOpen(t, m, "v")
	p, ok := m.popup.Content().(*detailPopup)
	if !ok || !p.issue.Loading() {
		t.Fatal("v should open a loading detail popup")
	}
	m = drain(t, m, cmd)
	p = m.popup.Content().(*detailPopup)
	if p.issue.Loading() || !strings.Contains(m.View().Content, "T-1") {
		t.Fatal("detail not loaded into popup")
	}

	m, _ = press(t, m, "down", "down")
	if p := m.popup.Content().(*detailPopup); p.issue.ScrollPercent() == 0 {
		t.Fatalf("down arrow should scroll, scrolled to %v", p.issue.ScrollPercent())
	}

	m, _ = press(t, m, "j")
	if m.board.nav.row != 0 {
		t.Fatal("board moved while popup was open")
	}
	m = pressDrain(t, m, "esc")
	if m.popup.IsOpen() {
		t.Fatal("esc should close the popup")
	}
}

func TestViewPopupTopAndBottom(t *testing.T) {
	m := loadedDetailModel(t)
	m = pressDrain(t, m, "v")

	m, _ = press(t, m, "G")
	p := m.popup.Content().(*detailPopup)
	if p.issue.ScrollPercent() != 1 {
		t.Fatalf("G should jump to the bottom, scrolled to %v", p.issue.ScrollPercent())
	}
	if m.board.nav.row != 0 {
		t.Fatal("G moved the board while the popup was open")
	}

	m, _ = press(t, m, "g")
	if p := m.popup.Content().(*detailPopup); p.issue.ScrollPercent() != 0 {
		t.Fatalf("g should jump to the top, scrolled to %v", p.issue.ScrollPercent())
	}
	if v := ansi.Strip(m.popup.View()); !strings.Contains(v, "g/G top/bottom") {
		t.Fatalf("footer is missing g/G:\n%s", v)
	}
}

func TestViewPopupHorizontalScroll(t *testing.T) {
	m := loadedDetailModel(t)
	m = pressDrain(t, m, "v")
	before := m.popup.Content().(*detailPopup).View().Content
	m, _ = press(t, m, "right")
	if m.popup.Content().(*detailPopup).View().Content == before {
		t.Fatal("right arrow should scroll wide content horizontally")
	}
}

func TestViewIgnoredOnEmptyColumn(t *testing.T) {
	m := loadedDetailModel(t)
	m = pressDrain(t, m, "l", "v")
	if m.popup.IsOpen() {
		t.Fatal("popup opened without a selected issue")
	}
}

func TestCtrlPIgnoredWhilePopupOpen(t *testing.T) {
	m := loadedDetailModel(t)
	m = pressDrain(t, m, "v", "ctrl+p")
	if _, ok := m.popup.Content().(*detailPopup); !ok {
		t.Fatal("ctrl+p should do nothing while a popup is open")
	}
}

func TestCommentsPopupShowsAllComments(t *testing.T) {
	m := loadedDetailModel(t)
	m = pressDrain(t, m, "c")
	_, ok := m.popup.Content().(*commentsPopup)
	if !ok {
		t.Fatal("c should open the comments popup")
	}
	content := ansi.Strip(m.popup.View())
	for _, want := range []string{"Ada", "first comment", "Grace", "second comment"} {
		if !strings.Contains(content, want) {
			t.Errorf("comments missing %q", want)
		}
	}
	m = pressDrain(t, m, "q")
	if m.popup.IsOpen() {
		t.Fatal("q should close the popup")
	}
}
