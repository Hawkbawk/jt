package tui

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

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

// fieldValue holds what a form field is bound to, next to the field as it
// loaded.
type fieldValue struct {
	jira.EditField
	text   string
	single string
	multi  []string
	input  huh.Field
}

func newFieldValue(f jira.EditField) *fieldValue {
	v := &fieldValue{EditField: f, multi: slices.Clone(f.Chosen)}
	if f.Kind == jira.FieldLabels {
		v.text = strings.Join(f.Chosen, " ")
	} else {
		v.text = f.Text
	}
	if len(f.Chosen) > 0 {
		v.single = f.Chosen[0]
	}
	return v
}

// current returns the field with the values entered so far.
func (v *fieldValue) current() jira.EditField {
	f := v.EditField
	switch f.Kind {
	case jira.FieldLabels:
		f.Chosen = strings.Fields(v.text)
	case jira.FieldSelect:
		f.Chosen = nil
		if v.single != "" {
			f.Chosen = []string{v.single}
		}
	case jira.FieldMultiSelect:
		f.Chosen = slices.Clone(v.multi)
	default:
		f.Text = v.text
	}
	return f
}

func (v *fieldValue) changed() bool {
	f := v.current()
	return f.Text != v.Text || !sameSet(f.Chosen, v.Chosen)
}

// huhField builds the form field that edits v.
func (v *fieldValue) huhField() huh.Field {
	switch v.Kind {
	case jira.FieldTextArea:
		return huh.NewText().Title(v.Name).CharLimit(0).ShowLineNumbers(false).Value(&v.text)
	case jira.FieldSelect:
		var opts []huh.Option[string]
		if !v.Required {
			opts = append(opts, huh.NewOption("(none)", ""))
		}
		for _, o := range v.Options {
			opts = append(opts, huh.NewOption(o.Name, o.ID))
		}
		return huh.NewSelect[string]().Title(v.Name).Options(opts...).Height(listHeight(len(opts))).Value(&v.single)
	case jira.FieldMultiSelect:
		opts := make([]huh.Option[string], len(v.Options))
		for i, o := range v.Options {
			opts[i] = huh.NewOption(o.Name, o.ID).Selected(slices.Contains(v.Chosen, o.ID))
		}
		return huh.NewMultiSelect[string]().Title(v.Name).Options(opts...).Height(listHeight(len(opts))).Value(&v.multi)
	}
	in := huh.NewInput().Title(v.Name).Value(&v.text)
	switch v.Kind {
	case jira.FieldLabels:
		in.Placeholder("separate labels with spaces")
	case jira.FieldDate:
		in.Placeholder("YYYY-MM-DD")
	}
	return in
}

// scrollStyle makes the position indicator stand out from the dim hints.
var scrollStyle = lipgloss.NewStyle().Foreground(highlight).Bold(true)

// minTextLines is the height of a text area that isn't being edited.
const minTextLines = 5

// listHeight is the height of a select with n options and its title.
func listHeight(n int) int {
	return min(n, 6) + 1
}

// fieldsPane edits all of an issue's editable fields in one form. Changes are
// kept until they're all saved together.
type fieldsPane struct {
	key     string
	backend Backend

	vals   []*fieldValue
	form   *huh.Form
	loaded bool
	saving bool
	err    error
	// formH is the height the form was last drawn at.
	formH int
}

func newFieldsPane(b Backend, issueKey string) fieldsPane {
	return fieldsPane{key: issueKey, backend: b}
}

func (p *fieldsPane) loading() bool {
	return (!p.loaded && p.err == nil) || p.saving
}

// capturing is true while the form is up, since it takes typed text.
func (p *fieldsPane) capturing() bool { return p.form != nil }

func (p *fieldsPane) unsaved() int { return len(p.changed()) }

// newForm builds the form from the values entered so far.
func (p *fieldsPane) newForm() *huh.Form {
	fields := make([]huh.Field, len(p.vals))
	for i, v := range p.vals {
		v.input = v.huhField()
		fields[i] = v.input
	}
	km := huh.NewDefaultKeyMap()
	// ctrl+c is the app's to handle.
	km.Quit = key.NewBinding(key.WithDisabled())
	return huh.NewForm(huh.NewGroup(fields...)).
		WithTheme(huh.ThemeFunc(huh.ThemeBase16)).
		WithKeyMap(km).
		WithShowHelp(false)
}

func (p *fieldsPane) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case fieldsLoadedMsg:
		if msg.key != p.key {
			break
		}
		p.loaded, p.err = true, msg.err
		for _, f := range msg.fields {
			p.vals = append(p.vals, newFieldValue(f))
		}
		if len(p.vals) > 0 {
			p.form = p.newForm()
			return p.form.Init()
		}
		return nil
	case fieldsSavedMsg:
		if msg.key == p.key {
			p.saving, p.err = false, msg.err
		}
		return nil
	}
	if p.form == nil || p.saving {
		return nil
	}
	if km, ok := msg.(tea.KeyMsg); ok && key.Matches(km, keys.send) {
		return p.save()
	}
	_, cmd := p.form.Update(msg)
	p.growFocusedText()
	// Submitting the form ends it, so the form is rebuilt to stay editable
	// if the save fails.
	if p.form.State == huh.StateCompleted {
		p.form = p.newForm()
		return tea.Batch(p.save(), p.form.Init())
	}
	return cmd
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
	for _, v := range p.vals {
		if v.changed() {
			out = append(out, v.current())
		}
	}
	return out
}

func (p *fieldsPane) view(spin string, innerW, height int, _ string) []string {
	var lines []string
	resized := p.formH != max(1, height-2)
	p.formH = max(1, height-2)
	switch {
	case !p.loaded && p.err == nil:
		lines = append(lines, spin+" Loading fields…")
	case p.loaded && len(p.vals) == 0 && p.err == nil:
		lines = append(lines, dimStyle.Render("No editable fields."))
	case p.form != nil:
		p.form.WithWidth(innerW).WithHeight(p.formH)
		if resized {
			p.growFocusedText()
		}
		lines = strings.Split(p.form.View(), "\n")
	}
	return append(lines, "", statusLine(spin, innerW, p.err, p.saving, "Saving…", p.hint()))
}

// refreshMsg makes the form redraw its content, which it only does on updates.
type refreshMsg struct{}

// growFocusedText gives the text area being edited most of the form's height,
// leaving the other fields at their usual size.
func (p *fieldsPane) growFocusedText() {
	for _, v := range p.vals {
		if t, ok := v.input.(*huh.Text); ok {
			t.Lines(minTextLines)
			if v.input == p.form.GetFocusedField() {
				t.Lines(max(minTextLines, p.formH-3))
			}
		}
	}
	p.form.Update(refreshMsg{})
}

// hint shows where the focused field sits in the form, with arrows for the
// fields out of view above or below it.
func (p *fieldsPane) hint() string {
	if p.form == nil {
		return ""
	}
	i := slices.IndexFunc(p.vals, func(v *fieldValue) bool { return v.input == p.form.GetFocusedField() })
	pos := ""
	if i >= 0 {
		up, down := " ", " "
		if i > 0 {
			up = "▲"
		}
		if i < len(p.vals)-1 {
			down = "▼"
		}
		pos = scrollStyle.Render(fmt.Sprintf("%s%s %d/%d", up, down, i+1, len(p.vals))) + dimStyle.Render(" · ")
	}
	return pos + dimStyle.Render("tab/enter next · shift+tab back · ctrl+j new line · ctrl+s save")
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
