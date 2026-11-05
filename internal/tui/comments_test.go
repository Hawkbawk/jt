package tui

import (
	"strings"
	"testing"
)

func TestCommentsPopupSendsAndReloadsIssue(t *testing.T) {
	fb := detailBackend()
	m := newTestModel(t, "TEST", fb)
	m.actions.md = plainMarkdown
	m = drain(t, m, m.Init())
	m = pressDrain(t, m, "c", "enter", "h", "i", "ctrl+s")

	if len(fb.comments) != 1 || fb.comments[0] != "T-1:hi" {
		t.Fatalf("comments = %q", fb.comments)
	}
	if !m.popup.IsOpen() || !strings.Contains(m.status, "Commented on T-1") {
		t.Fatalf("popup should stay open and the board should report the comment; status=%q", m.status)
	}

	m = pressDrain(t, m, "q")
	if m.popup.IsOpen() {
		t.Fatal("closing the comments should close the popup")
	}
}
