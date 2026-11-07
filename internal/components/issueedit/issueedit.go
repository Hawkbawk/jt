// Package issueedit is the popup that edits an issue's fields. It lists the
// fields the issue allows, edits one at a time in a form, and saves the changes
// together. When a save succeeds it closes with an Edited message.
package issueedit

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
	"github.com/hawkbawk/jt/internal/keys"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/popup"
	"github.com/hawkbawk/jt/internal/tui/popup/pane"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
)

// Backend loads and saves an issue's fields.
type Backend interface {
	EditFields(iss jira.Issue) ([]jira.EditField, error)
	Edit(key string, fields []jira.EditField) error
	SearchUsers(issueKey, query string) ([]jira.Option, error)
}

// Edited is what the popup closes with after saving. It's a tuimsg.Finished.
type Edited struct {
	Key   string
	Count int
}

var _ tuimsg.Finished = Edited{}

// IssueKey implements tuimsg.Finished.
func (e Edited) IssueKey() string { return e.Key }

// Status implements tuimsg.Finished.
func (e Edited) Status() string {
	return fmt.Sprintf("Updated %d %s on %s", e.Count, plural(e.Count, "field"), e.Key)
}

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
		for l := range strings.FieldsSeq(e.text) {
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
		return huh.NewText().Title(v.Name).CharLimit(0).ShowLineNumbers(false).ExternalEditor(true).Value(&v.text)
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

// helpMode picks which of the popup's help footers keyMap shows.
type helpMode int

const (
	helpList helpMode = iota
	helpFilter
	helpField
	helpUserField
	helpDiscard
	helpSave
)

// keyMap is the popup's key bindings. mode picks the help it shows.
type keyMap struct {
	close, blur, confirm, deny, accept, send, undo key.Binding
	choose, apply, cancel                          key.Binding
	filter, next, back, toggle                     key.Binding

	mode helpMode
	form []key.Binding
}

func newKeyMap() keyMap {
	k := keys.Get()
	return keyMap{
		close:   k.Close,
		blur:    k.Dismiss,
		confirm: k.Confirm,
		deny:    k.Deny,
		accept:  k.Accept,
		send:    k.Send,
		undo:    keys.Describe(k.Undo, "undo"),
		choose:  keys.Describe(k.ListNavigate, "select"),
		apply:   keys.Describe(k.Accept, "apply"),
		cancel:  keys.Describe(k.Dismiss, "cancel"),
		filter:  keys.Describe(k.ListFilter, "filter"),
		next:    keys.Describe(k.Accept, "next"),
		back:    keys.Describe(k.PrevTab, "back to search"),
		toggle:  keys.Describe(k.Toggle, "toggles"),
	}
}

// helpPane is the mode of the footer under the popup's list and forms.
const helpPane helpMode = -1

func (k keyMap) withMode(m helpMode) keyMap {
	k.mode = m
	return k
}

func (k keyMap) withForm(binds []key.Binding) keyMap {
	k.form = binds
	return k
}

// ShortHelp implements help.KeyMap.
func (k keyMap) ShortHelp() []key.Binding {
	save := keys.Describe(k.send, "save")
	switch k.mode {
	case helpSave:
		return []key.Binding{save}
	case helpDiscard:
		return []key.Binding{keys.Describe(k.confirm, "discard"), keys.Describe(k.deny, "keep editing"), save}
	case helpUserField:
		return []key.Binding{k.next, k.back, k.toggle, keys.Describe(k.blur, "back")}
	case helpField:
		return append([]key.Binding{keys.Describe(k.accept, "done"), keys.Describe(k.blur, "back")}, k.form...)
	case helpFilter:
		return []key.Binding{keys.Describe(k.accept, "apply filter"), keys.Describe(k.blur, "clear"), save}
	case helpList:
		return []key.Binding{keys.Describe(k.accept, "edit"), k.undo, k.filter, keys.Describe(k.blur, "close"), save}
	}
	return []key.Binding{k.choose, k.apply, k.cancel}
}

// FullHelp implements help.KeyMap.
func (k keyMap) FullHelp() [][]key.Binding { return [][]key.Binding{k.ShortHelp()} }

// Model lists an issue's editable fields and edits one at a time in a
// form. Changes are kept until they're all saved together.
type Model struct {
	keys    keyMap
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

// New returns the popup that edits iss, which loads the fields when it opens.
func New(b Backend, iss jira.Issue) *Model {
	return &Model{id: pane.NewID(), key: iss.Key, issue: iss, backend: b, spin: pane.NewSpinner(), help: styles.Help(), keys: newKeyMap()}
}

// Init implements tea.Model.
func (p *Model) Init() tea.Cmd {
	return tea.Batch(loadFields(p.backend, p.id, p.issue), p.spin.Tick)
}

func (p *Model) loading() bool {
	return (!p.loaded && p.err == nil) || p.saving
}

// capturing is true while typed text goes to a form field or the list filter.
func (p *Model) capturing() bool {
	return p.editing != nil || p.list.SettingFilter()
}

// consumesEsc is true when esc steps back within the pane instead of closing it.
func (p *Model) consumesEsc() bool {
	return p.editing != nil || p.list.FilterState() != list.Unfiltered
}

func (p *Model) unsaved() int { return len(p.changed()) }

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

// edit opens a form for v alone.
func (p *Model) edit(v *fieldValue) tea.Cmd {
	p.editing = v
	v.begin()
	if v.isUser() {
		v.input = v.people.Field()
	} else {
		v.input = v.huhField()
	}
	p.form = pane.NewForm(v.input)
	p.growText()
	return p.form.Init()
}

// closeForm goes back to the list of fields.
func (p *Model) closeForm() {
	p.editing, p.form = nil, nil
}

// Update implements tea.Model.
func (p *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	was := p.loading()
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width, p.height = msg.Width, msg.Height
	case spinner.TickMsg:
		cmd = pane.Tick(&p.spin, msg, p.loading())
	case tea.KeyPressMsg:
		cmd = p.updateKey(msg)
	default:
		cmd = p.update(msg)
	}
	return p, pane.Restart(p.spin, was, p.loading(), cmd)
}

func (p *Model) updateKey(km tea.KeyPressMsg) tea.Cmd {
	if p.confirmClose {
		return p.updateConfirm(km)
	}
	if p.consumesEsc() && key.Matches(km, p.keys.blur) {
		return p.update(km)
	}
	closes := p.keys.close
	if p.capturing() {
		closes = p.keys.blur
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

func (p *Model) updateConfirm(km tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(km, p.keys.confirm):
		return popup.Close()
	case key.Matches(km, p.keys.deny):
		p.confirmClose = false
	case key.Matches(km, p.keys.send):
		p.confirmClose = false
		return p.update(km)
	}
	return nil
}

func (p *Model) update(msg tea.Msg) tea.Cmd {
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
			return popup.CloseWith(Edited{Key: msg.key, Count: msg.count})
		}
		return nil
	}
	if !p.loaded || p.saving || len(p.vals) == 0 {
		return nil
	}
	km, isKey := msg.(tea.KeyMsg)
	if isKey && p.form == nil && key.Matches(km, p.keys.send) {
		return p.save()
	}
	if p.form == nil {
		if isKey && key.Matches(km, p.keys.accept) && !p.list.SettingFilter() {
			if v, ok := p.list.SelectedItem().(*fieldValue); ok {
				return p.edit(v)
			}
			return nil
		}
		if isKey && key.Matches(km, p.keys.undo) && !p.list.SettingFilter() {
			if v, ok := p.list.SelectedItem().(*fieldValue); ok {
				v.reset()
			}
			return nil
		}
		var cmd tea.Cmd
		p.list, cmd = p.list.Update(msg)
		return cmd
	}
	if isKey && key.Matches(km, p.keys.blur) {
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

func (p *Model) save() tea.Cmd {
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
func (p *Model) changed() []jira.EditField {
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
func (p *Model) holding() bool { return p.form != nil }

// View implements tea.Model.
func (p *Model) View() tea.View {
	p.help.SetWidth(p.width)
	title := styles.Title().Render("Edit · " + p.key)
	footer := p.help.View(p.keys.withMode(helpPane))
	if n := p.unsaved(); n > 0 {
		changes := fmt.Sprintf("%d unsaved %s", n, plural(n, "change"))
		warn := styles.Warn().Render("● " + changes)
		if !p.holding() {
			warn += styles.Dim().Render(" · ") + p.help.View(p.keys.withMode(helpSave))
		}
		title += "  " + warn
		if p.confirmClose {
			footer = styles.Warn().Render("Discard "+changes+"? ") + p.help.View(p.keys.withMode(helpDiscard))
		}
	}
	spin := p.spin.View()
	var lines []string
	formH := max(1, p.height-pane.ChromeLines-2)
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
	lines = append(lines, "", pane.StatusLine(spin, p.width, p.err, p.saving, "Saving…", p.hint()))
	return tea.NewView(popup.Stack(p.height, []string{title, ""}, lines, footer))
}

// refreshMsg makes the form redraw its content, which it only does on updates.
type refreshMsg struct{}

// growText gives a text area being edited most of the form's height.
func (p *Model) growText() {
	if t, ok := p.editing.input.(*huh.Text); ok {
		t.Lines(max(minTextLines, p.formH-3))
		p.form.Update(refreshMsg{})
	}
}

func (p *Model) hint() string {
	mode := helpList
	switch {
	case p.form != nil && p.editing.isUser():
		mode = helpUserField
	case p.form != nil:
		mode = helpField
	case p.list.SettingFilter():
		mode = helpFilter
	}
	km := p.keys.withMode(mode)
	if mode == helpField {
		km = km.withForm(p.form.KeyBinds())
	}
	return p.help.View(km)
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
