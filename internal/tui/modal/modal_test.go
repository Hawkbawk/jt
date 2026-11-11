package modal

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type initMsg struct{}

type doneMsg struct{}

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
			return s, CloseWith(doneMsg{})
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
		t.Fatal("OpenMsg did not open the modal")
	}
	var inits int
	for _, msg := range flatten(cmd) {
		if c, ok := msg.(contentMsg); ok {
			if _, ok := c.msg.(initMsg); ok {
				inits++
			}
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
		t.Fatal("modal closed before the CloseMsg arrived")
	}
	if c, ok := msg.(contentMsg); !ok || c.msg != (CloseMsg{Then: doneMsg{}}) {
		t.Fatalf("got %#v, want the content's CloseMsg", msg)
	}
	m, msg = send(t, m, msg)
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

// cmdStub returns cmd from its next Update.
type cmdStub struct {
	stub
	cmd tea.Cmd
}

func (s *cmdStub) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	s.got = append(s.got, msg)
	cmd := s.cmd
	s.cmd = nil
	return s, cmd
}

// returning opens a cmdStub and returns the host's command for the cmd the
// stub returns from its next Update.
func returning(t *testing.T, cmd tea.Cmd) (Model, *cmdStub, tea.Cmd) {
	t.Helper()
	m, _ := plain().Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	s := &cmdStub{}
	m, _ = send(t, m, OpenMsg{Content: s})
	s.got, s.cmd = nil, cmd
	m, out := m.Update(struct{}{})
	s.got = nil
	return m, s, out
}

func TestResponseComesBackToContent(t *testing.T) {
	m, s, cmd := returning(t, func() tea.Msg { return doneMsg{} })
	msg := cmd()
	if _, _, ok := route(m, msg); !ok {
		t.Fatalf("Route didn't take the content's message %#v", msg)
	}
	if len(s.got) != 1 || s.got[0] != (doneMsg{}) {
		t.Fatalf("content got %v", s.got)
	}
	if _, _, ok := route(m, doneMsg{}); ok {
		t.Fatal("Route took a message that isn't the modal's")
	}
}

func TestResponseForEarlierModalIsDropped(t *testing.T) {
	m, _, cmd := returning(t, func() tea.Msg { return doneMsg{} })
	second := &stub{}
	m, _ = send(t, m, OpenMsg{Content: second})
	m, out, ok := route(m, cmd())
	if !ok || out != nil || len(second.got) != 0 || !m.IsOpen() {
		t.Fatalf("handled=%v, second got %v", ok, second.got)
	}
}

func TestBatchAndSequenceAreAddressed(t *testing.T) {
	for name, combine := range map[string]func(...tea.Cmd) tea.Cmd{"batch": tea.Batch, "sequence": tea.Sequence} {
		done := func() tea.Msg { return doneMsg{} }
		init := func() tea.Msg { return initMsg{} }
		_, _, cmd := returning(t, combine(done, init))
		msg := cmd()
		cmds := reflect.ValueOf(msg)
		if cmds.Kind() != reflect.Slice || cmds.Len() != 2 {
			t.Fatalf("%s: got %#v", name, msg)
		}
		for i := range cmds.Len() {
			if _, ok := cmds.Index(i).Interface().(tea.Cmd)().(contentMsg); !ok {
				t.Errorf("%s: cmd %d isn't addressed", name, i)
			}
		}
	}
}

func TestBubbleteaMessagesPassThrough(t *testing.T) {
	_, _, cmd := returning(t, tea.Quit)
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("tea.Quit was addressed to the content")
	}
}

type parentMsg struct{}

func (parentMsg) ForParent() {}

type closeMsg struct{}

func (closeMsg) ClosesModal() {}

type resultMsg struct{}

func (resultMsg) ForParent()   {}
func (resultMsg) ClosesModal() {}

func TestForParentReachesParent(t *testing.T) {
	m, s, cmd := returning(t, func() tea.Msg { return parentMsg{} })
	msg := cmd()
	if _, _, ok := route(m, msg); msg != (parentMsg{}) || ok || len(s.got) != 0 {
		t.Fatalf("got %#v, routed=%v, content got %v", msg, ok, s.got)
	}
}

func TestClosesClosesTheModal(t *testing.T) {
	for name, tc := range map[string]struct {
		msg  tea.Msg
		then tea.Msg
	}{
		"plain":  {closeMsg{}, nil},
		"result": {resultMsg{}, resultMsg{}},
	} {
		m, s, cmd := returning(t, func() tea.Msg { return tc.msg })
		m, out, ok := route(m, cmd())
		if !ok || m.IsOpen() || len(s.got) != 0 {
			t.Fatalf("%s: routed=%v open=%v content got %v", name, ok, m.IsOpen(), s.got)
		}
		var then tea.Msg
		if out != nil {
			then = out()
		}
		if then != tc.then {
			t.Fatalf("%s: parent got %#v, want %#v", name, then, tc.then)
		}
	}
}

func TestCloseFromEarlierModalIsDropped(t *testing.T) {
	m, _, cmd := returning(t, func() tea.Msg { return closeMsg{} })
	m, _ = send(t, m, OpenMsg{Content: &stub{}})
	if m, _, _ = route(m, cmd()); !m.IsOpen() {
		t.Fatal("an earlier modal's close closed the new one")
	}
}

// chromed has a title and a footer, and a body of lines.
type chromed struct {
	stub
	lines int
}

func (c *chromed) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	c.stub.Update(msg)
	return c, nil
}

func (c *chromed) Title() string { return "title" }

func (c *chromed) Footer() (string, help.KeyMap) { return "status", nil }

func (c *chromed) View() tea.View {
	body := make([]string, c.lines)
	for i := range body {
		body[i] = fmt.Sprint(i)
	}
	return tea.NewView(strings.Join(body, "\n"))
}

func TestHostDrawsTitleAndFooter(t *testing.T) {
	m, _ := plain().Update(tea.WindowSizeMsg{Width: 20, Height: 10})
	for _, n := range []int{1, 20} {
		c := &chromed{lines: n}
		m, _ := send(t, m, OpenMsg{Content: c})
		// 8 of the 10 rows, less the border, then less the title and footer.
		if c.size.Height != 4 {
			t.Fatalf("content sized %d rows, want 4", c.size.Height)
		}
		lines := strings.Split(ansi.Strip(m.View()), "\n")
		inner := lines[1 : len(lines)-1]
		if len(inner) != 6 || !strings.Contains(inner[0], "title") || !strings.Contains(inner[5], "status") {
			t.Fatalf("%d body lines: title and footer not pinned:\n%s", n, strings.Join(lines, "\n"))
		}
	}
}

// route sends msg through the host only if it belongs to the modal.
func route(m Model, msg tea.Msg) (Model, tea.Cmd, bool) {
	if !m.Routes(msg) {
		return m, nil, false
	}
	m, cmd := m.Update(msg)
	return m, cmd, true
}
