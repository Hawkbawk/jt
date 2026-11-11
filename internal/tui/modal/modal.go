// Package modal hosts any tea.Model as a modal drawn over the screen behind it.
//
// A parent embeds a Model and routes messages through its Update. Everything
// that changes the modal happens in that Update, so there are no Open or Close
// methods. Anything under the host asks for a modal by returning a command:
//
//	return m, modal.Open(newConfirm())
//
// Content doesn't have to import this package. The host addresses every
// command its content returns, so the message the command produces comes back
// to that content and nothing else. A response that arrives after the content
// closed, or after another modal replaced it, is dropped. Content reaches past
// itself with messages that have marker methods:
//
//   - A message with a ForParent method goes to the parent instead.
//   - A message with a ClosesModal method closes the modal. If it also has a
//     ForParent method, the parent gets it once the modal is closed.
//
// For example, a picker that reports its choice declares:
//
//	type pickedMsg struct{ choice string }
//
//	func (pickedMsg) ForParent()  {}
//	func (pickedMsg) ClosesModal() {}
//
// Messages from bubbletea itself, such as tea.QuitMsg, aren't addressed, so the
// program still sees them. The parent passes everything to Route first, which
// takes the modal's own messages and the input while it's open.
//
// The host sizes its content by sending it a tea.WindowSizeMsg for the area
// inside the frame, so any model that sizes itself from that message works
// unchanged. Content that implements Titled or Footed has its title and footer
// drawn by the host, which leaves those lines out of the size it sends.
// Content must use virtual cursors, since the host ignores the tea.View cursor
// the content returns.
package modal

import (
	"reflect"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// OpenMsg shows Content, replacing any modal that's already open.
type OpenMsg struct {
	Content tea.Model
}

// CloseMsg closes the modal. If Then is set, it's sent to the parent once the
// modal is closed.
type CloseMsg struct {
	Then tea.Msg
}

// Open returns the command that opens a modal with content.
func Open(content tea.Model) tea.Cmd {
	return func() tea.Msg { return OpenMsg{Content: content} }
}

// Close returns the command that closes the modal.
func Close() tea.Cmd {
	return func() tea.Msg { return CloseMsg{} }
}

// CloseWith returns the command that closes the modal and then sends msg.
func CloseWith(msg tea.Msg) tea.Cmd {
	return func() tea.Msg { return CloseMsg{Then: msg} }
}

// ForParent is a message from the content for the parent, which the host
// passes on instead of returning it to the content.
type ForParent interface {
	ForParent()
}

// Closes is a message from the content that closes the modal.
type Closes interface {
	ClosesModal()
}

// contentMsg is a message produced by a command of the content opened as gen.
type contentMsg struct {
	gen uint64
	msg tea.Msg
}

var (
	cmdType = reflect.TypeFor[tea.Cmd]()
	teaPkg  = reflect.TypeFor[tea.QuitMsg]().PkgPath()
)

// wrap addresses what cmd produces to the content opened as gen.
func wrap(gen uint64, cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg { return address(gen, cmd()) }
}

func address(gen uint64, msg tea.Msg) tea.Msg {
	switch msg := msg.(type) {
	case nil:
		return nil
	case Closes:
		return contentMsg{gen: gen, msg: msg}
	case ForParent:
		return msg
	case tea.BatchMsg:
		out := make(tea.BatchMsg, len(msg))
		for i, c := range msg {
			out[i] = wrap(gen, c)
		}
		return out
	}
	t := reflect.TypeOf(msg)
	if t.PkgPath() != teaPkg {
		return contentMsg{gen: gen, msg: msg}
	}
	// tea.Sequence's message is unexported, so it's recognized by its shape.
	if t.Kind() == reflect.Slice && t.Elem() == cmdType {
		v := reflect.ValueOf(msg)
		cmds := make([]tea.Cmd, v.Len())
		for i := range cmds {
			cmds[i] = wrap(gen, v.Index(i).Interface().(tea.Cmd))
		}
		return tea.Sequence(cmds...)()
	}
	return msg
}

// Titled is content with a title, which the host draws on the first line.
type Titled interface {
	Title() string
}

// Footed is content with a footer, which the host draws on the last line:
// status, then the help for keys in the room left. Either can be empty.
type Footed interface {
	Footer() (status string, keys help.KeyMap)
}

// DefaultSize is the outer size of a modal in an area: most of it, but never
// too small to use.
func DefaultSize(width, height int) (int, int) {
	return max(20, width*4/5), max(6, height*4/5)
}

// Model is the modal host. The zero value isn't usable; build one with New.
type Model struct {
	content tea.Model
	// gen counts the modals opened, so messages for an earlier one are
	// dropped.
	gen uint64
	// areaW and areaH are the area the modal is drawn in, as last reported by
	// a tea.WindowSizeMsg.
	areaW, areaH int

	size    func(width, height int) (int, int)
	style   func() lipgloss.Style
	title   func() lipgloss.Style
	help    func() help.Model
	dismiss key.Binding
}

// Option configures a Model.
type Option func(*Model)

// WithSize sets the outer size of the modal, frame included, for an area. The
// default is DefaultSize.
func WithSize(f func(width, height int) (int, int)) Option {
	return func(m *Model) { m.size = f }
}

// WithStyle sets the style of the frame around the content. It's called each
// time the modal is drawn or sized, so a style can follow a changing theme.
func WithStyle(f func() lipgloss.Style) Option {
	return func(m *Model) { m.style = f }
}

// WithTitleStyle sets the style of a Titled content's title. It's called each
// time the modal is drawn.
func WithTitleStyle(f func() lipgloss.Style) Option {
	return func(m *Model) { m.title = f }
}

// WithHelp sets the help model that draws a Footed content's keys. It's called
// each time the modal is drawn.
func WithHelp(f func() help.Model) Option {
	return func(m *Model) { m.help = f }
}

// WithDismiss closes the modal when a key matches b, before the content sees
// it. Use it for content you can't change. Content that uses the key itself
// should close with Close.
func WithDismiss(b key.Binding) Option {
	return func(m *Model) { m.dismiss = b }
}

// New returns a closed host.
func New(opts ...Option) Model {
	m := Model{
		size:  DefaultSize,
		style: func() lipgloss.Style { return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1) },
		title: lipgloss.NewStyle,
		help:  help.New,
	}
	for _, opt := range opts {
		opt(&m)
	}
	return m
}

// IsOpen reports whether a modal is showing.
func (m Model) IsOpen() bool { return m.content != nil }

// Content returns the model in the modal, or nil when closed.
func (m Model) Content() tea.Model { return m.content }

// Captures reports whether msg is input the open modal takes for itself, so
// the parent shouldn't act on it.
func (m Model) Captures(msg tea.Msg) bool {
	if m.content == nil {
		return false
	}
	switch msg.(type) {
	case tea.KeyMsg, tea.MouseMsg, tea.PasteMsg:
		return true
	}
	return false
}

// Routes reports whether msg belongs to the modal: opening and closing, the
// messages the content's commands produced, and input while it's open. The
// parent passes those to Update and handles everything else itself.
func (m Model) Routes(msg tea.Msg) bool {
	switch msg.(type) {
	case OpenMsg, CloseMsg, contentMsg:
		return true
	}
	return m.Captures(msg)
}

// inner is the size of the area inside the frame.
func (m Model) inner() (int, int) {
	w, h := m.size(m.areaW, m.areaH)
	st := m.style()
	return max(0, w-st.GetHorizontalFrameSize()), max(0, h-st.GetVerticalFrameSize())
}

// body is the size left for the content inside the frame, after the title and
// footer the host draws for it.
func (m Model) body() (int, int) {
	w, h := m.inner()
	if _, ok := m.content.(Titled); ok {
		h--
	}
	if _, ok := m.content.(Footed); ok {
		h--
	}
	return w, max(0, h)
}

// Update handles the messages that open, close and size the modal, delivers
// the content's own messages, and passes the rest to the content, so the
// parent can send it anything it should hear, such as a theme change. While
// closed, it ignores everything else.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case contentMsg:
		if m.content == nil || msg.gen != m.gen {
			return m, nil
		}
		switch inner := msg.msg.(type) {
		case OpenMsg, CloseMsg:
			return m.Update(inner)
		case Closes:
			if _, ok := inner.(ForParent); ok {
				return m.Update(CloseMsg{Then: inner})
			}
			return m.Update(CloseMsg{})
		}
		return m.deliver(msg.msg)
	case tea.WindowSizeMsg:
		m.areaW, m.areaH = msg.Width, msg.Height
		return m.resize()
	case OpenMsg:
		if msg.Content == nil {
			return m, nil
		}
		m.content = msg.Content
		m.gen++
		m, sized := m.resize()
		return m, tea.Batch(sized, wrap(m.gen, m.content.Init()))
	case CloseMsg:
		m.content = nil
		if msg.Then == nil {
			return m, nil
		}
		return m, func() tea.Msg { return msg.Then }
	}
	if m.content == nil {
		return m, nil
	}
	if k, ok := msg.(tea.KeyPressMsg); ok && key.Matches(k, m.dismiss) {
		m.content = nil
		return m, nil
	}
	return m.deliver(msg)
}

// deliver passes msg to the content and addresses the commands it returns.
func (m Model) deliver(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	m.content, cmd = m.content.Update(msg)
	return m, wrap(m.gen, cmd)
}

// resize tells the content the size it has inside the frame. Nothing is sent
// before the host knows its area.
func (m Model) resize() (Model, tea.Cmd) {
	if m.content == nil || m.areaW == 0 {
		return m, nil
	}
	w, h := m.body()
	return m.deliver(tea.WindowSizeMsg{Width: w, Height: h})
}

// View returns the framed content, or "" when closed. The content is clipped or
// padded to the size the host gave it, so the modal never changes size.
func (m Model) View() string {
	if m.content == nil {
		return ""
	}
	w, _ := m.inner()
	_, h := m.body()
	body := ""
	if h > 0 {
		body = fitHeight(m.content.View().Content, h)
	}
	st := m.style()
	return st.Width(w + st.GetHorizontalFrameSize()).Render(m.chrome(body, w, h > 0))
}

// Chrome returns content's view with the title and footer that a host built
// with opts draws around it, width wide and without the frame. It shows
// content as a modal would without opening one.
func Chrome(content tea.Model, width int, opts ...Option) string {
	m := New(opts...)
	m.content = content
	return m.chrome(content.View().Content, width, true)
}

// chrome puts the content's title and footer around body.
func (m Model) chrome(body string, width int, showBody bool) string {
	var lines []string
	if t, ok := m.content.(Titled); ok {
		lines = append(lines, m.title().Render(t.Title()))
	}
	if showBody {
		lines = append(lines, body)
	}
	if f, ok := m.content.(Footed); ok {
		status, keys := f.Footer()
		lines = append(lines, m.footer(status, keys, width))
	}
	return strings.Join(lines, "\n")
}

func (m Model) footer(status string, keys help.KeyMap, width int) string {
	if keys == nil {
		return status
	}
	h := m.help()
	h.SetWidth(max(0, width-lipgloss.Width(status)))
	return status + h.View(keys)
}

// Overlay draws the modal centered on bg, the area's rendered screen. It
// returns bg unchanged when closed.
func (m Model) Overlay(bg string) string {
	if m.content == nil {
		return bg
	}
	return Overlay(bg, m.View(), m.areaW)
}
