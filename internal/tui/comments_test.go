package tui

import (
	"errors"
	"strings"
	"testing"
)

func openComments(t *testing.T, fb *fakeBackend) BoardModel {
	t.Helper()
	m := newTestModel(t, "TEST", fb)
	m.actions.md = plainMarkdown
	m = drain(t, m, m.Init())
	m, cmd := press(t, m, "c")
	return drain(t, m, cmd)
}

func TestCommentsComposeAndSend(t *testing.T) {
	fb := detailBackend()
	m := openComments(t, fb)

	m, _ = press(t, m, "x")
	if p := m.popup.Content().(*commentsPopup); p.input.Focused() || p.input.Value() != "" {
		t.Fatal("typing should not reach the textarea before enter")
	}

	m, _ = press(t, m, "enter", "h", "i", "enter", "y", "o")
	p := m.popup.Content().(*commentsPopup)
	if !p.input.Focused() || p.input.Value() != "hi\nyo" {
		t.Fatalf("enter should focus, then insert newlines; value=%q", p.input.Value())
	}

	m, _ = press(t, m, "esc")
	if !m.popup.IsOpen() || m.popup.Content().(*commentsPopup).input.Focused() {
		t.Fatal("esc should leave the textarea but keep the popup open")
	}
	m, _ = press(t, m, "enter")

	m = pressDrain(t, m, "ctrl+s")
	if len(fb.comments) != 1 || fb.comments[0] != "T-1:hi\nyo" {
		t.Fatalf("comments = %q", fb.comments)
	}
	p = m.popup.Content().(*commentsPopup)
	if p.input.Value() != "" || p.input.Focused() || p.sending {
		t.Fatal("textarea should reset and blur after sending")
	}
}

func TestCommentsSendNeedsContent(t *testing.T) {
	fb := detailBackend()
	m := openComments(t, fb)
	m, _ = press(t, m, "enter", " ", "enter")
	pressDrain(t, m, "ctrl+s")
	if len(fb.comments) != 0 {
		t.Fatalf("blank comment was sent: %q", fb.comments)
	}
}

func TestCommentsSendErrorKeepsDraft(t *testing.T) {
	fb := detailBackend()
	fb.commentErr = errors.New("nope")
	m := openComments(t, fb)
	m, _ = press(t, m, "enter", "a")
	m = pressDrain(t, m, "ctrl+s")
	p := m.popup.Content().(*commentsPopup)
	if p.input.Value() != "a" || !p.input.Focused() {
		t.Fatal("draft should survive a failed send")
	}
	if !strings.Contains(m.View().Content, "nope") {
		t.Fatal("send error not shown")
	}
}

func TestCommentsTopAndBottom(t *testing.T) {
	fb := detailBackend()
	for i := range 40 {
		fb.detail.Comments = append(fb.detail.Comments, fb.detail.Comments[i%2])
	}
	m := openComments(t, fb)

	m, _ = press(t, m, "G")
	if p := m.popup.Content().(*commentsPopup); p.issue.ScrollPercent() != 1 {
		t.Fatalf("G should jump to the bottom, scrolled to %v", p.issue.ScrollPercent())
	}
	m, _ = press(t, m, "g")
	if p := m.popup.Content().(*commentsPopup); p.issue.ScrollPercent() != 0 {
		t.Fatalf("g should jump to the top, scrolled to %v", p.issue.ScrollPercent())
	}

	m, _ = press(t, m, "enter", "g", "G")
	p := m.popup.Content().(*commentsPopup)
	if p.input.Value() != "gG" || p.issue.ScrollPercent() != 0 {
		t.Fatalf("g/G should type into the focused input; value=%q scrolled to %v", p.input.Value(), p.issue.ScrollPercent())
	}
}

func TestCommentsFooterSaysWhenOlderCommentsAreNotLoaded(t *testing.T) {
	fb := detailBackend()
	m := openComments(t, fb)
	if v := m.View().Content; strings.Contains(v, "Newest 2 of") {
		t.Fatalf("footer should stay quiet when every comment is loaded:\n%s", v)
	}

	fb.detail.CommentsTotal = 240
	m = openComments(t, fb)
	if v := m.View().Content; !strings.Contains(v, "Newest 2 of 240") {
		t.Fatalf("footer should say older comments are missing:\n%s", v)
	}
}
