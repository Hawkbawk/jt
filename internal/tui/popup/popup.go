// Package popup hosts any tea.Model as a modal drawn over the screen behind it.
//
// A parent embeds a Model and routes messages through its Update. Everything
// that changes the popup happens in that Update, so there are no Open or Close
// methods. Anything under the host asks for a popup by returning a command:
//
//	return m, popup.Open(newConfirm())
//
// Content doesn't need to know it's in a popup. It ends by returning a message
// that implements tuimsg.Final, such as its result or tuimsg.Cancelled, and
// the host closes the popup and hands that message to the parent:
//
//	return c, tuimsg.End(Deleted{Key: c.key})
//
// The host sizes its content by sending it a tea.WindowSizeMsg for the area
// inside the frame, so any model that sizes itself from that message works
// unchanged. Content must use virtual cursors, since the host ignores the
// tea.View cursor the content returns.
//
// Where a message goes:
//
//   - Input goes to the content while a popup is open (see Captures).
//   - Content that embeds tuimsg.Requests passes its requests, such as network
//     loads, through Request. The host wraps them so their responses come back
//     to the parent as an opaque message, which the parent hands to the host
//     without looking at it. The host gives it to the content that made the
//     request, or drops it if that popup has closed or been replaced since, so
//     content never has to check whose response it got.
//   - A tuimsg.Final message closes the popup, then reaches the parent, except
//     tuimsg.Cancelled, which only closes it.
//   - Any other message the content returns reaches the parent first. The
//     parent handles the ones meant for it and hands the rest to the host,
//     which passes them to the content.
package popup

import (
	"reflect"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hawkbawk/jt/internal/tui/layout"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
)

// OpenMsg shows Content, replacing any popup that's already open.
type OpenMsg struct {
	Content tea.Model
}

// CloseMsg closes the popup. If Then is set, it's sent to the parent once the
// popup is closed.
type CloseMsg struct {
	Then tea.Msg
}

// Open returns the command that opens a popup with content.
func Open(content tea.Model) tea.Cmd {
	return func() tea.Msg { return OpenMsg{Content: content} }
}

// Close returns the command that closes the popup.
func Close() tea.Cmd {
	return func() tea.Msg { return CloseMsg{} }
}

// requester is content whose requests the host can wrap. Embedding
// tuimsg.Requests implements it.
type requester interface {
	WrapRequests(func(tea.Cmd) tea.Cmd)
}

// replyMsg is the response to a request the content made. gen is the popup
// that made it, which the host sets as the request leaves the content.
type replyMsg struct {
	gen uint64
	msg tea.Msg
}

func reply(cmd tea.Cmd) tea.Cmd {
	return func() tea.Msg { return replyMsg{msg: cmd()} }
}

// DefaultSize is the outer size of a popup in an area: most of it, but never
// too small to use.
func DefaultSize(width, height int) (int, int) {
	return max(20, width*4/5), max(6, height*4/5)
}

// Model is the popup host. The zero value isn't usable; build one with New.
type Model struct {
	content tea.Model
	// gen counts the popups opened, so a reply can be matched to the popup
	// that asked for it.
	gen uint64
	// areaW and areaH are the area the popup is drawn in, as last reported by
	// a tea.WindowSizeMsg.
	areaW, areaH int

	size    func(width, height int) (int, int)
	style   func() lipgloss.Style
	dismiss key.Binding
}

// Option configures a Model.
type Option func(*Model)

// WithSize sets the outer size of the popup, frame included, for an area. The
// default is DefaultSize.
func WithSize(f func(width, height int) (int, int)) Option {
	return func(m *Model) { m.size = f }
}

// WithStyle sets the style of the frame around the content. It's called each
// time the popup is drawn or sized, so a style can follow a changing theme.
func WithStyle(f func() lipgloss.Style) Option {
	return func(m *Model) { m.style = f }
}

// WithDismiss closes the popup when a key matches b, before the content sees
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
	}
	for _, opt := range opts {
		opt(&m)
	}
	return m
}

// IsOpen reports whether a popup is showing.
func (m Model) IsOpen() bool { return m.content != nil }

// Content returns the model in the popup, or nil when closed.
func (m Model) Content() tea.Model { return m.content }

// Captures reports whether msg is input the open popup takes for itself, so
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

// inner is the size of the area inside the frame.
func (m Model) inner() (int, int) {
	w, h := m.size(m.areaW, m.areaH)
	st := m.style()
	return max(0, w-st.GetHorizontalFrameSize()), max(0, h-st.GetVerticalFrameSize())
}

// Update handles the messages that open, close and size the popup, and passes
// the rest to the content. While closed, it ignores everything else.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.areaW, m.areaH = msg.Width, msg.Height
		return m.resize()
	case OpenMsg:
		if msg.Content == nil {
			return m, nil
		}
		m.content = msg.Content
		m.gen++
		if r, ok := m.content.(requester); ok {
			r.WrapRequests(reply)
		}
		m, sized := m.resize()
		return m, tea.Batch(sized, m.stamp(m.content.Init()))
	case replyMsg:
		if m.content == nil || msg.gen != m.gen {
			return m, nil
		}
		var cmd tea.Cmd
		m.content, cmd = m.content.Update(msg.msg)
		return m, m.stamp(cmd)
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
	var cmd tea.Cmd
	m.content, cmd = m.content.Update(msg)
	return m, m.stamp(cmd)
}

// stamp marks the replies cmd produces as the current popup's, and turns the
// content's final message into closing the popup.
func (m Model) stamp(cmd tea.Cmd) tea.Cmd {
	return stamp(cmd, m.gen)
}

var cmdType = reflect.TypeFor[tea.Cmd]()

func stamp(cmd tea.Cmd, gen uint64) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		msg := cmd()
		switch msg := msg.(type) {
		case replyMsg:
			msg.gen = gen
			return msg
		case tuimsg.Cancelled:
			return CloseMsg{}
		case tuimsg.Final:
			return CloseMsg{Then: msg}
		case tea.BatchMsg:
			out := make(tea.BatchMsg, len(msg))
			for i, c := range msg {
				out[i] = stamp(c, gen)
			}
			return out
		}
		// tea.Sequence's message is unexported, but it's a []tea.Cmd too, so
		// it's rebuilt as the same type with each command stamped.
		v := reflect.ValueOf(msg)
		if v.Kind() != reflect.Slice || v.Type().Elem() != cmdType {
			return msg
		}
		out := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := range v.Len() {
			c, _ := reflect.TypeAssert[tea.Cmd](v.Index(i))
			out.Index(i).Set(reflect.ValueOf(stamp(c, gen)))
		}
		return out.Interface()
	}
}

// resize tells the content the size of the area inside the frame. Nothing is
// sent before the host knows its area.
func (m Model) resize() (Model, tea.Cmd) {
	if m.content == nil || m.areaW == 0 {
		return m, nil
	}
	w, h := m.inner()
	var cmd tea.Cmd
	m.content, cmd = m.content.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m, m.stamp(cmd)
}

// View returns the framed content, or "" when closed. The content is clipped or
// padded to the size the host gave it, so the popup never changes size.
func (m Model) View() string {
	if m.content == nil {
		return ""
	}
	w, h := m.inner()
	st := m.style()
	body := layout.FitHeight(m.content.View().Content, h)
	return st.Width(w + st.GetHorizontalFrameSize()).Render(body)
}

// Overlay draws the popup centered on bg, the area's rendered screen. It
// returns bg unchanged when closed.
func (m Model) Overlay(bg string) string {
	if m.content == nil {
		return bg
	}
	return Overlay(bg, m.View(), m.areaW)
}
