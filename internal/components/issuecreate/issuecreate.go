// Package issuecreate is the screen that creates an issue. It first asks for the
// project, then the issue type, then lists the fields that type can be created
// with and edits them one at a time. When the issue is created it closes with a
// Created message.
package issuecreate

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/components/fieldform"
	"github.com/hawkbawk/jt/internal/components/pane"
	"github.com/hawkbawk/jt/internal/components/projectpicker"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/keys"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
)

// Backend loads what an issue can be created with and creates it.
type Backend interface {
	Projects() ([]jira.Project, error)
	DefaultProject() string
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

// projectPickedMsg is the picker's SelectedMsg, modified for our purposes.
type projectPickedMsg struct {
	project string
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
	stageProject stage = iota
	stageType
	stageFields
)

type confirmKind int

const (
	confirmNone confirmKind = iota
	confirmBack
)

// Model asks for a project, then an issue type, then lists the fields to create
// the issue with. Values entered are kept until the issue is created.
type Model struct {
	keys     fieldform.KeyMap
	typeKeys pane.KeyMap
	backend  Backend
	picker   *projectpicker.Picker
	spin     spinner.Model
	help     help.Model

	width, height int
	stage         stage
	confirm       confirmKind
	err           error

	loadingTypes bool
	loadingForm  bool
	saving       bool

	types   []jira.IssueType
	project string
	direct  bool
	itype   jira.IssueType
	list    list.Model

	fields fieldform.Editor
	loaded bool
}

// New returns the modal that creates an issue, with the cursor on project in
// the project list. It loads the projects when it opens.
func New(b Backend, project string) *Model {
	k := keys.Get()
	m := &Model{
		backend: b, spin: pane.NewSpinner(), help: styles.Help(),
		keys: fieldform.NewKeyMap(),
	}
	m.picker = projectpicker.New(b, nil, true)
	m.picker.Select(project)
	m.keys.SaveLabel, m.keys.DismissLabel = "create", "back"
	m.typeKeys = pane.NewKeyMap(keys.Describe(k.Dismiss, "back")).
		WithLead(k.ListNavigate, keys.Describe(k.Accept, "continue"))
	m.fields.Help("create", "back")
	return m
}

// NewForProject returns the modal that creates an issue in project, skipping
// the project list. It loads the issue types when it opens.
func NewForProject(b Backend, project string) *Model {
	m := New(b, project)
	m.direct, m.project, m.loadingTypes = true, project, true
	return m
}

// Init implements tea.Model.
func (p *Model) Init() tea.Cmd {
	if p.direct {
		return tea.Batch(p.loadTypes(p.project), p.spin.Tick)
	}
	return p.picker.Init()
}

func (p *Model) loadTypes(project string) tea.Cmd {
	b := p.backend
	return func() tea.Msg {
		ts, err := b.CreateIssueTypes(project)
		return typesLoadedMsg{project: project, types: ts, err: err}
	}
}

func (p *Model) loadFields() tea.Cmd {
	b, project, t := p.backend, p.project, p.itype.ID
	return func() tea.Msg {
		fs, err := b.CreateFields(project, t)
		return fieldsLoadedMsg{project: project, issueType: t, fields: fs, err: err}
	}
}

func (p *Model) create(fields []jira.IssueField) tea.Cmd {
	b, project, t := p.backend, p.project, p.itype.ID
	return func() tea.Msg {
		k, err := b.Create(project, t, fields)
		return createdMsg{key: k, err: err}
	}
}

func (p *Model) loading() bool {
	return p.loadingTypes || p.loadingForm || p.saving
}

func (p *Model) unsaved() int { return len(p.fields.Changed()) }

// buildTypeList lists p.types, with the cursor on the type picked before if
// there was one.
func (p *Model) buildTypeList() tea.Cmd {
	items := make([]list.Item, len(p.types))
	selected := 0
	for i, t := range p.types {
		items[i] = typeItem(t)
		if t.ID == p.itype.ID {
			selected = i
		}
	}
	l := list.New(items, typeDelegate(), p.width, max(1, p.height-pane.BodyLines))
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetShowFilter(false)
	l.SetFilteringEnabled(false)
	l.KeyMap.Quit = key.NewBinding()
	l.KeyMap.ForceQuit = key.NewBinding()
	l.Select(selected)
	p.list = l
	return nil
}

type typeItem jira.IssueType

func (i typeItem) Title() string       { return i.Name }
func (i typeItem) Description() string { return "" }
func (i typeItem) FilterValue() string { return i.Name }

func typeDelegate() list.DefaultDelegate {
	d := styles.ListDelegate()
	d.ShowDescription = false
	d.SetSpacing(0)
	return d
}

// noTypes is the error for a project that can't create anything, or nil.
func noTypes(project string, types []jira.IssueType) error {
	if len(types) > 0 {
		return nil
	}
	return fmt.Errorf("no issue types can be created in %s", project)
}

// Update implements tea.Model.
func (p *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	was := p.loading()
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width, p.height = msg.Width, msg.Height
		_, cmd = p.picker.Update(tea.WindowSizeMsg{Width: msg.Width, Height: max(1, msg.Height-2)})
	case spinner.TickMsg:
		_, pickerTick := p.picker.Update(msg)
		cmd = tea.Batch(pane.Tick(&p.spin, msg, p.loading()), pickerTick)
	case tuimsg.ThemeChanged:
		_, cmd = p.picker.Update(msg)
		p.list.SetDelegate(typeDelegate())
		cmd = tea.Batch(cmd, p.update(msg))
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
	switch p.stage {
	case stageProject:
		if p.direct {
			if key.Matches(km, p.keys.Blur) || key.Matches(km, keys.Get().Quit) {
				return tuimsg.Close()
			}
			return nil
		}
		if p.loadingTypes {
			if key.Matches(km, p.keys.Blur) {
				return tuimsg.Close()
			}
			return nil
		}
		if key.Matches(km, keys.Get().Quit) && !p.picker.Filtering() {
			return tuimsg.Close()
		}
		return p.update(km)
	case stageType:
		switch {
		case p.direct && key.Matches(km, keys.Get().Quit):
			return tuimsg.Close()
		case key.Matches(km, p.keys.Blur):
			if p.direct {
				return tuimsg.Close()
			}
			p.stage, p.err = stageProject, nil
			return nil
		}
		return p.update(km)
	}
	if p.fields.ConsumesEsc() && key.Matches(km, p.keys.Blur) {
		return p.update(km)
	}
	switch {
	case p.fields.Capturing():
	case key.Matches(km, p.keys.Blur):
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
		p.confirm = confirmNone
		return p.goBack()
	case key.Matches(km, p.keys.Deny):
		p.confirm = confirmNone
	case key.Matches(km, p.keys.Send):
		p.confirm = confirmNone
		return p.update(km)
	}
	return nil
}

// goBack returns to picking the issue type, dropping the fields.
func (p *Model) goBack() tea.Cmd {
	p.stage, p.loaded, p.err, p.loadingForm = stageType, false, nil, false
	p.fields.Load(nil, nil)
	return p.buildTypeList()
}

func (p *Model) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case projectPickedMsg:
		p.project, p.loadingTypes, p.err = msg.project, true, nil
		return p.loadTypes(msg.project)
	case typesLoadedMsg:
		if !p.loadingTypes || msg.project != p.project {
			return nil
		}
		p.loadingTypes = false
		if msg.err != nil {
			p.err = msg.err
			return nil
		}
		if p.err = noTypes(msg.project, msg.types); p.err != nil {
			return nil
		}
		p.types, p.stage = msg.types, stageType
		return p.buildTypeList()
	case fieldsLoadedMsg:
		if msg.project != p.project || msg.issueType != p.itype.ID {
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
	switch p.stage {
	case stageProject:
		_, cmd := p.picker.Update(msg)
		return keepOpen(cmd)
	case stageType:
		return p.updateType(msg)
	}
	if !p.loaded || p.saving || p.fields.Len() == 0 {
		return nil
	}
	if km, ok := msg.(tea.KeyMsg); ok && !p.fields.Holding() && key.Matches(km, p.keys.Send) {
		return p.submit()
	}
	return p.fields.Update(msg)
}

// keepOpen makes the picker's SelectedMsg a projectPickedMsg, since the picker
// is a step of this modal and a pick must not close it.
func keepOpen(cmd tea.Cmd) tea.Cmd {
	return tuimsg.Map(cmd, func(msg tea.Msg) tea.Msg {
		if sel, ok := msg.(projectpicker.SelectedMsg); ok {
			return projectPickedMsg{project: sel.Key}
		}
		return msg
	})
}

// updateType passes msg to the issue type list, loading the fields once a type
// is picked.
func (p *Model) updateType(msg tea.Msg) tea.Cmd {
	if km, ok := msg.(tea.KeyPressMsg); ok && key.Matches(km, keys.Get().Accept) {
		sel, ok := p.list.SelectedItem().(typeItem)
		if !ok {
			return nil
		}
		p.itype = jira.IssueType(sel)
		p.err = nil
		p.stage, p.loadingForm = stageFields, true
		return p.loadFields()
	}
	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return cmd
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
	if p.direct && p.stage == stageProject {
		return "Create · " + p.project
	}
	switch p.stage {
	case stageType:
		return "Create · " + p.project
	case stageFields:
		return "Create · " + p.project + " · " + p.itype.Name
	}
	return "Create"
}

// Footer implements modal.Footed.
func (p *Model) Footer() (string, help.KeyMap) {
	if n := p.unsaved(); n > 0 && p.confirm != confirmNone {
		return styles.Warn().Render("Discard " + fieldform.Changes(n) + "? "), p.keys.WithMode(fieldform.HelpDiscard)
	}
	if p.stage == stageType {
		return "", p.typeKeys
	}
	return "", noKeys{}
}

// noKeys is the footer help of the project list, which shows its own.
type noKeys struct{}

func (noKeys) ShortHelp() []key.Binding  { return nil }
func (noKeys) FullHelp() [][]key.Binding { return nil }

// View implements tea.Model.
func (p *Model) View() tea.View {
	p.help.SetWidth(p.width)
	spin := p.spin.View()
	if p.stage == stageProject {
		var lines []string
		if !p.direct {
			lines = append(lines, p.picker.View().Content)
		}
		if p.err != nil || p.loadingTypes {
			lines = append(lines, pane.StatusLine(spin, p.width, p.err, p.loadingTypes, "Loading issue types…", ""))
		}
		return tea.NewView(strings.Join(lines, "\n"))
	}
	bodyH := max(1, p.height-pane.BodyLines)
	var warn string
	if n := p.unsaved(); n > 0 {
		warn = styles.Warn().Render("● " + fieldform.Changes(n))
		if !p.fields.Holding() {
			warn += styles.Dim().Render(" · ") + p.help.View(p.keys.WithMode(fieldform.HelpSave))
		}
	}
	lines := []string{warn}
	var hint string
	switch {
	case p.stage == stageType:
		p.list.SetSize(p.width, bodyH)
		lines = append(lines, p.list.View())
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
