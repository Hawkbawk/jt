package board

import (
	"strings"
	"testing"

	"github.com/hawkbawk/jt/internal/tui/tuitest"
)

func TestCommentsModalSendsAndReloadsIssue(t *testing.T) {
	fb := detailBackend()
	m := newTestModel(t, "TEST", fb)
	m.actions.MD = plainMarkdown
	m = tuitest.Drain(t, m, m.Init())
	m = tuitest.PressDrain(t, m, "c", "enter", "h", "i", "ctrl+s")

	if len(fb.Commented) != 1 || fb.Commented[0] != "T-1:hi" {
		t.Fatalf("comments = %q", fb.Commented)
	}
	if !m.Modal.IsOpen() || !strings.Contains(m.Status, "Commented on T-1") {
		t.Fatalf("modal should stay open and the board should report the comment; status=%q", m.Status)
	}

	m = tuitest.PressDrain(t, m, "q")
	if m.Modal.IsOpen() {
		t.Fatal("closing the comments should close the modal")
	}
}
