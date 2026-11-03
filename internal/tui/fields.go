package tui

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/hawkbawk/jt/internal/components/usersearch"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/popup"
)

type fieldsLoadedMsg struct {
	id     int64
	key    string
	fields []jira.EditField
	err    error
}

type fieldsSavedMsg struct {
	id    int64
	key   string
	count int
	err   error
}

func loadFields(b Backend, id int64, iss jira.Issue) tea.Cmd {
	return func() tea.Msg {
		fs, err := b.EditFields(iss)
		return fieldsLoadedMsg{id: id, key: iss.Key, fields: fs, err: err}
	}
}

// fieldValue holds what a form field is bound to, next to the field as it
// loaded.
type fieldValue struct {
	jira.EditField
	text   string
	single jira.Option
	multi  []jira.Option
	input  huh.Field

	// people edits a user field. It's only set for those.
	people peoplePicker
	// before is the entered values when editing began, to put back on esc.
	before entered
}

// entered is the values a form field is bound to.
type entered struct {
	text   string
	single jira.Option
	multi  []jira.Option
}

func newFieldValue(f jira.EditField, search usersearch.Func) *fieldValue {
	v := &fieldValue{EditField: f}
	v.reset()
	if v.isUser() {
		v.people = v.newPeoplePicker(search)
	}
	return v
}

// reset puts the entered values back to how the field loaded.
func (v *fieldValue) reset() {
	v.multi = slices.Clone(v.Chosen)
	v.text, v.single = v.Text, jira.Option{}
	if v.Kind == jira.FieldLabels {
		v.text = strings.Join(jira.Names(v.Chosen), " ")
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
		f.Chosen = nil
		for _, l := range strings.Fields(e.text) {
			f.Chosen = append(f.Chosen, jira.Option{ID: l, Name: l})
		}
	case jira.FieldSelect, jira.FieldUser:
		f.Chosen = nil
		if e.single.ID != "" {
			f.Chosen = []jira.Option{e.single}
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
	case jira.FieldSelect, jira.FieldMultiSelect, jira.FieldUser, jira.FieldMultiUser, jira.FieldLabels:
		s = strings.Join(jira.Names(f.Chosen), ", ")
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
		var opts []huh.Option[jira.Option]
		if !v.Required {
			opts = append(opts, huh.NewOption("(none)", jira.Option{}))
		}
		for _, o := range v.Options {
			opts = append(opts, huh.NewOption(o.Name, o))
		}
		return huh.NewSelect[jira.Option]().Title(v.Name).Options(opts...).Height(listHeight(len(opts))).Value(&v.single)
	case jira.FieldMultiSelect:
		opts := make([]huh.Option[jira.Option], len(v.Options))
		for i, o := range v.Options {
			opts[i] = huh.NewOption(o.Name, o).Selected(jira.IndexID(v.multi, o.ID) >= 0)
		}
		return huh.NewMultiSelect[jira.Option]().Title(v.Name).Options(opts...).Height(listHeight(len(opts))).Value(&v.multi)
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

// editPopup lists an issue's editable fields and edits one at a time in a
// form. Changes are kept until they're all saved together.
type editPopup struct {
	id      int64
	key     string
	issue   jira.Issue
	backend Backend
	spin    spinner.Model
	help    help.Model

	width, height int
	// confirmClose is set while asking whether to drop unsaved changes.
	confirmClose bool

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

func newEditPopup(b Backend, iss jira.Issue) *editPopup {
	return &editPopup{id: newPopupID(), key: iss.Key, issue: iss, backend: b, spin: newSpinner(), help: styles.Help()}
}

// Init implements tea.Model.
func (p *editPopup) Init() tea.Cmd {
	return tea.Batch(loadFields(p.backend, p.id, p.issue), p.spin.Tick)
}

func (p *editPopup) loading() bool {
	return (!p.loaded && p.err == nil) || p.saving
}

// capturing is true while typed text goes to a form field or the list filter.
func (p *editPopup) capturing() bool {
	return p.editing != nil || p.list.SettingFilter()
}

// consumesEsc is true when esc steps back within the pane instead of closing it.
func (p *editPopup) consumesEsc() bool {
	return p.editing != nil || p.list.FilterState() != list.Unfiltered
}

func (p *editPopup) unsaved() int { return len(p.changed()) }

// fieldDelegate is the themed delegate with no gap between rows.
func fieldDelegate() list.DefaultDelegate {
	d := themedDelegate()
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

// edit opens a form for v alone.
func (p *editPopup) edit(v *fieldValue) tea.Cmd {
	p.editing = v
	v.begin()
	if v.isUser() {
		v.input = v.people.Field()
	} else {
		v.input = v.huhField()
	}
	p.form = newForm(v.input)
	p.growText()
	return p.form.Init()
}

// newForm puts fields in a themed form that leaves ctrl+c to the app.
func newForm(fields ...huh.Field) *huh.Form {
	return newGroupsForm(huh.NewGroup(fields...))
}

func newGroupsForm(groups ...*huh.Group) *huh.Form {
	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(key.WithDisabled())
	return huh.NewForm(groups...).
		WithTheme(styles.FormTheme()).
		WithKeyMap(km).
		WithShowHelp(false)
}

// closeForm goes back to the list of fields.
func (p *editPopup) closeForm() {
	p.editing, p.form = nil, nil
}

// Update implements tea.Model.
func (p *editPopup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	was := p.loading()
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width, p.height = msg.Width, msg.Height
	case spinner.TickMsg:
		cmd = tickSpinner(&p.spin, msg, p.loading())
	case tea.KeyPressMsg:
		cmd = p.updateKey(msg)
	default:
		cmd = p.update(msg)
	}
	return p, restartSpinner(p.spin, was, p.loading(), cmd)
}

func (p *editPopup) updateKey(km tea.KeyPressMsg) tea.Cmd {
	if p.confirmClose {
		return p.updateConfirm(km)
	}
	if p.consumesEsc() && key.Matches(km, keys.blur) {
		return p.update(km)
	}
	closes := keys.close
	if p.capturing() {
		closes = keys.blur
	}
	if key.Matches(km, closes) {
		if p.unsaved() > 0 {
			p.confirmClose = true
			return nil
		}
		return popup.Close()
	}
	return p.update(km)
}

func (p *editPopup) updateConfirm(km tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(km, keys.confirm):
		return popup.Close()
	case key.Matches(km, keys.deny):
		p.confirmClose = false
	case key.Matches(km, keys.send):
		p.confirmClose = false
		return p.update(km)
	}
	return nil
}

func (p *editPopup) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case fieldsLoadedMsg:
		if msg.id != p.id {
			break
		}
		p.loaded, p.err = true, msg.err
		for _, f := range msg.fields {
			p.vals = append(p.vals, newFieldValue(f, searchUsers(p.backend, p.key)))
		}
		p.list = newFieldList(p.vals)
		return nil
	case fieldsSavedMsg:
		if msg.id != p.id {
			return nil
		}
		p.saving, p.err = false, msg.err
		if msg.err == nil {
			status := fmt.Sprintf("Updated %d %s on %s", msg.count, plural(msg.count, "field"), msg.key)
			return closeEdited(msg.key, status)
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
	p.growText()
	if p.form.State == huh.StateCompleted {
		p.editing.normalizeTime(time.Now())
		p.closeForm()
	}
	return cmd
}

func (p *editPopup) save() tea.Cmd {
	changed := p.changed()
	if p.saving || len(changed) == 0 {
		return nil
	}
	p.saving, p.err = true, nil
	b, id, issueKey := p.backend, p.id, p.key
	return func() tea.Msg {
		return fieldsSavedMsg{id: id, key: issueKey, count: len(changed), err: b.Edit(issueKey, changed)}
	}
}

// changed returns the fields that differ from how they loaded. What is still
// being entered in an open form doesn't count until enter keeps it.
func (p *editPopup) changed() []jira.EditField {
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
func (p *editPopup) holding() bool { return p.form != nil }

// View implements tea.Model.
func (p *editPopup) View() tea.View {
	p.help.SetWidth(p.width)
	title := styles.Title().Render("Edit · " + p.key)
	footer := p.help.View(paneHelpKeys)
	if n := p.unsaved(); n > 0 {
		changes := fmt.Sprintf("%d unsaved %s", n, plural(n, "change"))
		warn := "● " + changes
		if !p.holding() {
			warn += " · ctrl+s to save"
		}
		title += "  " + styles.Warn().Render(warn)
		if p.confirmClose {
			footer = styles.Warn().Render("Discard " + changes + "? y discard · n keep editing · ctrl+s save")
		}
	}
	spin := p.spin.View()
	var lines []string
	formH := max(1, p.height-issueChromeLines-2)
	switch {
	case !p.loaded && p.err == nil:
		lines = append(lines, spin+" Loading fields…")
	case p.loaded && len(p.vals) == 0 && p.err == nil:
		lines = append(lines, styles.Dim().Render("No editable fields."))
	case p.form != nil:
		resized := p.formH != formH
		p.formH = formH
		p.form.WithWidth(p.width).WithHeight(formH)
		if resized {
			p.growText()
		}
		lines = strings.Split(p.form.View(), "\n")
	case p.loaded:
		p.list.SetSize(p.width, formH)
		lines = strings.Split(p.list.View(), "\n")
	}
	lines = append(lines, "", statusLine(spin, p.width, p.err, p.saving, "Saving…", p.hint()))
	return tea.NewView(popup.Stack(p.height, []string{title, ""}, lines, footer))
}

// refreshMsg makes the form redraw its content, which it only does on updates.
type refreshMsg struct{}

// growText gives a text area being edited most of the form's height.
func (p *editPopup) growText() {
	if t, ok := p.editing.input.(*huh.Text); ok {
		t.Lines(max(minTextLines, p.formH-3))
		p.form.Update(refreshMsg{})
	}
}

func (p *editPopup) hint() string {
	switch {
	case p.form != nil && p.editing.isUser():
		return styles.Dim().Render("enter next · shift+tab back to search · space toggles · esc back")
	case p.form != nil:
		return styles.Dim().Render("enter done · esc back · ctrl+j new line")
	case p.list.SettingFilter():
		return styles.Dim().Render("enter apply filter · esc clear · ctrl+s save")
	}
	return styles.Dim().Render("enter edit · u undo · / filter · esc close · ctrl+s save")
}

// sameSet reports whether a and b hold the same options, by id.
func sameSet(a, b []jira.Option) bool {
	ids := func(opts []jira.Option) []string {
		out := make([]string, len(opts))
		for i, o := range opts {
			out[i] = o.ID
		}
		slices.Sort(out)
		return out
	}
	return slices.Equal(ids(a), ids(b))
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
