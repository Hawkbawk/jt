package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/styles"
)

const anyOption = "Any"

const (
	fieldProject     = "project"
	fieldType        = "type"
	fieldSprint      = "sprint"
	fieldSort        = "sort"
	fieldOrder       = "order"
	fieldKey         = "key"
	fieldCreatedFrom = "created-from"
	fieldCreatedTo   = "created-to"
	fieldUpdatedFrom = "updated-from"
	fieldUpdatedTo   = "updated-to"
	fieldJQL         = "jql"
)

var (
	modeKey        = key.NewBinding(key.WithKeys("ctrl+t"), key.WithHelp("ctrl+t", "basic/advanced"))
	formSearchHelp = key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "search"))
	jqlSearchHelp  = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "search"))
	resultViewHelp = key.NewBinding(key.WithKeys("enter", "v"), key.WithHelp("enter/v", "view"))
)

const (
	// selectHeight is how many lines a select shows, including its title.
	selectHeight = 4
	// roomyHeight is the form height the columns need with a blank line between
	// fields. Shorter forms drop the blank lines.
	roomyHeight = 20
)

type searchValues struct {
	project, issueType string
	sprint             jira.SprintFilter
	sort               jira.SortField
	descending         bool
	keys               string
	createdFrom        string
	createdTo          string
	updatedFrom        string
	updatedTo          string
}

// searchForm owns one huh form, laid out in two columns in basic mode.
type searchForm struct {
	values   searchValues
	jql      string
	advanced bool
	now      func() time.Time

	form       *huh.Form
	project    *huh.Select[string]
	issueType  *huh.Select[string]
	fields     []huh.Field
	projects   []jira.Project
	issueTypes []string
	width      int
	height     int
	dense      bool
}

func newSearchForm(project string) *searchForm {
	f := &searchForm{
		values: searchValues{project: project, sort: jira.SortUpdated, descending: true},
		now:    time.Now,
		width:  80,
		height: 24,
	}
	f.build()
	return f
}

// build keeps the entered values but replaces the fields when the mode changes.
func (f *searchForm) build() tea.Cmd {
	if f.form != nil {
		f.form.GetFocusedField().Blur()
	}
	f.fields = nil
	if f.advanced {
		jql := huh.NewText().Key(fieldJQL).Title("JQL").
			Placeholder("project = DEMO AND status != Done ORDER BY updated DESC").
			CharLimit(0).ShowLineNumbers(false).Lines(5).Value(&f.jql).Validate(requireJQL)
		f.fields = []huh.Field{jql}
		f.form = newForm(f.fields...)
	} else {
		left, right := f.basicColumns()
		f.fields = append(append([]huh.Field{}, left...), right...)
		f.form = newGroupsForm(huh.NewGroup(left...), huh.NewGroup(right...)).
			WithLayout(huh.LayoutColumns(2))
	}
	f.restyle()
	f.resize(f.width, f.height)
	return f.form.Init()
}

func requireJQL(s string) error {
	if strings.TrimSpace(s) == "" {
		return errors.New("write some JQL to search with")
	}
	return nil
}

func (f *searchForm) text(field, title string, value *string, validate func(string) error) *huh.Input {
	return huh.NewInput().Key(field).Title(title).Value(value).Validate(validate)
}

func (f *searchForm) timeInput(field, title string, value *string) *huh.Input {
	return f.text(field, title, value, func(s string) error {
		_, err := parseSearchTime(s, f.now())
		return err
	}).Placeholder("YYYY-MM-DD or -2d")
}

// basicColumns splits the basic fields into two columns of similar height.
func (f *searchForm) basicColumns() (left, right []huh.Field) {
	v := &f.values
	f.project = huh.NewSelect[string]().Key(fieldProject).Title("Project").
		Options(f.projectOptions()...).Height(selectHeight).Value(&v.project)
	f.issueType = huh.NewSelect[string]().Key(fieldType).Title("Issue type").
		Options(f.typeOptions()...).Height(selectHeight).Value(&v.issueType)
	sortOptions := make([]huh.Option[jira.SortField], len(jira.SortFields))
	for i, s := range jira.SortFields {
		sortOptions[i] = huh.NewOption(s.String(), s)
	}
	keys := f.text(fieldKey, "Issue key", &v.keys, func(s string) error {
		_, err := jira.ParseKeys(s)
		return err
	}).Placeholder("DEMO-1, DEMO-2")
	left = []huh.Field{
		f.project,
		f.issueType,
		keys,
		f.timeInput(fieldCreatedFrom, "Created from", &v.createdFrom),
		f.timeInput(fieldCreatedTo, "Created to", &v.createdTo),
	}
	right = []huh.Field{
		huh.NewSelect[jira.SprintFilter]().Key(fieldSprint).Title("Sprint").
			Options(
				huh.NewOption(anyOption, jira.SprintAny),
				huh.NewOption("In an active sprint", jira.SprintActive),
				huh.NewOption("Not in an active sprint", jira.SprintNotActive),
			).Height(selectHeight).Value(&v.sprint),
		huh.NewSelect[jira.SortField]().Key(fieldSort).Title("Sort by").
			Options(sortOptions...).Height(selectHeight).Value(&v.sort),
		huh.NewSelect[bool]().Key(fieldOrder).Title("Order").
			Options(huh.NewOption("Ascending", false), huh.NewOption("Descending", true)).
			Height(3).Value(&v.descending),
		f.timeInput(fieldUpdatedFrom, "Updated from", &v.updatedFrom),
		f.timeInput(fieldUpdatedTo, "Updated to", &v.updatedTo),
	}
	return left, right
}

func (f *searchForm) projectOptions() []huh.Option[string] {
	opts := []huh.Option[string]{huh.NewOption(anyOption, "")}
	found := f.values.project == ""
	for _, p := range f.projects {
		label := p.Key
		if p.Name != "" {
			label += " · " + p.Name
		}
		opts = append(opts, huh.NewOption(label, p.Key))
		found = found || p.Key == f.values.project
	}
	if !found {
		opts = append(opts, huh.NewOption(f.values.project, f.values.project))
	}
	return opts
}

func (f *searchForm) typeOptions() []huh.Option[string] {
	opts := []huh.Option[string]{huh.NewOption(anyOption, "")}
	for _, t := range f.issueTypes {
		opts = append(opts, huh.NewOption(t, t))
	}
	return opts
}

func (f *searchForm) setProjects(projects []jira.Project) {
	f.projects = projects
	if f.project != nil {
		f.project.Options(f.projectOptions()...)
	}
}

func (f *searchForm) setIssueTypes(types []string) {
	f.issueTypes = types
	if f.issueType != nil {
		f.issueType.Options(f.typeOptions()...)
	}
}

func (f *searchForm) focused() string { return f.form.GetFocusedField().GetKey() }

func (f *searchForm) restyle() { f.form.WithTheme(f.theme()) }

func (f *searchForm) theme() huh.Theme {
	base := styles.FormTheme()
	return huh.ThemeFunc(func(dark bool) *huh.Styles {
		s := base.Theme(dark)
		if f.dense {
			s.FieldSeparator = lipgloss.NewStyle().SetString("\n")
		}
		return s
	})
}

func (f *searchForm) resize(width, height int) {
	// huh subtracts borders and prompts from these sizes; clip smaller windows in render.
	f.width, f.height = max(16, width), max(4, height)
	if dense := f.height < roomyHeight; dense != f.dense {
		f.dense = dense
		f.restyle()
	}
	f.form.WithWidth(f.width)
	if !f.advanced {
		f.form.WithHeight(f.height)
	}
}

func (f *searchForm) update(msg tea.Msg) tea.Cmd {
	_, cmd := f.form.Update(msg)
	return cmd
}

func (f *searchForm) query(now time.Time) (jira.SearchQuery, error) {
	v := f.values
	q := jira.SearchQuery{
		Project: v.project, IssueType: v.issueType, Sprint: v.sprint,
		Sort: v.sort, Descending: v.descending,
	}
	var err error
	if q.Keys, err = jira.ParseKeys(v.keys); err != nil {
		return q, fmt.Errorf("Issue key: %w", err)
	}
	for _, r := range []struct {
		label, from, to string
		dst             *jira.TimeRange
	}{
		{"Created", v.createdFrom, v.createdTo, &q.Created},
		{"Updated", v.updatedFrom, v.updatedTo, &q.Updated},
	} {
		if r.dst.From, err = searchTime(r.label+" from", r.from, now, false); err != nil {
			return q, err
		}
		if r.dst.To, err = searchTime(r.label+" to", r.to, now, true); err != nil {
			return q, err
		}
	}
	return q, nil
}

// parseSearchTime reads a bound of a time range; empty text means no bound.
func parseSearchTime(text string, now time.Time) (time.Time, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return time.Time{}, nil
	}
	return parseTimeText(text, now, false)
}

func searchTime(label, text string, now time.Time, end bool) (time.Time, error) {
	t, err := parseSearchTime(text, now)
	if err != nil {
		return t, fmt.Errorf("%s: %w", label, err)
	}
	if _, dateErr := time.Parse(time.DateOnly, strings.TrimSpace(text)); end && dateErr == nil {
		t = t.AddDate(0, 0, 1).Add(-time.Minute)
	}
	return t, nil
}
