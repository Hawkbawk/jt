package tui

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hawkbawk/jt/internal/styles"
)

const jqlEditorHeight = 6

var modeKey = key.NewBinding(key.WithKeys("ctrl+t"), key.WithHelp("ctrl+t", "basic/advanced"))

// jqlEditor is the advanced search mode's box for writing JQL.
type jqlEditor struct {
	input textarea.Model
}

func newJQLEditor() *jqlEditor {
	ta := textarea.New()
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.Placeholder = `project = DEMO AND status != Done ORDER BY updated DESC`
	ta.SetStyles(styles.TextArea())
	ta.SetHeight(jqlEditorHeight)
	// JQL is searched as a whole, so a line break only wraps it for reading.
	ta.KeyMap.InsertNewline = key.NewBinding()
	return &jqlEditor{input: ta}
}

func (e *jqlEditor) value() string { return e.input.Value() }

func (e *jqlEditor) setValue(s string) { e.input.SetValue(s) }

func (e *jqlEditor) focus() tea.Cmd { return e.input.Focus() }

func (e *jqlEditor) blur() { e.input.Blur() }

func (e *jqlEditor) restyle() { e.input.SetStyles(styles.TextArea()) }

func (e *jqlEditor) update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	e.input, cmd = e.input.Update(msg)
	return cmd
}

// view draws the editor in a bordered box that fits width.
func (e *jqlEditor) view(width int) string {
	border := styles.Input().BorderForeground(styles.Highlight())
	e.input.SetWidth(width - border.GetHorizontalFrameSize())
	return lipgloss.JoinVertical(lipgloss.Left,
		styles.Dim().Render("JQL"),
		border.Render(e.input.View()),
	)
}
