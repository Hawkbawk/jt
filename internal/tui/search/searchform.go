package search

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/reltime"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/widget"
)

const anyOption = "Any"

const (
	fieldProject = "project"
	fieldType    = "type"
	fieldSprint  = "sprint"
	fieldSort    = "sort"
	fieldOrder   = "order"
	fieldKey     = "key"
	fieldCreated = "created"
	fieldUpdated = "updated"
	fieldJQL     = "jql"
)

// maxFormHeight is the form height the columns need, with a blank line between
// fields and room for an error footer.
const maxFormHeight = 14

type searchValues struct {
	project, issueType string
	sprint             jira.SprintFilter
	sort               jira.SortField
	descending         bool
	keys               string
	created, updated   string
}

// searchForm owns one huh form, laid out in two columns in basic mode.
type searchForm struct {
	values   searchValues
	jql      string
	advanced bool
	now      func() time.Time

	form       *huh.Form
	project    *huh.Input
	issueType  *huh.Input
	fields     []huh.Field
	projects   []jira.Project
	issueTypes []string
	width      int
	height     int
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
		f.form = widget.NewForm(f.fields...)
	} else {
		left, right := f.basicColumns()
		f.fields = append(append([]huh.Field{}, left...), right...)
		f.form = widget.NewGroupsForm(huh.NewGroup(left...), huh.NewGroup(right...)).
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
		_, err := parseTimeRange(s, f.now())
		return err
	}).Placeholder("-2d or 2026-01-01..2026-02-01")
}

// basicColumns splits the basic fields into two columns of similar height.
func (f *searchForm) basicColumns() (left, right []huh.Field) {
	v := &f.values
	keys := f.text(fieldKey, "Issue key", &v.keys, func(s string) error {
		_, err := jira.ParseKeys(s)
		return err
	}).Placeholder("DEMO-1, DEMO-2")
	f.project = f.text(fieldProject, "Project", &v.project, func(s string) error {
		if _, ok := f.resolveProject(s); !ok {
			return errors.New("not a project you can see")
		}
		return nil
	}).Placeholder(anyOption).Suggestions(f.projectSuggestions())
	f.issueType = f.text(fieldType, "Issue type", &v.issueType, func(s string) error {
		if _, ok := f.resolveType(s); !ok {
			return errors.New("not an issue type you can use")
		}
		return nil
	}).Placeholder(anyOption).Suggestions(f.typeSuggestions())
	sortOptions := make([]huh.Option[jira.SortField], len(jira.SortFields))
	for i, s := range jira.SortFields {
		sortOptions[i] = huh.NewOption(s.String(), s)
	}
	left = []huh.Field{
		keys,
		f.project,
		f.issueType,
		huh.NewSelect[jira.SprintFilter]().Key(fieldSprint).Title("Sprint").
			Options(
				huh.NewOption(anyOption, jira.SprintAny),
				huh.NewOption("In an active sprint", jira.SprintActive),
				huh.NewOption("Not in an active sprint", jira.SprintNotActive),
			).Inline(true).Value(&v.sprint),
	}
	right = []huh.Field{
		huh.NewSelect[bool]().Key(fieldOrder).Title("Order").
			Options(huh.NewOption("Ascending", false), huh.NewOption("Descending", true)).
			Inline(true).Value(&v.descending),
		huh.NewSelect[jira.SortField]().Key(fieldSort).Title("Sort by").
			Options(sortOptions...).Inline(true).Value(&v.sort),
		f.timeInput(fieldCreated, "Created", &v.created),
		f.timeInput(fieldUpdated, "Updated", &v.updated),
	}
	return left, right
}

// resolveProject maps text naming a project, by key or name, to its key. Empty
// text means any project. Until the projects load, text is taken as a key.
func (f *searchForm) resolveProject(text string) (string, bool) {
	text = strings.TrimSpace(text)
	if text == "" || len(f.projects) == 0 {
		return text, true
	}
	for _, p := range f.projects {
		if strings.EqualFold(text, p.Key) || strings.EqualFold(text, p.Name) {
			return p.Key, true
		}
	}
	return "", false
}

// resolveType maps text to the issue type it names, ignoring case. Empty text
// means any type. Until the types load, text is taken as is.
func (f *searchForm) resolveType(text string) (string, bool) {
	text = strings.TrimSpace(text)
	if text == "" || len(f.issueTypes) == 0 {
		return text, true
	}
	for _, t := range f.issueTypes {
		if strings.EqualFold(text, t) {
			return t, true
		}
	}
	return "", false
}

func (f *searchForm) projectSuggestions() []string {
	var out []string
	for _, p := range f.projects {
		out = append(out, p.Key)
		if p.Name != "" {
			out = append(out, p.Name)
		}
	}
	return out
}

func (f *searchForm) typeSuggestions() []string { return f.issueTypes }

func (f *searchForm) setProjects(projects []jira.Project) {
	f.projects = projects
	if f.project != nil {
		f.project.Suggestions(f.projectSuggestions())
	}
}

func (f *searchForm) setIssueTypes(types []string) {
	f.issueTypes = types
	if f.issueType != nil {
		f.issueType.Suggestions(f.typeSuggestions())
	}
}

func (f *searchForm) focused() string { return f.form.GetFocusedField().GetKey() }

func (f *searchForm) onLastField() bool {
	return f.form.GetFocusedField() == f.fields[len(f.fields)-1]
}

func (f *searchForm) restyle() {
	base := styles.FormTheme()
	f.form.WithTheme(huh.ThemeFunc(func(dark bool) *huh.Styles {
		s := base.Theme(dark)
		s.Group.Base = s.Group.Base.PaddingTop(1)
		return s
	}))
}

func (f *searchForm) resize(width, height int) {
	// huh subtracts borders and prompts from these sizes; clip smaller windows in render.
	f.width, f.height = max(16, width), max(4, height)
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
	q := jira.SearchQuery{Sprint: v.sprint, Sort: v.sort, Descending: v.descending}
	var ok bool
	if q.Project, ok = f.resolveProject(v.project); !ok {
		return q, fmt.Errorf("project %q isn't a project you can see", v.project)
	}
	if q.IssueType, ok = f.resolveType(v.issueType); !ok {
		return q, fmt.Errorf("Issue type: %q isn't an issue type you can use", v.issueType) //nolint:staticcheck // ST1005: "Issue" starts the message as shown in the UI.
	}
	var err error
	if q.Keys, err = jira.ParseKeys(v.keys); err != nil {
		return q, fmt.Errorf("Issue key: %w", err) //nolint:staticcheck // ST1005: "Issue" starts the message as shown in the UI.
	}
	if q.Created, err = parseTimeRange(v.created, now); err != nil {
		return q, fmt.Errorf("created range: %w", err)
	}
	if q.Updated, err = parseTimeRange(v.updated, now); err != nil {
		return q, fmt.Errorf("updated range: %w", err)
	}
	return q, nil
}

// parseTimeRange reads a range such as 2026-01-01..2026-02-01. Either end can
// be left out to leave it open. A single time runs from then until now, or
// from now until then when it's in the future.
func parseTimeRange(text string, now time.Time) (jira.TimeRange, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return jira.TimeRange{}, nil
	}
	from, to, isRange := strings.Cut(text, "..")
	if !isRange {
		t, err := parseBound(text, now, false)
		if err != nil {
			return jira.TimeRange{}, err
		}
		if !t.After(now) {
			return jira.TimeRange{From: t}, nil
		}
		end, _ := parseBound(text, now, true)
		return jira.TimeRange{From: now, To: end}, nil
	}
	var r jira.TimeRange
	var err error
	if from = strings.TrimSpace(from); from != "" {
		if r.From, err = parseBound(from, now, false); err != nil {
			return r, err
		}
	}
	if to = strings.TrimSpace(to); to != "" {
		if r.To, err = parseBound(to, now, true); err != nil {
			return r, err
		}
	}
	if !r.From.IsZero() && !r.To.IsZero() && r.From.After(r.To) {
		return r, errors.New("the range ends before it starts")
	}
	return r, nil
}

// parseBound reads one end of a range. The end of a range given as a date
// covers that whole day.
func parseBound(text string, now time.Time, end bool) (time.Time, error) {
	t, err := reltime.ParseTime(text, now, false)
	if err != nil {
		return t, err
	}
	if _, dateErr := time.Parse(time.DateOnly, text); end && dateErr == nil {
		t = t.AddDate(0, 0, 1).Add(-time.Minute)
	}
	return t, nil
}
