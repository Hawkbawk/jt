package tui

import (
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/hawkbawk/jt/internal/jira"
)

type fieldsLoadedMsg struct {
	key    string
	fields []jira.EditField
	err    error
}

type fieldsSavedMsg struct {
	key   string
	count int
	err   error
}

func loadFields(b Backend, iss jira.Issue) tea.Cmd {
	return func() tea.Msg {
		fs, err := b.EditFields(iss)
		return fieldsLoadedMsg{key: iss.Key, fields: fs, err: err}
	}
}

// fieldInput edits one field. It updates its value with every key, so the
// value is always current.
type fieldInput interface {
	// update handles a key while the field is being edited, and reports
	// whether editing is done.
	update(tea.KeyMsg) (tea.Cmd, bool)
	view(innerW, height int) []string
	hint() string
	value() jira.EditField
}

func newFieldInput(f jira.EditField, b Backend, issueKey string) fieldInput {
	f.Chosen = slices.Clone(f.Chosen)
	f.Options = slices.Clone(f.Options)
	switch f.Kind {
	case jira.FieldTextArea:
		return newAreaField(f)
	case jira.FieldSelect, jira.FieldMultiSelect:
		return newSelectField(f)
	case jira.FieldUser, jira.FieldMultiUser:
		return newUserField(f, b, issueKey)
	}
	return newTextField(f)
}

// fieldsPane lists an issue's editable fields and edits one at a time. Changes
// are kept until they're all saved together.
type fieldsPane struct {
	choiceList
	key     string
	backend Backend

	// orig holds the values as loaded, to tell which fields changed.
	orig   []jira.EditField
	fields []fieldInput
	// editing is the field being edited, if any.
	editing fieldInput
	loaded  bool
	saving  bool
	err     error
}

func newFieldsPane(b Backend, issueKey string) fieldsPane {
	return fieldsPane{key: issueKey, backend: b}
}

func (p *fieldsPane) loading() bool {
	return (!p.loaded && p.err == nil) || p.saving
}

func (p *fieldsPane) capturing() bool { return p.editing != nil }

func (p *fieldsPane) unsaved() int { return len(p.changed()) }

func (p *fieldsPane) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case fieldsLoadedMsg:
		if msg.key == p.key {
			p.loaded, p.err, p.orig = true, msg.err, msg.fields
			p.fields = make([]fieldInput, len(msg.fields))
			for i, f := range msg.fields {
				p.fields[i] = newFieldInput(f, p.backend, p.key)
			}
		}
	case fieldsSavedMsg:
		if msg.key == p.key {
			p.saving, p.err = false, msg.err
		}
	case userSearchedMsg:
		if u, ok := p.editing.(*userField); ok {
			u.receive(msg)
		}
	case tea.KeyMsg:
		if key.Matches(msg, keys.send) {
			p.editing = nil
			return p.save()
		}
		if p.editing != nil {
			cmd, done := p.editing.update(msg)
			if done {
				p.editing = nil
			}
			return cmd
		}
		if p.handle(msg, len(p.fields)) || p.cursor >= len(p.fields) {
			break
		}
		switch {
		case key.Matches(msg, keys.accept):
			p.editing, p.err = p.fields[p.cursor], nil
		case key.Matches(msg, keys.undo):
			p.fields[p.cursor] = newFieldInput(p.orig[p.cursor], p.backend, p.key)
		}
	}
	return nil
}

func (p *fieldsPane) save() tea.Cmd {
	changed := p.changed()
	if p.saving || len(changed) == 0 {
		return nil
	}
	p.saving, p.err = true, nil
	b, issueKey := p.backend, p.key
	return func() tea.Msg {
		return fieldsSavedMsg{key: issueKey, count: len(changed), err: b.Edit(issueKey, changed)}
	}
}

// changed returns the fields that differ from how they loaded.
func (p *fieldsPane) changed() []jira.EditField {
	var out []jira.EditField
	for i, f := range p.fields {
		if p.isChanged(i) {
			out = append(out, f.value())
		}
	}
	return out
}

func (p *fieldsPane) isChanged(i int) bool {
	f, o := p.fields[i].value(), p.orig[i]
	return f.Text != o.Text || !sameSet(f.Chosen, o.Chosen)
}

func (p *fieldsPane) view(spin string, innerW, height int, _ string) []string {
	var lines []string
	switch {
	case !p.loaded && p.err == nil:
		lines = append(lines, spin+" Loading fields…")
	case p.loaded && len(p.fields) == 0 && p.err == nil:
		lines = append(lines, dimStyle.Render("No editable fields."))
	case p.editing != nil:
		lines = append([]string{titleStyle.Render(p.editing.value().Name)}, p.editing.view(innerW, height-3)...)
	default:
		lines = p.renderWindow(p.rows(innerW), innerW, height-2)
	}
	return append(lines, "", statusLine(spin, innerW, p.err, p.saving, "Saving…", p.hint()))
}

func (p *fieldsPane) rows(innerW int) []string {
	nameW := 0
	for _, f := range p.fields {
		nameW = max(nameW, ansi.StringWidth(f.value().Name))
	}
	nameW = min(nameW, innerW/3)
	rows := make([]string, len(p.fields))
	for i, f := range p.fields {
		name := ansi.Truncate(f.value().Name, nameW, "…")
		rows[i] = name + strings.Repeat(" ", nameW-ansi.StringWidth(name)) + "  " + displayValue(f.value())
		if p.isChanged(i) {
			rows[i] += " (edited)"
		}
	}
	return rows
}

func (p *fieldsPane) hint() string {
	switch {
	case p.editing != nil:
		return p.editing.hint()
	case p.unsaved() > 0:
		return "enter edit · u undo"
	}
	return "enter edit"
}

// textField edits a one-line value. A labels field is typed as words separated
// by spaces.
type textField struct {
	jira.EditField
	input textinput.Model
}

func newTextField(f jira.EditField) *textField {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Cursor.SetMode(cursor.CursorStatic)
	if f.Kind == jira.FieldLabels {
		ti.SetValue(strings.Join(f.Chosen, " "))
	} else {
		ti.SetValue(f.Text)
	}
	ti.Focus()
	return &textField{EditField: f, input: ti}
}

func (f *textField) value() jira.EditField { return f.EditField }

func (f *textField) update(msg tea.KeyMsg) (tea.Cmd, bool) {
	if key.Matches(msg, keys.accept, keys.blur) {
		return nil, true
	}
	var cmd tea.Cmd
	f.input, cmd = f.input.Update(msg)
	if f.Kind == jira.FieldLabels {
		f.Chosen = strings.Fields(f.input.Value())
	} else {
		f.Text = f.input.Value()
	}
	return cmd, false
}

func (f *textField) view(innerW, _ int) []string {
	border := inputBorder.BorderForeground(highlight)
	f.input.Width = innerW - border.GetHorizontalFrameSize() - 1
	return []string{border.Render(f.input.View())}
}

func (f *textField) hint() string {
	switch f.Kind {
	case jira.FieldLabels:
		return "separate labels with spaces · enter done"
	case jira.FieldDate:
		return "YYYY-MM-DD · enter done"
	}
	return "enter done · ctrl+s save"
}

// areaField edits multi-line text.
type areaField struct {
	jira.EditField
	area textarea.Model
}

func newAreaField(f jira.EditField) *areaField {
	ta := textarea.New()
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	// The default caps the value at 99 lines, which would cut long
	// descriptions short.
	ta.MaxHeight = 0
	ta.Cursor.SetMode(cursor.CursorStatic)
	ta.SetValue(f.Text)
	ta.Focus()
	return &areaField{EditField: f, area: ta}
}

func (f *areaField) value() jira.EditField { return f.EditField }

func (f *areaField) update(msg tea.KeyMsg) (tea.Cmd, bool) {
	// enter adds a line, so only esc finishes.
	if key.Matches(msg, keys.blur) {
		return nil, true
	}
	var cmd tea.Cmd
	f.area, cmd = f.area.Update(msg)
	f.Text = f.area.Value()
	return cmd, false
}

func (f *areaField) view(innerW, height int) []string {
	border := inputBorder.BorderForeground(highlight)
	f.area.SetWidth(innerW - border.GetHorizontalFrameSize())
	f.area.SetHeight(max(1, height-border.GetVerticalFrameSize()))
	return []string{border.Render(f.area.View())}
}

func (f *areaField) hint() string { return "esc done · ctrl+s save" }

// selectField picks one of a field's options, or several for a multi-select.
type selectField struct {
	jira.EditField
	list choiceList
}

func newSelectField(f jira.EditField) *selectField {
	s := &selectField{EditField: f}
	if f.Kind == jira.FieldSelect {
		s.list.cursor = max(0, slices.IndexFunc(s.rows(), func(o jira.Option) bool {
			return o.ID == "" && len(f.Chosen) == 0 || slices.Contains(f.Chosen, o.ID)
		}))
	}
	return s
}

func (f *selectField) value() jira.EditField { return f.EditField }

// rows lists the choices, with a way to clear a single select that isn't
// required.
func (f *selectField) rows() []jira.Option {
	if f.Kind == jira.FieldSelect && !f.Required {
		return append([]jira.Option{{Name: "(none)"}}, f.Options...)
	}
	return f.Options
}

func (f *selectField) update(msg tea.KeyMsg) (tea.Cmd, bool) {
	rows := f.rows()
	if f.list.handle(msg, len(rows)) {
		return nil, false
	}
	id := rows[f.list.cursor].ID
	switch {
	case key.Matches(msg, keys.blur):
		return nil, true
	case f.Kind == jira.FieldMultiSelect && key.Matches(msg, keys.toggle):
		if i := slices.Index(f.Chosen, id); i >= 0 {
			f.Chosen = slices.Delete(f.Chosen, i, i+1)
		} else {
			f.Chosen = append(f.Chosen, id)
		}
	case key.Matches(msg, keys.accept):
		if f.Kind == jira.FieldSelect {
			f.Chosen = nil
			if id != "" {
				f.Chosen = []string{id}
			}
		}
		return nil, true
	}
	return nil, false
}

func (f *selectField) view(innerW, height int) []string {
	rows := f.rows()
	labels := make([]string, len(rows))
	for i, o := range rows {
		labels[i] = o.Name
		if f.Kind == jira.FieldMultiSelect {
			mark := "[ ] "
			if slices.Contains(f.Chosen, o.ID) {
				mark = "[x] "
			}
			labels[i] = mark + o.Name
		}
	}
	return f.list.renderWindow(labels, innerW, height)
}

func (f *selectField) hint() string {
	if f.Kind == jira.FieldMultiSelect {
		return "space toggle · enter done"
	}
	return "enter choose · esc cancel"
}

func displayValue(f jira.EditField) string {
	var s string
	switch f.Kind {
	case jira.FieldSelect, jira.FieldMultiSelect, jira.FieldUser, jira.FieldMultiUser:
		names := make([]string, len(f.Chosen))
		for i, id := range f.Chosen {
			names[i] = id
			if j := slices.IndexFunc(f.Options, func(o jira.Option) bool { return o.ID == id }); j >= 0 {
				names[i] = f.Options[j].Name
			}
		}
		s = strings.Join(names, ", ")
	case jira.FieldLabels:
		s = strings.Join(f.Chosen, " ")
	default:
		first, rest, more := strings.Cut(strings.TrimSpace(f.Text), "\n")
		s = first
		if more && strings.TrimSpace(rest) != "" {
			s += " …"
		}
	}
	if s == "" {
		return "—"
	}
	return s
}

// sameSet reports whether a and b hold the same values in any order.
func sameSet(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
