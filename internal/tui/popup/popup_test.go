package popup

import (
	"reflect"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
)

type initMsg struct{}

type doneMsg struct{}

func (doneMsg) Final() {}

// stub records what it receives and closes on "x".
type stub struct {
	size tea.WindowSizeMsg
	got  []tea.Msg
}

func (s *stub) Init() tea.Cmd { return func() tea.Msg { return initMsg{} } }

func (s *stub) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.size = msg
		return s, nil
	case tea.KeyPressMsg:
		if msg.Text == "x" {
			return s, tuimsg.End(doneMsg{})
		}
	}
	s.got = append(s.got, msg)
	return s, nil
}

func (s *stub) View() tea.View { return tea.NewView("hello") }

func plain() Model {
	return New(WithStyle(func() lipgloss.Style { return lipgloss.NewStyle().Border(lipgloss.NormalBorder()) }))
}

func send(t *testing.T, m Model, msg tea.Msg) (Model, tea.Msg) {
	t.Helper()
	m, cmd := m.Update(msg)
	if cmd == nil {
		return m, nil
	}
	return m, cmd()
}

// flatten runs cmd and returns the messages it produces, unpacking batches.
func flatten(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}
	var out []tea.Msg
	for _, c := range batch {
		out = append(out, flatten(c)...)
	}
	return out
}

func opened(t *testing.T) (Model, *stub) {
	t.Helper()
	m, _ := plain().Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	s := &stub{}
	m, _ = send(t, m, OpenMsg{Content: s})
	return m, s
}

func TestOpenShowsContentAndRunsInit(t *testing.T) {
	m, _ := plain().Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	if m.IsOpen() {
		t.Fatal("open before OpenMsg")
	}
	m, cmd := m.Update(OpenMsg{Content: &stub{}})
	if !m.IsOpen() {
		t.Fatal("OpenMsg did not open the popup")
	}
	var inits int
	for _, msg := range flatten(cmd) {
		if _, ok := msg.(initMsg); ok {
			inits++
		}
	}
	if inits != 1 {
		t.Fatalf("Init ran %d times, want 1", inits)
	}
}

func TestOpenSizesContentInsideFrame(t *testing.T) {
	_, s := opened(t)
	// 80% of 100x50, less the one-cell border on each side.
	if s.size.Width != 78 || s.size.Height != 38 {
		t.Fatalf("content sized %dx%d, want 78x38", s.size.Width, s.size.Height)
	}
}

func TestResizeReachesOpenContent(t *testing.T) {
	m, s := opened(t)
	send(t, m, tea.WindowSizeMsg{Width: 50, Height: 20})
	if s.size.Width != 38 || s.size.Height != 14 {
		t.Fatalf("content sized %dx%d, want 38x14", s.size.Width, s.size.Height)
	}
}

func TestNothingSentBeforeAreaIsKnown(t *testing.T) {
	s := &stub{}
	m, _ := send(t, plain(), OpenMsg{Content: s})
	if !m.IsOpen() || s.size != (tea.WindowSizeMsg{}) {
		t.Fatalf("open=%v size=%v", m.IsOpen(), s.size)
	}
	send(t, m, tea.WindowSizeMsg{Width: 100, Height: 50})
	if s.size.Width == 0 {
		t.Fatal("content never sized once the area was known")
	}
}

func TestMessagesGoToContentOnlyWhileOpen(t *testing.T) {
	m, s := opened(t)
	type custom struct{}
	m, _ = send(t, m, custom{})
	if len(s.got) != 1 {
		t.Fatalf("content got %v", s.got)
	}
	m, _ = send(t, m, CloseMsg{})
	if _, cmd := m.Update(custom{}); cmd != nil || len(s.got) != 1 {
		t.Fatal("closed host passed a message on")
	}
}

func TestContentClosesWithResult(t *testing.T) {
	m, _ := opened(t)
	m, msg := send(t, m, tea.KeyPressMsg{Code: 'x', Text: "x"})
	if !m.IsOpen() {
		t.Fatal("popup closed before the CloseMsg arrived")
	}
	closeMsg, ok := msg.(CloseMsg)
	if !ok {
		t.Fatalf("got %T, want CloseMsg", msg)
	}
	m, msg = send(t, m, closeMsg)
	if m.IsOpen() {
		t.Fatal("CloseMsg did not close")
	}
	if _, ok := msg.(doneMsg); !ok {
		t.Fatalf("after closing got %T, want the result", msg)
	}
}

func TestPlainCloseSendsNothing(t *testing.T) {
	m, _ := opened(t)
	m, cmd := m.Update(CloseMsg{})
	if m.IsOpen() || cmd != nil {
		t.Fatalf("open=%v cmd=%v", m.IsOpen(), cmd != nil)
	}
}

func TestOpenReplacesContent(t *testing.T) {
	m, first := opened(t)
	second := &stub{}
	m, _ = send(t, m, OpenMsg{Content: second})
	if m.Content() != second {
		t.Fatal("OpenMsg did not replace the content")
	}
	send(t, m, struct{}{})
	if len(first.got) != 0 || len(second.got) != 1 {
		t.Fatalf("first got %v, second got %v", first.got, second.got)
	}
}

func TestDismissClosesBeforeContentSeesKey(t *testing.T) {
	esc := key.NewBinding(key.WithKeys("esc"))
	m, _ := New(WithDismiss(esc)).Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	s := &stub{}
	m, _ = send(t, m, OpenMsg{Content: s})
	m, _ = send(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.IsOpen() || len(s.got) != 0 {
		t.Fatalf("open=%v, content got %v", m.IsOpen(), s.got)
	}
}

func TestCaptures(t *testing.T) {
	m, _ := opened(t)
	closed := plain()
	for _, tc := range []struct {
		name string
		msg  tea.Msg
		host Model
		want bool
	}{
		{"key while open", tea.KeyPressMsg{Code: 'a', Text: "a"}, m, true},
		{"paste while open", tea.PasteMsg{Content: "a"}, m, true},
		{"click while open", tea.MouseClickMsg{}, m, true},
		{"other while open", struct{}{}, m, false},
		{"key while closed", tea.KeyPressMsg{Code: 'a', Text: "a"}, closed, false},
	} {
		if got := tc.host.Captures(tc.msg); got != tc.want {
			t.Errorf("%s: Captures = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestViewIsFramedAndKeepsItsSize(t *testing.T) {
	m, _ := opened(t)
	v := m.View()
	if w, h := lipgloss.Size(v); w != 80 || h != 40 {
		t.Fatalf("view is %dx%d, want 80x40", w, h)
	}
	if !strings.Contains(ansi.Strip(v), "hello") {
		t.Fatalf("view missing content:\n%s", v)
	}
	if got := plain().View(); got != "" {
		t.Fatalf("closed view = %q", got)
	}
}

func TestOverlayDrawsOverBackgroundOnlyWhenOpen(t *testing.T) {
	m, _ := opened(t)
	bg := strings.TrimSuffix(strings.Repeat(strings.Repeat(".", 100)+"\n", 50), "\n")
	got := ansi.Strip(m.Overlay(bg))
	if !strings.Contains(got, "hello") || !strings.Contains(got, "..........") {
		t.Fatalf("overlay lost the content or the background:\n%s", got)
	}
	if plain().Overlay(bg) != bg {
		t.Fatal("closed host changed the background")
	}
}

type loadedMsg struct{ n int }

// asker asks for a reply on Init, and again in a sequence on "r".
type asker struct {
	tuimsg.Requests
	n   int
	got []tea.Msg
}

func (r *asker) Init() tea.Cmd {
	return r.Request(func() tea.Msg { return loadedMsg{r.n} })
}

func (r *asker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && k.Text == "r" {
		return r, tea.Sequence(
			func() tea.Msg { return nil },
			r.Request(func() tea.Msg { return loadedMsg{-r.n} }),
		)
	}
	if l, ok := msg.(loadedMsg); ok {
		r.got = append(r.got, l)
	}
	return r, nil
}

func (r *asker) View() tea.View { return tea.NewView("") }

func TestRepliesReachOnlyThePopupThatAsked(t *testing.T) {
	m, _ := plain().Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	first, second := &asker{n: 1}, &asker{n: 2}
	m, cmd := m.Update(OpenMsg{Content: first})
	stale := flatten(cmd)
	m, cmd = m.Update(OpenMsg{Content: second})
	fresh := flatten(cmd)
	for _, msg := range append(stale, fresh...) {
		m, _ = m.Update(msg)
	}
	if len(first.got) != 0 {
		t.Fatalf("replaced popup got %v", first.got)
	}
	if len(second.got) != 1 || second.got[0] != (loadedMsg{2}) {
		t.Fatalf("open popup got %v", second.got)
	}
}

func TestRepliesInsideSequencesAreStamped(t *testing.T) {
	m, _ := plain().Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	r := &asker{n: 3}
	m, _ = m.Update(OpenMsg{Content: r})
	m, cmd := m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	seq := reflect.ValueOf(cmd())
	if seq.Kind() != reflect.Slice || seq.Len() != 2 {
		t.Fatalf("sequence lost its type: %T", cmd())
	}
	m.Update(seq.Index(1).Interface().(tea.Cmd)())
	if len(r.got) != 1 || r.got[0] != (loadedMsg{-3}) {
		t.Fatalf("got %v", r.got)
	}
}

func TestRepliesAfterCloseAreDropped(t *testing.T) {
	m, _ := plain().Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	r := &asker{n: 4}
	m, cmd := m.Update(OpenMsg{Content: r})
	pending := flatten(cmd)
	m, _ = m.Update(CloseMsg{})
	for _, msg := range pending {
		m, _ = m.Update(msg)
	}
	if len(r.got) != 0 || m.IsOpen() {
		t.Fatalf("closed popup got %v", r.got)
	}
}

func TestCancelClosesWithoutResult(t *testing.T) {
	m, _ := plain().Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	m, _ = m.Update(OpenMsg{Content: &ender{end: tuimsg.Cancel()}})
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	if msg, ok := cmd().(CloseMsg); !ok || msg.Then != nil {
		t.Fatalf("got %#v", cmd())
	}
}

// ender ends with end on any key.
type ender struct{ end tea.Cmd }

func (e *ender) Init() tea.Cmd { return nil }

func (e *ender) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tea.KeyPressMsg); ok {
		return e, e.end
	}
	return e, nil
}

func (e *ender) View() tea.View { return tea.NewView("") }
