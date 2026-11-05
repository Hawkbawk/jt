package issuecomments

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/hawkbawk/jt/internal/jira"
)

type fakeBackend struct {
	list   *jira.CommentList
	posted []string
	addErr error
}

func (f *fakeBackend) Issue(string) (*jira.IssueDetail, error) { return &jira.IssueDetail{}, nil }

func (f *fakeBackend) Comments(string) (*jira.CommentList, error) { return f.list, nil }

func (f *fakeBackend) AddComment(key, body string) error {
	if f.addErr != nil {
		return f.addErr
	}
	f.posted = append(f.posted, key+":"+body)
	return nil
}

func newBackend() *fakeBackend {
	return &fakeBackend{list: &jira.CommentList{Total: 2, Items: []jira.Comment{
		{Author: "Ada", Created: "2026-10-01 09:00", Body: "first comment"},
		{Author: "Grace", Created: "2026-10-02 10:00", Body: "second comment"},
	}}}
}

func plain(md string, _ int) string { return md }

func keyMsg(k string) tea.KeyPressMsg {
	switch k {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "ctrl+s":
		return tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	}
	return tea.KeyPressMsg{Code: rune(k[0]), Text: k}
}

// run feeds cmd's messages back to m, skipping timers, and returns the
// messages the host would see.
func run(m *Model, cmd tea.Cmd) []tea.Msg {
	var out []tea.Msg
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
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
		case Closed, Sent:
			out = append(out, msg)
		default:
			_, next := m.Update(msg)
			queue = append(queue, next)
		}
	}
	return out
}

func press(m *Model, keys ...string) []tea.Msg {
	var out []tea.Msg
	for _, k := range keys {
		_, cmd := m.Update(keyMsg(k))
		out = append(out, run(m, cmd)...)
	}
	return out
}

func open(t *testing.T, b *fakeBackend) *Model {
	t.Helper()
	m := New(b, "T-1", plain)
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 30})
	run(m, m.Init())
	return m
}

func view(m *Model) string { return ansi.Strip(m.View().Content) }

func TestComposeAndSend(t *testing.T) {
	b := newBackend()
	m := open(t, b)

	press(m, "x")
	if m.input.Focused() || m.input.Value() != "" {
		t.Fatal("typing should not reach the textarea before enter")
	}

	press(m, "enter", "h", "i", "enter", "y", "o")
	if !m.input.Focused() || m.input.Value() != "hi\nyo" {
		t.Fatalf("enter should focus, then insert newlines; value=%q", m.input.Value())
	}

	if out := press(m, "esc"); len(out) != 0 || m.input.Focused() {
		t.Fatal("esc should leave the textarea without closing")
	}
	press(m, "enter")

	out := press(m, "ctrl+s")
	if len(b.posted) != 1 || b.posted[0] != "T-1:hi\nyo" {
		t.Fatalf("posted = %q", b.posted)
	}
	if len(out) != 1 || out[0] != (Sent{Key: "T-1"}) {
		t.Fatalf("sending should report Sent, got %v", out)
	}
	if m.input.Value() != "" || m.input.Focused() || m.sending {
		t.Fatal("textarea should reset and blur after sending")
	}
}

func TestSendNeedsContent(t *testing.T) {
	b := newBackend()
	m := open(t, b)
	press(m, "enter", " ", "ctrl+s")
	if len(b.posted) != 0 {
		t.Fatalf("blank comment was sent: %q", b.posted)
	}
}

func TestSendErrorKeepsDraft(t *testing.T) {
	b := newBackend()
	b.addErr = errors.New("nope")
	m := open(t, b)
	out := press(m, "enter", "a", "ctrl+s")
	if len(out) != 0 || m.input.Value() != "a" || !m.input.Focused() {
		t.Fatal("draft should survive a failed send without reporting Sent")
	}
	if !strings.Contains(view(m), "nope") {
		t.Fatal("send error not shown")
	}
}

func TestEscClosesOnlyWhenNotWriting(t *testing.T) {
	m := open(t, newBackend())
	if out := press(m, "q"); len(out) != 1 || out[0] != (Closed{}) {
		t.Fatalf("q should ask to close, got %v", out)
	}
}

func TestTopAndBottom(t *testing.T) {
	b := newBackend()
	for i := range 40 {
		b.list.Items = append(b.list.Items, b.list.Items[i%2])
	}
	m := open(t, b)

	press(m, "G")
	if m.issue.ScrollPercent() != 1 {
		t.Fatalf("G should jump to the bottom, scrolled to %v", m.issue.ScrollPercent())
	}
	press(m, "g")
	if m.issue.ScrollPercent() != 0 {
		t.Fatalf("g should jump to the top, scrolled to %v", m.issue.ScrollPercent())
	}

	press(m, "enter", "g", "G")
	if m.input.Value() != "gG" || m.issue.ScrollPercent() != 0 {
		t.Fatalf("g/G should type into the focused input; value=%q scrolled to %v", m.input.Value(), m.issue.ScrollPercent())
	}
}

func TestFooterSaysWhenOlderCommentsAreNotLoaded(t *testing.T) {
	b := newBackend()
	if v := view(open(t, b)); strings.Contains(v, "Newest 2 of") {
		t.Fatalf("footer should stay quiet when every comment is loaded:\n%s", v)
	}

	b.list.Total = 240
	if v := view(open(t, b)); !strings.Contains(v, "Newest 2 of 240") {
		t.Fatalf("footer should say older comments are missing:\n%s", v)
	}
}
