package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/styles"
)

const (
	anyOption = "Any"

	// labelWidth fits the longest field label.
	labelWidth = 13
	// minCellWidth keeps a column readable on narrow terminals.
	minCellWidth = 30
)

var (
	sprintOptions = []string{anyOption, "In an active sprint", "Not in an active sprint"}
	orderOptions  = []string{"Ascending", "Descending"}
)

var (
	formNextKey  = key.NewBinding(key.WithKeys("tab", "down"))
	formPrevKey  = key.NewBinding(key.WithKeys("shift+tab", "up"))
	formLeftKey  = key.NewBinding(key.WithKeys("left"))
	formRightKey = key.NewBinding(key.WithKeys("right"))
	formClearKey = key.NewBinding(key.WithKeys("backspace", "delete"))

	// The help-only bindings carry keys since help hides those without.
	formMoveHelp   = key.NewBinding(key.WithKeys("tab", "shift+tab"), key.WithHelp("tab/shift+tab", "next/previous field"))
	formChooseHelp = key.NewBinding(key.WithKeys("left", "right"), key.WithHelp("←/→", "change option"))
	formPickHelp   = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "pick project"))
)

// Indexes of the form's fields, in focus order. The first five are drawn in the
// left column and the rest in the right.
const (
	fieldProject = iota
	fieldType
	fieldSprint
	fieldSort
	fieldOrder
	fieldKey
	fieldCreatedFrom
	fieldCreatedTo
	fieldUpdatedFrom
	fieldUpdatedTo
	fieldCount
)

const leftColumn = fieldKey

// searchField is one control of the search form: a text box, or a value picked
// from options. The project field's options are chosen in the project picker.
type searchField struct {
	label       string
	input       textinput.Model
	options     []string
	index       int
	isText      bool
	placeholder string
}

func newTextField(label, placeholder string) *searchField {
	in := textinput.New()
	in.Placeholder = placeholder
	in.Prompt = ""
	in.SetStyles(styles.TextInput())
	return &searchField{label: label, input: in, isText: true}
}

func newChoiceField(label string, options ...string) *searchField {
	return &searchField{label: label, options: options}
}

func (f *searchField) value() string {
	if f.isText {
		return strings.TrimSpace(f.input.Value())
	}
	return f.options[f.index]
}

// cycle moves to the next or previous option, wrapping around.
func (f *searchField) cycle(delta int) {
	f.index = (f.index + delta + len(f.options)) % len(f.options)
}

// searchForm is the basic search mode's form.
type searchForm struct {
	fields  [fieldCount]*searchField
	project string
	focus   int
}

func newSearchForm(project string) *searchForm {
	f := &searchForm{project: project}
	f.fields = [fieldCount]*searchField{
		fieldProject:     {label: "Project"},
		fieldType:        newChoiceField("Issue type", anyOption),
		fieldSprint:      newChoiceField("Sprint", sprintOptions...),
		fieldSort:        newChoiceField("Sort by", sortLabels()...),
		fieldOrder:       newChoiceField("Order", orderOptions...),
		fieldKey:         newTextField("Issue key", "DEMO-1, DEMO-2"),
		fieldCreatedFrom: newTextField("Created from", "2006-01-02 or -2d"),
		fieldCreatedTo:   newTextField("Created to", "2006-01-02 or -2d"),
		fieldUpdatedFrom: newTextField("Updated from", "2006-01-02 or -2d"),
		fieldUpdatedTo:   newTextField("Updated to", "2006-01-02 or -2d"),
	}
	// New searches show what changed most recently first.
	f.fields[fieldSort].index = 1
	f.fields[fieldOrder].index = 1
	return f
}

func sortLabels() []string {
	labels := make([]string, len(jira.SortFields))
	for i, s := range jira.SortFields {
		labels[i] = s.Label()
	}
	return labels
}

// setIssueTypes offers types after "Any", keeping the chosen one if it's still
// there.
func (f *searchForm) setIssueTypes(types []string) {
	field := f.fields[fieldType]
	chosen := field.value()
	field.options = append([]string{anyOption}, types...)
	field.index = 0
	for i, o := range field.options {
		if o == chosen {
			field.index = i
		}
	}
}

// setProject restricts the search to project, or lifts the restriction when
// it's empty.
func (f *searchForm) setProject(project string) {
	f.project = project
}

// restyle redraws the text boxes in the current theme.
func (f *searchForm) restyle() {
	for _, fld := range f.fields {
		if fld.isText {
			fld.input.SetStyles(styles.TextInput())
		}
	}
}

// focusCmd focuses the current field and returns the command that starts its
// cursor.
func (f *searchForm) focusCmd() tea.Cmd {
	for i, fld := range f.fields {
		if fld.isText && i != f.focus {
			fld.input.Blur()
		}
	}
	if fld := f.fields[f.focus]; fld.isText {
		return fld.input.Focus()
	}
	return nil
}

func (f *searchForm) move(delta int) tea.Cmd {
	f.focus = (f.focus + delta + fieldCount) % fieldCount
	return f.focusCmd()
}

// openProjectsMsg asks for the project picker.
type openProjectsMsg struct{}

// update applies a message to the focused field.
func (f *searchForm) update(msg tea.Msg) tea.Cmd {
	fld := f.fields[f.focus]
	km, isKey := msg.(tea.KeyPressMsg)
	switch {
	case isKey && key.Matches(km, formNextKey):
		return f.move(1)
	case isKey && key.Matches(km, formPrevKey):
		return f.move(-1)
	case f.focus == fieldProject && isKey:
		switch {
		case key.Matches(km, keys.accept):
			return func() tea.Msg { return openProjectsMsg{} }
		case key.Matches(km, formClearKey):
			f.project = ""
		}
		return nil
	case !fld.isText && isKey:
		switch {
		case key.Matches(km, formLeftKey):
			fld.cycle(-1)
		case key.Matches(km, formRightKey, keys.accept):
			fld.cycle(1)
		}
		return nil
	case fld.isText:
		var cmd tea.Cmd
		fld.input, cmd = fld.input.Update(msg)
		return cmd
	}
	return nil
}

// query reads the form. The error names the field that can't be read.
func (f *searchForm) query(now time.Time) (jira.SearchQuery, error) {
	q := jira.SearchQuery{Project: f.project, Sort: jira.SortFields[f.fields[fieldSort].index]}
	q.Descending = f.fields[fieldOrder].index == 1
	q.Sprint = jira.SprintFilter(f.fields[fieldSprint].index)
	if f.fields[fieldType].index > 0 {
		q.IssueType = f.fields[fieldType].value()
	}

	var err error
	if q.Keys, err = jira.ParseKeys(f.fields[fieldKey].value()); err != nil {
		return q, fmt.Errorf("%s: %w", f.fields[fieldKey].label, err)
	}
	for _, r := range []struct {
		from, to int
		dst      *jira.TimeRange
	}{
		{fieldCreatedFrom, fieldCreatedTo, &q.Created},
		{fieldUpdatedFrom, fieldUpdatedTo, &q.Updated},
	} {
		if r.dst.From, err = f.parseTime(r.from, now, false); err != nil {
			return q, err
		}
		if r.dst.To, err = f.parseTime(r.to, now, true); err != nil {
			return q, err
		}
	}
	return q, nil
}

// parseTime reads a date field. A date without a time as the end of a range
// stands for the end of that day.
func (f *searchForm) parseTime(field int, now time.Time, end bool) (time.Time, error) {
	fld := f.fields[field]
	text := fld.value()
	if text == "" {
		return time.Time{}, nil
	}
	t, err := parseTimeText(text, now, false)
	if err != nil {
		return t, fmt.Errorf("%s: %w", fld.label, err)
	}
	if _, dateErr := time.Parse(time.DateOnly, text); end && dateErr == nil {
		t = t.AddDate(0, 0, 1).Add(-time.Minute)
	}
	return t, nil
}

// view draws the fields in two columns that fit width.
func (f *searchForm) view(width int) string {
	cell := max(minCellWidth, (width-2)/2)
	var left, right []string
	for i := range f.fields {
		line := f.cell(i, cell)
		if i < leftColumn {
			left = append(left, line)
		} else {
			right = append(right, line)
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(cell+2).Render(strings.Join(left, "\n")),
		strings.Join(right, "\n"),
	)
}

func (f *searchForm) cell(i, width int) string {
	fld := f.fields[i]
	focused := i == f.focus
	label := styles.Dim().Width(labelWidth).Render(fld.label)
	if focused {
		label = lipgloss.NewStyle().Foreground(styles.Highlight()).Bold(true).Width(labelWidth).Render(fld.label)
	}
	room := max(1, width-labelWidth-1)

	var value string
	switch {
	case fld.isText:
		fld.input.SetWidth(room - 1)
		value = fld.input.View()
	case i == fieldProject:
		value = f.projectValue(focused)
	default:
		value = fld.options[fld.index]
		if focused {
			value = "‹ " + value + " ›"
		}
	}
	if !fld.isText && !focused {
		value = styles.Dim().Render(value)
	}
	return label + " " + ansi.Truncate(value, room, "…")
}

func (f *searchForm) projectValue(focused bool) string {
	switch {
	case f.project == "" && focused:
		return anyOption + styles.Dim().Render("  (enter to pick)")
	case f.project == "":
		return anyOption
	case focused:
		return f.project + styles.Dim().Render("  (enter to change, backspace to clear)")
	}
	return f.project
}
