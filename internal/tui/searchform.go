package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

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
	fieldResults     = "results"
)

var (
	modeKey        = key.NewBinding(key.WithKeys("ctrl+t"), key.WithHelp("ctrl+t", "basic/advanced"))
	formSearchHelp = key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "search"))
	jqlSearchHelp  = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "search"))
	resultViewHelp = key.NewBinding(key.WithKeys("enter", "v"), key.WithHelp("enter/v", "view"))
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

// searchForm owns one huh form, including its dynamically created result select.
type searchForm struct {
	values   searchValues
	jql      string
	advanced bool
	now      func() time.Time

	form        *huh.Form
	project     *huh.Select[string]
	issueType   *huh.Select[string]
	preview     *huh.Note
	previewText string
	fields      []huh.Field
	projects    []jira.Project
	issueTypes  []string
	width       int
	height      int
}

func newSearchForm(project string) *searchForm {
	f := &searchForm{
		values: searchValues{project: project, sort: jira.SortUpdated, descending: true},
		now:    time.Now,
		width:  80,
		height: 24,
	}
	f.build(nil, "", "")
	return f
}

// build keeps the entered values but replaces the fields when mode or results change.
func (f *searchForm) build(results *resultList, message, focus string) tea.Cmd {
	if f.form != nil {
		f.form.GetFocusedField().Blur()
	}
	f.fields = nil
	f.preview = nil
	if f.advanced {
		f.fields = append(f.fields, huh.NewText().Key(fieldJQL).Title("JQL").
			Placeholder("project = DEMO AND status != Done ORDER BY updated DESC").
			CharLimit(0).ShowLineNumbers(false).Lines(5).Value(&f.jql))
	} else {
		f.basicFields()
		f.preview = huh.NewNote().Title("JQL")
		f.updatePreview()
		f.fields = append(f.fields, f.preview)
	}
	if results != nil && len(results.issues) > 0 {
		results.resize(f.width, f.height)
		f.fields = append(f.fields, results.selector)
	}
	if message != "" {
		f.fields = append(f.fields, huh.NewNote().Title("Results").Description(message))
	}
	f.form = newForm(f.fields...)
	f.resize(f.width, f.height)
	cmd := f.form.Init()
	if focus != "" && focus != f.focused() {
		cmd = tea.Batch(cmd, f.focus(focus))
	}
	return cmd
}

func (f *searchForm) basicFields() {
	v := &f.values
	f.project = huh.NewSelect[string]().Key(fieldProject).Title("Project").
		Inline(true).Options(f.projectOptions()...).Value(&v.project)
	f.issueType = huh.NewSelect[string]().Key(fieldType).Title("Issue type").
		Inline(true).Options(f.typeOptions()...).Value(&v.issueType)
	sortOptions := make([]huh.Option[jira.SortField], len(jira.SortFields))
	for i, s := range jira.SortFields {
		sortOptions[i] = huh.NewOption(s.String(), s)
	}
	f.fields = append(f.fields,
		f.project,
		f.issueType,
		huh.NewSelect[jira.SprintFilter]().Key(fieldSprint).Title("Sprint").Inline(true).
			Options(
				huh.NewOption(anyOption, jira.SprintAny),
				huh.NewOption("In an active sprint", jira.SprintActive),
				huh.NewOption("Not in an active sprint", jira.SprintNotActive),
			).Value(&v.sprint),
		huh.NewSelect[jira.SortField]().Key(fieldSort).Title("Sort by").Inline(true).
			Options(sortOptions...).Value(&v.sort),
		huh.NewSelect[bool]().Key(fieldOrder).Title("Order").Inline(true).
			Options(huh.NewOption("Ascending", false), huh.NewOption("Descending", true)).Value(&v.descending),
		huh.NewInput().Key(fieldKey).Title("Issue key").Placeholder("DEMO-1, DEMO-2").Value(&v.keys),
		huh.NewInput().Key(fieldCreatedFrom).Title("Created from").Placeholder("YYYY-MM-DD or -2d").Value(&v.createdFrom),
		huh.NewInput().Key(fieldCreatedTo).Title("Created to").Placeholder("YYYY-MM-DD or -2d").Value(&v.createdTo),
		huh.NewInput().Key(fieldUpdatedFrom).Title("Updated from").Placeholder("YYYY-MM-DD or -2d").Value(&v.updatedFrom),
		huh.NewInput().Key(fieldUpdatedTo).Title("Updated to").Placeholder("YYYY-MM-DD or -2d").Value(&v.updatedTo),
	)
}

func (f *searchForm) projectOptions() []huh.Option[string] {
	opts := []huh.Option[string]{huh.NewOption(anyOption, "")}
	found := f.values.project == ""
	for _, p := range f.projects {
		opts = append(opts, huh.NewOption(p.Key, p.Key))
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

// focus moves through huh's own navigation, keeping its group viewport in sync.
func (f *searchForm) focus(target string) tea.Cmd {
	current, dest := -1, -1
	for i, field := range f.fields {
		if field.GetKey() == f.focused() {
			current = i
		}
		if field.GetKey() == target {
			dest = i
		}
	}
	var cmd tea.Cmd
	for current >= 0 && dest >= 0 && current != dest {
		if current < dest {
			cmd = f.form.NextField()
			current++
		} else {
			cmd = f.form.PrevField()
			current--
		}
		// Notes are skipped by huh.
		for current >= 0 && current < len(f.fields) && f.fields[current].Skip() {
			if current < dest {
				current++
			} else {
				current--
			}
		}
	}
	return cmd
}

func (f *searchForm) restyle() { f.form.WithTheme(styles.FormTheme()) }

func (f *searchForm) resize(width, height int) {
	// huh subtracts borders and prompts from these sizes; clip smaller windows in render.
	f.width, f.height = max(16, width), max(4, height)
	f.form.WithWidth(f.width).WithHeight(f.height)
}

func (f *searchForm) update(msg tea.Msg) tea.Cmd {
	_, cmd := f.form.Update(msg)
	if f.preview != nil && f.updatePreview() {
		_, refresh := f.form.Update(nil)
		return tea.Batch(cmd, refresh)
	}
	return cmd
}

func (f *searchForm) updatePreview() bool {
	q, err := f.query(f.now())
	text := q.String()
	if err != nil {
		text = "Error: " + err.Error()
	}
	changed := text != f.previewText
	f.previewText = text
	f.preview.Description(text)
	return changed
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

func searchTime(label, text string, now time.Time, end bool) (time.Time, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return time.Time{}, nil
	}
	t, err := parseTimeText(text, now, false)
	if err != nil {
		return t, fmt.Errorf("%s: %w", label, err)
	}
	if _, dateErr := time.Parse(time.DateOnly, text); end && dateErr == nil {
		t = t.AddDate(0, 0, 1).Add(-time.Minute)
	}
	return t, nil
}
