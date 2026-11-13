// Package fieldform lists an issue's fields and edits one at a time in a form.
// The issue edit and create modals share it. It keeps the values entered. The
// modals around it load the fields and send what changed.
package fieldform

import (
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/hawkbawk/jt/internal/components/pane"
	"github.com/hawkbawk/jt/internal/components/usersearch"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/styles"
)

// minTextLines is the height of a text area that isn't being edited.
const minTextLines = 5

// refreshMsg makes the form redraw its content, which it only does on updates.
type refreshMsg struct{}

// Editor lists fields and edits one at a time in a form. The zero value is an
// editor with no fields.
type Editor struct {
	keys    KeyMap
	vals    []*fieldValue
	list    list.Model
	editing *fieldValue
	form    *huh.Form
	// formH is the height the form was last drawn at.
	formH int
}

// Help sets what the save and dismiss keys are called and which keys besides
// the usual ones the help lists while browsing fields.
func (e *Editor) Help(saveLabel, dismissLabel string, extra ...key.Binding) {
	e.keys = NewKeyMap()
	e.keys.SaveLabel, e.keys.DismissLabel, e.keys.Extra = saveLabel, dismissLabel, extra
}

// Load replaces the fields with fields. search finds the people for user
// fields.
func (e *Editor) Load(fields []jira.IssueField, search usersearch.Func) {
	if e.keys.SaveLabel == "" {
		e.keys = NewKeyMap()
	}
	e.vals, e.editing, e.form = nil, nil, nil
	for _, f := range fields {
		e.vals = append(e.vals, newFieldValue(f, search))
	}
	e.list = newFieldList(e.vals)
}

// Len is how many fields there are.
func (e *Editor) Len() int { return len(e.vals) }

// Holding is true while a field's form is open, which blocks saving.
func (e *Editor) Holding() bool { return e.form != nil }

// Capturing is true while typed text goes to a form field or the list filter.
func (e *Editor) Capturing() bool {
	return e.editing != nil || e.list.SettingFilter()
}

// ConsumesEsc is true when esc steps back within the editor instead of closing
// the modal.
func (e *Editor) ConsumesEsc() bool {
	return e.editing != nil || e.list.FilterState() != list.Unfiltered
}

// Changed returns the fields that differ from how they loaded. What is still
// being entered in an open form doesn't count until enter keeps it.
func (e *Editor) Changed() []jira.IssueField {
	var out []jira.IssueField
	for _, v := range e.vals {
		f := v.current()
		if v == e.editing {
			f = v.with(v.before)
		}
		if v.differs(f) {
			out = append(out, f)
		}
	}
	return out
}

// Missing returns the names of the required fields that hold no value.
func (e *Editor) Missing() []string {
	var out []string
	for _, v := range e.vals {
		f := v.current()
		if v == e.editing {
			f = v.with(v.before)
		}
		if v.Required && empty(f) {
			out = append(out, v.Name)
		}
	}
	return out
}

// Update handles a message for the list or the open form.
func (e *Editor) Update(msg tea.Msg) tea.Cmd {
	km, isKey := msg.(tea.KeyMsg)
	if e.form == nil {
		if isKey && key.Matches(km, e.keys.Accept) && !e.list.SettingFilter() {
			if v, ok := e.list.SelectedItem().(*fieldValue); ok {
				return e.edit(v)
			}
			return nil
		}
		if isKey && key.Matches(km, e.keys.Undo) && !e.list.SettingFilter() {
			if v, ok := e.list.SelectedItem().(*fieldValue); ok {
				v.reset()
			}
			return nil
		}
		var cmd tea.Cmd
		e.list, cmd = e.list.Update(msg)
		return cmd
	}
	if isKey && key.Matches(km, e.keys.Blur) {
		e.editing.revert()
		e.closeForm()
		return nil
	}
	_, cmd := e.form.Update(msg)
	e.growText()
	if e.form.State == huh.StateCompleted {
		e.editing.normalizeTime(time.Now())
		e.closeForm()
	}
	return cmd
}

// View draws the open form, or else the list, in a space height lines high.
func (e *Editor) View(width, height int) string {
	if e.form != nil {
		resized := e.formH != height
		e.formH = height
		e.form.WithWidth(width).WithHeight(height)
		if resized {
			e.growText()
		}
		return e.form.View()
	}
	e.list.SetSize(width, height)
	return e.list.View()
}

// Hint is the help for what the editor shows now.
func (e *Editor) Hint(h help.Model) string {
	mode := HelpList
	switch {
	case e.form != nil && e.editing.isUser():
		mode = HelpUserField
	case e.form != nil:
		mode = HelpField
	case e.list.SettingFilter():
		mode = HelpFilter
	}
	km := e.keys.WithMode(mode)
	if mode == HelpField {
		km = km.withForm(e.form.KeyBinds())
	}
	return h.View(km)
}

// edit opens a form for v alone.
func (e *Editor) edit(v *fieldValue) tea.Cmd {
	e.editing = v
	v.begin()
	if v.isUser() {
		v.input = v.people.Field()
	} else {
		v.input = v.huhField()
	}
	e.form = pane.NewForm(v.input)
	e.growText()
	return e.form.Init()
}

// closeForm goes back to the list of fields.
func (e *Editor) closeForm() {
	e.editing, e.form = nil, nil
}

// growText gives a text area being edited most of the form's height.
func (e *Editor) growText() {
	if t, ok := e.editing.input.(*huh.Text); ok {
		t.Lines(max(minTextLines, e.formH-3))
		e.form.Update(refreshMsg{})
	}
}

// fieldDelegate is the themed delegate with no gap between rows.
func fieldDelegate() list.DefaultDelegate {
	d := styles.ListDelegate()
	d.SetSpacing(0)
	return d
}

func newFieldList(vals []*fieldValue) list.Model {
	items := make([]list.Item, len(vals))
	for i, v := range vals {
		items[i] = v
	}
	l := list.New(items, fieldDelegate(), 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()
	return l
}
