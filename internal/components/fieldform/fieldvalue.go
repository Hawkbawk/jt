package fieldform

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/huh/v2"

	"github.com/hawkbawk/jt/internal/components/usersearch"
	"github.com/hawkbawk/jt/internal/jira"
)

// fieldValue holds what a form field is bound to, next to the field as it
// loaded.
type fieldValue struct {
	jira.IssueField
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

func newFieldValue(f jira.IssueField, search usersearch.Func) *fieldValue {
	v := &fieldValue{IssueField: f}
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
func (v *fieldValue) current() jira.IssueField {
	return v.with(v.entered())
}

// with returns the field holding the values in e.
func (v *fieldValue) with(e entered) jira.IssueField {
	f := v.IssueField
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
func (v *fieldValue) differs(f jira.IssueField) bool {
	return f.Text != v.Text || !sameSet(f.Chosen, v.Chosen)
}

// empty reports whether f holds no value.
func empty(f jira.IssueField) bool {
	return strings.TrimSpace(f.Text) == "" && len(f.Chosen) == 0
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

// listHeight is the height of a select with n options and its title.
func listHeight(n int) int {
	return min(n, 6) + 1
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

// Plural returns word, in its plural form unless n is one.
func Plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// Changes describes n unsaved changes.
func Changes(n int) string {
	return strconv.Itoa(n) + " unsaved " + Plural(n, "change")
}
