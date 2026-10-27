package tui

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

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

	// query and people are only set while a user field is edited.
	query  string
	people *userSearch
	rev    peopleRev
	// before is the entered values when editing began, to put back on esc.
	before entered
}

// entered is the values a form field is bound to.
type entered struct {
	text   string
	single string
	multi  []string
}

func newFieldValue(f jira.EditField) *fieldValue {
	v := &fieldValue{EditField: f}
	v.reset()
	return v
}

// reset puts the entered values back to how the field loaded.
func (v *fieldValue) reset() {
	v.multi = slices.Clone(v.Chosen)
	v.text, v.single = v.Text, ""
	if v.Kind == jira.FieldLabels {
		v.text = strings.Join(v.Chosen, " ")
	}
	if len(v.Chosen) > 0 {
		v.single = v.Chosen[0]
	}
}

// begin remembers the entered values, for revert.
func (v *fieldValue) begin() {
	v.before = v.entered()
}

func (v *fieldValue) entered() entered {
	return entered{v.text, v.single, slices.Clone(v.multi)}
}

// revert puts back the values entered when editing began.
func (v *fieldValue) revert() {
	v.text, v.single, v.multi = v.before.text, v.before.single, slices.Clone(v.before.multi)
}

// current returns the field with the values entered so far.
func (v *fieldValue) current() jira.EditField {
	return v.with(v.entered())
}

// with returns the field holding the values in e.
func (v *fieldValue) with(e entered) jira.EditField {
	f := v.EditField
	switch f.Kind {
	case jira.FieldLabels:
		f.Chosen = strings.Fields(e.text)
	case jira.FieldSelect, jira.FieldUser:
		f.Chosen = nil
		if e.single != "" {
			f.Chosen = []string{e.single}
		}
	case jira.FieldMultiSelect, jira.FieldMultiUser:
		f.Chosen = slices.Clone(e.multi)
	default:
		f.Text = e.text
	}
	return f
}

func (v *fieldValue) changed() bool {
	return v.differs(v.current())
}

// differs reports whether f holds something other than how the field loaded.
func (v *fieldValue) differs(f jira.EditField) bool {
	return f.Text != v.Text || !sameSet(f.Chosen, v.Chosen)
}

// FilterValue implements list.Item.
func (v *fieldValue) FilterValue() string { return v.Name }

// Title implements list.DefaultItem.
func (v *fieldValue) Title() string {
	title := v.Name
	if v.Required {
		title += " *"
	}
	if v.changed() {
		title += " ●"
	}
	return title
}

// Description implements list.DefaultItem, showing the value entered so far.
func (v *fieldValue) Description() string {
	f := v.current()
	var s string
	switch f.Kind {
	case jira.FieldSelect, jira.FieldMultiSelect, jira.FieldUser, jira.FieldMultiUser:
		var names []string
		for _, id := range f.Chosen {
			i := slices.IndexFunc(f.Options, func(o jira.Option) bool { return o.ID == id })
			if i >= 0 {
				names = append(names, f.Options[i].Name)
			}
		}
		s = strings.Join(names, ", ")
	case jira.FieldLabels:
		s = strings.Join(f.Chosen, ", ")
	default:
		s, _, _ = strings.Cut(f.Text, "\n")
	}
	if s == "" {
		return "(empty)"
	}
	return s
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
			opts[i] = huh.NewOption(o.Name, o.ID).Selected(slices.Contains(v.multi, o.ID))
		}
		return huh.NewMultiSelect[string]().Title(v.Name).Options(opts...).Height(listHeight(len(opts))).Value(&v.multi)
	}
	in := huh.NewInput().Title(v.Name).Value(&v.text)
	switch v.Kind {
	case jira.FieldLabels:
		in.Placeholder("separate labels with spaces")
	case jira.FieldNumber:
		in.Validate(v.validate)
	case jira.FieldDate:
		in.Placeholder("YYYY-MM-DD or relative like -2d").Validate(v.validate)
	case jira.FieldDateTime:
		in.Placeholder("YYYY-MM-DDTHH:MM or relative like -2d").Validate(v.validate)
	}
	return in
}

// validate checks the text entered for number, date and date time fields. An empty value
// clears the field. Only optional fields can have an empty value.
func (v *fieldValue) validate(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		if v.Required {
			return errors.New("required")
		}
		return nil
	}
	switch v.Kind {
	case jira.FieldNumber:
		if _, err := strconv.ParseFloat(s, 64); err != nil {
			return errors.New("must be a number")
		}
	case jira.FieldDate, jira.FieldDateTime:
		if _, err := v.parseTime(s, time.Now()); err != nil {
			return err
		}
	}
	return nil
}

// minTextLines is the height of a text area that isn't being edited.
const minTextLines = 5

// listHeight is the height of a select with n options and its title.
func listHeight(n int) int {
	return min(n, 6) + 1
}

// fieldsPane lists an issue's editable fields and edits one at a time in a
// form. Changes are kept until they're all saved together.
type fieldsPane struct {
	key     string
	backend Backend

	vals    []*fieldValue
	list    list.Model
	editing *fieldValue
	form    *huh.Form
	loaded  bool
	saving  bool
	err     error
	// formH is the height the form was last drawn at.
	formH int
}

func newFieldsPane(b Backend, issueKey string) fieldsPane {
	return fieldsPane{key: issueKey, backend: b}
}

func (p *fieldsPane) loading() bool {
	return (!p.loaded && p.err == nil) || p.saving
}

// capturing is true while typed text goes to a form field or the list filter.
func (p *fieldsPane) capturing() bool {
	return p.editing != nil || p.list.SettingFilter()
}

// consumesEsc is true when esc steps back within the pane instead of closing it.
func (p *fieldsPane) consumesEsc() bool {
	return p.editing != nil || p.list.FilterState() != list.Unfiltered
}

func (p *fieldsPane) unsaved() int { return len(p.changed()) }

func newFieldList(vals []*fieldValue) list.Model {
	items := make([]list.Item, len(vals))
	for i, v := range vals {
		items[i] = v
	}
	d := list.NewDefaultDelegate()
	d.SetSpacing(0)
	l := list.New(items, d, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()
	return l
}

// edit opens a form for v alone.
func (p *fieldsPane) edit(v *fieldValue) tea.Cmd {
	p.editing = v
	v.begin()
	fields := []huh.Field{}
	if v.isUser() {
		v.query, v.people = "", newUserSearch(p.backend, p.key)
		fields = v.userFields()
		v.input = fields[0]
	} else {
		v.input = v.huhField()
		fields = append(fields, v.input)
	}
	km := huh.NewDefaultKeyMap()
	// ctrl+c is the app's to handle.
	km.Quit = key.NewBinding(key.WithDisabled())
	if v.isUser() {
		// The search box above the list is the way to narrow people down.
		km.Select.Filter.SetEnabled(false)
		km.MultiSelect.Filter.SetEnabled(false)
	}
	p.form = huh.NewForm(huh.NewGroup(fields...)).
		WithTheme(huh.ThemeFunc(huh.ThemeBase16)).
		WithKeyMap(km).
		WithShowHelp(false)
	p.growText()
	return p.form.Init()
}

// closeForm goes back to the list of fields.
func (p *fieldsPane) closeForm() {
	p.editing, p.form = nil, nil
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
		p.list = newFieldList(p.vals)
		return nil
	case fieldsSavedMsg:
		if msg.key == p.key {
			p.saving, p.err = false, msg.err
		}
		return nil
	}
	if !p.loaded || p.saving || len(p.vals) == 0 {
		return nil
	}
	km, isKey := msg.(tea.KeyMsg)
	if isKey && p.form == nil && key.Matches(km, keys.send) {
		return p.save()
	}
	if p.form == nil {
		if isKey && key.Matches(km, keys.accept) && !p.list.SettingFilter() {
			if v, ok := p.list.SelectedItem().(*fieldValue); ok {
				return p.edit(v)
			}
			return nil
		}
		if isKey && key.Matches(km, keys.undo) && !p.list.SettingFilter() {
			if v, ok := p.list.SelectedItem().(*fieldValue); ok {
				v.reset()
			}
			return nil
		}
		var cmd tea.Cmd
		p.list, cmd = p.list.Update(msg)
		return cmd
	}
	if isKey && key.Matches(km, keys.blur) {
		p.editing.revert()
		p.closeForm()
		return nil
	}
	_, cmd := p.form.Update(msg)
	if p.editing.isUser() {
		p.editing.rememberPeople()
	}
	p.growText()
	if p.form.State == huh.StateCompleted {
		p.editing.normalizeTime(time.Now())
		p.closeForm()
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

// changed returns the fields that differ from how they loaded. What is still
// being entered in an open form doesn't count until enter keeps it.
func (p *fieldsPane) changed() []jira.EditField {
	var out []jira.EditField
	for _, v := range p.vals {
		f := v.current()
		if v == p.editing {
			f = v.with(v.before)
		}
		if v.differs(f) {
			out = append(out, f)
		}
	}
	return out
}

// holding is true while a field's form is open, which blocks saving.
func (p *fieldsPane) holding() bool { return p.form != nil }

func (p *fieldsPane) view(spin string, innerW, height int, _ string) []string {
	var lines []string
	formH := max(1, height-2)
	switch {
	case !p.loaded && p.err == nil:
		lines = append(lines, spin+" Loading fields…")
	case p.loaded && len(p.vals) == 0 && p.err == nil:
		lines = append(lines, dimStyle.Render("No editable fields."))
	case p.form != nil:
		resized := p.formH != formH
		p.formH = formH
		p.form.WithWidth(innerW).WithHeight(formH)
		if resized {
			p.growText()
		}
		lines = strings.Split(p.form.View(), "\n")
	case p.loaded:
		p.list.SetSize(innerW, formH)
		lines = strings.Split(p.list.View(), "\n")
	}
	return append(lines, "", statusLine(spin, innerW, p.err, p.saving, "Saving…", p.hint()))
}

// refreshMsg makes the form redraw its content, which it only does on updates.
type refreshMsg struct{}

// growText gives a text area being edited most of the form's height.
func (p *fieldsPane) growText() {
	if t, ok := p.editing.input.(*huh.Text); ok {
		t.Lines(max(minTextLines, p.formH-3))
		p.form.Update(refreshMsg{})
	}
}

func (p *fieldsPane) hint() string {
	switch {
	case p.form != nil && p.editing.isUser():
		return dimStyle.Render("enter next · shift+tab back to search · space toggles · esc back")
	case p.form != nil:
		return dimStyle.Render("enter done · esc back · ctrl+j new line")
	case p.list.SettingFilter():
		return dimStyle.Render("enter apply filter · esc clear · ctrl+s save")
	}
	return dimStyle.Render("enter edit · u undo · / filter · esc close · ctrl+s save")
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
