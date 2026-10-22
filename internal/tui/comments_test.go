package tui

import (
	"errors"
	"strings"
	"testing"
)

func openComments(t *testing.T, fb *fakeBackend) Model {
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
	if p := m.popup.(*commentsPopup); p.input.Focused() || p.input.Value() != "" {
		t.Fatal("typing should not reach the textarea before enter")
	}

	m, _ = press(t, m, "enter", "h", "i", "enter", "y", "o")
	p := m.popup.(*commentsPopup)
	if !p.input.Focused() || p.input.Value() != "hi\nyo" {
		t.Fatalf("enter should focus, then insert newlines; value=%q", p.input.Value())
	}

	m, _ = press(t, m, "esc")
	if m.popup == nil || m.popup.(*commentsPopup).input.Focused() {
		t.Fatal("esc should leave the textarea but keep the popup open")
	}
	m, _ = press(t, m, "enter")

	m = pressDrain(t, m, "ctrl+s")
	if len(fb.comments) != 1 || fb.comments[0] != "T-1:hi\nyo" {
		t.Fatalf("comments = %q", fb.comments)
	}
	p = m.popup.(*commentsPopup)
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
	p := m.popup.(*commentsPopup)
	if p.input.Value() != "a" || !p.input.Focused() {
		t.Fatal("draft should survive a failed send")
	}
	if !strings.Contains(m.View(), "nope") {
		t.Fatal("send error not shown")
	}
}
