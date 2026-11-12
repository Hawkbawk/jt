// Package issuecreate is the modal that creates an issue. It first asks for the
// project and issue type, then lists the fields that type can be created with
// and edits them one at a time. When the issue is created it closes with a
// Created message.
package issuecreate

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/hawkbawk/jt/internal/components/fieldform"
	"github.com/hawkbawk/jt/internal/components/pane"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/keys"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
)

// Backend loads what an issue can be created with and creates it.
type Backend interface {
	Projects() ([]jira.Project, error)
	CreateIssueTypes(project string) ([]jira.IssueType, error)
	CreateFields(project, issueTypeID string) ([]jira.IssueField, error)
	Create(project, issueTypeID string, fields []jira.IssueField) (string, error)
	SearchProjectUsers(project, query string) ([]jira.Option, error)
}

// Created is what the modal closes with after creating an issue. It's a
// tuimsg.Finished.
type Created struct {
	Key string
}

// ForParent marks Created for the modal host.
func (Created) ForParent() {}

// ClosesModal marks Created for the modal host.
func (Created) ClosesModal() {}

var _ tuimsg.Finished = Created{}

// IssueKey implements tuimsg.Finished.
func (c Created) IssueKey() string { return c.Key }

// Status implements tuimsg.Finished.
func (c Created) Status() string { return "Created " + c.Key }

type setupLoadedMsg struct {
	projects []jira.Project
	types    []jira.IssueType
	err      error
}

type typesLoadedMsg struct {
	project string
	types   []jira.IssueType
	err     error
}

type fieldsLoadedMsg struct {
	project, issueType string
	fields             []jira.IssueField
	err                error
}

type createdMsg struct {
	key string
	err error
}

type stage int

const (
	stageSetup stage = iota
	stageFields
)

type confirmKind int

const (
	confirmNone confirmKind = iota
	confirmClose
	confirmBack
)

// Model asks for a project and issue type, then lists the fields to create the
// issue with. Values entered are kept until the issue is created.
type Model struct {
	keys    fieldform.KeyMap
	back    key.Binding
	backend Backend
	initial string
	spin    spinner.Model
	help    help.Model

	width, height int
	stage         stage
	confirm       confirmKind
	err           error

	// loadingSetup is true until the projects and the first project's issue
	// types arrive.
	loadingSetup bool
	loadingTypes bool
	loadingForm  bool
	saving       bool

	projects []jira.Project
	types    []jira.IssueType
	// typesFor is the project the issue types listed are for.
	typesFor string
	project  jira.Project
	itype    jira.IssueType
	typeSel  *huh.Select[jira.IssueType]
	form     *huh.Form

	fields fieldform.Editor
	loaded bool
}

// New returns the modal that creates an issue, starting on project. It loads
// the projects and issue types when it opens.
func New(b Backend, project string) *Model {
	k := keys.Get()
	m := &Model{
		backend: b, initial: project, spin: pane.NewSpinner(), help: styles.Help(),
		keys: fieldform.NewKeyMap(), back: keys.Describe(k.PrevTab, "back"),
		loadingSetup: true,
	}
	m.keys.SaveLabel = "create"
	m.fields.Help("create", m.back)
	return m
}

// Init implements tea.Model.
func (p *Model) Init() tea.Cmd {
	return tea.Batch(p.loadSetup(), p.spin.Tick)
}

func (p *Model) loadSetup() tea.Cmd {
	b, initial := p.backend, p.initial
	return func() tea.Msg {
		ps, err := b.Projects()
		if err != nil {
			return setupLoadedMsg{err: err}
		}
		if len(ps) == 0 {
			return setupLoadedMsg{}
		}
		first := ps[0].Key
		for _, pr := range ps {
			if pr.Key == initial {
				first = initial
			}
		}
		ts, err := b.CreateIssueTypes(first)
		return setupLoadedMsg{projects: ps, types: ts, err: err}
	}
}

func (p *Model) loadTypes(project string) tea.Cmd {
	b := p.backend
	return func() tea.Msg {
		ts, err := b.CreateIssueTypes(project)
		return typesLoadedMsg{project: project, types: ts, err: err}
	}
}

func (p *Model) loadFields() tea.Cmd {
	b, project, t := p.backend, p.project.Key, p.itype.ID
	return func() tea.Msg {
		fs, err := b.CreateFields(project, t)
		return fieldsLoadedMsg{project: project, issueType: t, fields: fs, err: err}
	}
}

func (p *Model) create(fields []jira.IssueField) tea.Cmd {
	b, project, t := p.backend, p.project.Key, p.itype.ID
	return func() tea.Msg {
		k, err := b.Create(project, t, fields)
		return createdMsg{key: k, err: err}
	}
}

func (p *Model) loading() bool {
	return p.loadingSetup || p.loadingTypes || p.loadingForm || p.saving
}

func (p *Model) unsaved() int { return len(p.fields.Changed()) }

// buildForm makes the form that picks the project and issue type, with the
// issue types in p.types listed. focusType puts the cursor on the issue type.
func (p *Model) buildForm(focusType bool) tea.Cmd {
	popts := make([]huh.Option[jira.Project], len(p.projects))
	for i, pr := range p.projects {
		popts[i] = huh.NewOption(pr.Key+" · "+pr.Name, pr)
	}
	topts := make([]huh.Option[jira.IssueType], len(p.types))
	for i, t := range p.types {
		topts[i] = huh.NewOption(t.Name, t)
	}
	p.itype = jira.IssueType{}
	p.typeSel = huh.NewSelect[jira.IssueType]().Title("Issue type").Options(topts...).Height(listHeight(len(topts))).Value(&p.itype)
	projectSel := huh.NewSelect[jira.Project]().Title("Project").Options(popts...).Height(listHeight(len(popts))).Value(&p.project)
	p.form = pane.NewForm(projectSel, p.typeSel)
	cmd := p.form.Init()
	if focusType {
		cmd = tea.Batch(cmd, p.form.NextField())
	}
	return cmd
}

// noTypes is the error for a project that can't create anything, or nil.
func noTypes(project string, types []jira.IssueType) error {
	if len(types) > 0 {
		return nil
	}
	return fmt.Errorf("no issue types can be created in %s", project)
}

// listHeight is the height of a select with n options and its title.
func listHeight(n int) int { return min(max(n, 1), 6) + 1 }

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
	if p.confirm != confirmNone {
		return p.updateConfirm(km)
	}
	if p.stage == stageSetup {
		if key.Matches(km, p.keys.Blur) {
			return tuimsg.Close()
		}
		return p.update(km)
	}
	if p.fields.ConsumesEsc() && key.Matches(km, p.keys.Blur) {
		return p.update(km)
	}
	closes := p.keys.Close
	if p.fields.Capturing() {
		closes = p.keys.Blur
	}
	switch {
	case key.Matches(km, closes):
		if p.unsaved() > 0 {
			p.confirm = confirmClose
			return nil
		}
		return tuimsg.Close()
	case !p.fields.Capturing() && key.Matches(km, p.back):
		if p.unsaved() > 0 {
			p.confirm = confirmBack
			return nil
		}
		return p.goBack()
	}
	return p.update(km)
}

func (p *Model) updateConfirm(km tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(km, p.keys.Confirm):
		was := p.confirm
		p.confirm = confirmNone
		if was == confirmBack {
			return p.goBack()
		}
		return tuimsg.Close()
	case key.Matches(km, p.keys.Deny):
		p.confirm = confirmNone
	case key.Matches(km, p.keys.Send):
		p.confirm = confirmNone
		return p.update(km)
	}
	return nil
}

// goBack returns to picking the project and issue type, dropping the fields.
func (p *Model) goBack() tea.Cmd {
	p.stage, p.loaded, p.err, p.loadingForm = stageSetup, false, nil, false
	p.fields.Load(nil, nil)
	return p.buildForm(true)
}

func (p *Model) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case setupLoadedMsg:
		p.loadingSetup, p.err = false, msg.err
		if msg.err != nil {
			return nil
		}
		if len(msg.projects) == 0 {
			p.err = fmt.Errorf("no projects to create issues in")
			return nil
		}
		p.projects, p.types = msg.projects, msg.types
		p.project = msg.projects[0]
		for _, pr := range msg.projects {
			if pr.Key == p.initial {
				p.project = pr
			}
		}
		p.typesFor = p.project.Key
		p.err = noTypes(p.project.Key, p.types)
		return p.buildForm(false)
	case typesLoadedMsg:
		p.loadingTypes = false
		if msg.err != nil {
			p.err = msg.err
			return nil
		}
		if msg.project != p.project.Key {
			return p.syncTypes()
		}
		p.types, p.typesFor = msg.types, msg.project
		p.err = noTypes(msg.project, msg.types)
		return p.buildForm(true)
	case fieldsLoadedMsg:
		if msg.project != p.project.Key || msg.issueType != p.itype.ID {
			return nil
		}
		p.loadingForm, p.err = false, msg.err
		if msg.err != nil {
			return nil
		}
		p.loaded = true
		project := msg.project
		b := p.backend
		p.fields.Load(msg.fields, func(q string) ([]jira.Option, error) { return b.SearchProjectUsers(project, q) })
		return nil
	case createdMsg:
		p.saving, p.err = false, msg.err
		if msg.err == nil {
			return tuimsg.Send(Created{Key: msg.key})
		}
		return nil
	}
	if p.stage == stageSetup {
		return p.updateSetup(msg)
	}
	if !p.loaded || p.saving || p.fields.Len() == 0 {
		return nil
	}
	if km, ok := msg.(tea.KeyMsg); ok && !p.fields.Holding() && key.Matches(km, p.keys.Send) {
		return p.submit()
	}
	return p.fields.Update(msg)
}

// updateSetup passes msg to the project and issue type form, loading the issue
// types of the project once the cursor reaches them, and the fields once the
// form is done.
func (p *Model) updateSetup(msg tea.Msg) tea.Cmd {
	if p.form == nil || p.loadingSetup {
		return nil
	}
	if km, ok := msg.(tea.KeyMsg); ok && p.loadingTypes && key.Matches(km, p.keys.Accept) {
		return nil
	}
	_, cmd := p.form.Update(msg)
	if p.form.State == huh.StateCompleted {
		if p.itype.ID == "" {
			p.err = noTypes(p.project.Key, nil)
			return tea.Batch(cmd, p.buildForm(true))
		}
		p.err = nil
		p.stage, p.loadingForm = stageFields, true
		return tea.Batch(cmd, p.loadFields())
	}
	return tea.Batch(cmd, p.syncTypes())
}

// syncTypes loads the issue types of the chosen project when the cursor is on
// the issue type and the ones listed are for another project.
func (p *Model) syncTypes() tea.Cmd {
	if p.form == nil || p.loadingTypes || p.project.Key == p.typesFor {
		return nil
	}
	if p.form.GetFocusedField() != huh.Field(p.typeSel) {
		return nil
	}
	p.loadingTypes, p.err = true, nil
	return p.loadTypes(p.project.Key)
}

func (p *Model) submit() tea.Cmd {
	if p.saving {
		return nil
	}
	if missing := p.fields.Missing(); len(missing) > 0 {
		p.err = fmt.Errorf("required: %s", strings.Join(missing, ", "))
		return nil
	}
	p.saving, p.err = true, nil
	return p.create(p.fields.Changed())
}

// Title implements modal.Titled.
func (p *Model) Title() string {
	if p.stage == stageFields {
		return "Create · " + p.project.Key + " · " + p.itype.Name
	}
	return "Create"
}

// Footer implements modal.Footed.
func (p *Model) Footer() (string, help.KeyMap) {
	if n := p.unsaved(); n > 0 && p.confirm != confirmNone {
		return styles.Warn().Render("Discard " + fieldform.Changes(n) + "? "), p.keys.WithMode(fieldform.HelpDiscard)
	}
	return "", p.keys.WithMode(fieldform.HelpPane)
}

// View implements tea.Model.
func (p *Model) View() tea.View {
	p.help.SetWidth(p.width)
	spin := p.spin.View()
	bodyH := max(1, p.height-pane.BodyLines)
	var warn string
	if n := p.unsaved(); n > 0 {
		warn = styles.Warn().Render("● " + fieldform.Changes(n))
		if !p.fields.Holding() {
			warn += styles.Dim().Render(" · ") + p.help.View(p.keys.WithMode(fieldform.HelpSave))
		}
	}
	lines := []string{warn}
	hint := p.help.View(p.keys.WithMode(fieldform.HelpPane))
	switch {
	case p.loadingSetup:
		lines = append(lines, spin+" Loading projects…")
	case p.stage == stageSetup && p.form != nil:
		p.form.WithWidth(p.width).WithHeight(bodyH)
		lines = append(lines, p.form.View())
		if p.loadingTypes {
			lines = append(lines, spin+" Loading issue types…")
		}
	case p.loadingForm:
		lines = append(lines, spin+" Loading fields…")
	case p.loaded && p.fields.Len() == 0:
		lines = append(lines, styles.Dim().Render("No fields to fill in."))
	case p.loaded:
		lines = append(lines, p.fields.View(p.width, bodyH))
		hint = p.fields.Hint(p.help)
	}
	busy := p.saving
	progress := "Creating…"
	lines = append(lines, "", pane.StatusLine(spin, p.width, p.err, busy, progress, hint))
	return tea.NewView(strings.Join(lines, "\n"))
}
