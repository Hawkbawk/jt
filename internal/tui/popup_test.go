package tui

import (
	"strings"
	"testing"

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

func loadedDetailModel(t *testing.T) Model {
	t.Helper()
	fb := detailBackend()
	m := newTestModel(t, testConfig(), fb)
	m.md = plainMarkdown
	return drain(t, m, m.Init())
}

func TestViewPopupLoadsAndScrolls(t *testing.T) {
	m := loadedDetailModel(t)
	m, cmd := press(t, m, "v")
	p, ok := m.popup.(*textPopup)
	if !ok || !p.loading() {
		t.Fatal("v should open a loading detail popup")
	}
	m = drain(t, m, cmd)
	p = m.popup.(*textPopup)
	if p.detail == nil || !strings.Contains(m.View(), "T-1") {
		t.Fatal("detail not loaded into popup")
	}

	m, _ = press(t, m, "down", "down")
	if p := m.popup.(*textPopup); p.vp.YOffset != 2 {
		t.Fatalf("down arrow should scroll, YOffset=%d", p.vp.YOffset)
	}

	m, _ = press(t, m, "j")
	if m.nav.row != 0 {
		t.Fatal("board moved while popup was open")
	}
	m, _ = press(t, m, "esc")
	if m.popup != nil {
		t.Fatal("esc should close the popup")
	}
}

func TestViewPopupHorizontalScroll(t *testing.T) {
	m := loadedDetailModel(t)
	m, cmd := press(t, m, "v")
	m = drain(t, m, cmd)
	before := m.popup.(*textPopup).vp.View()
	m, _ = press(t, m, "right")
	if m.popup.(*textPopup).vp.View() == before {
		t.Fatal("right arrow should scroll wide content horizontally")
	}
}

func TestViewIgnoredOnEmptyColumn(t *testing.T) {
	m := loadedDetailModel(t)
	m, _ = press(t, m, "l", "v")
	if m.popup != nil {
		t.Fatal("popup opened without a selected issue")
	}
}

func TestStaleIssueLoadIsIgnored(t *testing.T) {
	m := loadedDetailModel(t)
	m, _ = press(t, m, "v")
	next, _ := m.Update(issueLoadedMsg{key: "OTHER-1", detail: &jira.IssueDetail{}})
	if !next.(Model).popup.(*textPopup).loading() {
		t.Fatal("popup accepted an issue it did not request")
	}
}

func TestCtrlPClosesPopup(t *testing.T) {
	m := loadedDetailModel(t)
	m, _ = press(t, m, "v", "ctrl+p")
	if m.popup != nil || !m.pickerOpen {
		t.Fatal("ctrl+p should replace the popup with the picker")
	}
}

func TestCommentsPopupShowsAllComments(t *testing.T) {
	m := loadedDetailModel(t)
	m, cmd := press(t, m, "c")
	m = drain(t, m, cmd)
	p, ok := m.popup.(*textPopup)
	if !ok || p.title != "Comments" {
		t.Fatal("c should open the comments popup")
	}
	content := renderComments(p.detail, 40, plainMarkdown)
	for _, want := range []string{"Ada", "first comment", "Grace", "second comment"} {
		if !strings.Contains(content, want) {
			t.Errorf("comments missing %q", want)
		}
	}
	m, _ = press(t, m, "q")
	if m.popup != nil {
		t.Fatal("q should close the popup")
	}
}

func TestRenderCommentsEmpty(t *testing.T) {
	if got := renderComments(&jira.IssueDetail{}, 40, plainMarkdown); !strings.Contains(got, "No comments") {
		t.Fatalf("got %q", got)
	}
}
