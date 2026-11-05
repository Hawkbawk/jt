// Package filterpicker is a tea.Model that lists a board's quick filters and
// assignees on separate tabs and reports the one the user picks. It's meant to
// be hosted by a popup.
package filterpicker

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/keys"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/popup"
	"github.com/hawkbawk/jt/internal/tui/popup/pane"
)

// SelectedMsg reports the filter the user picked. A nil Filter clears it.
type SelectedMsg struct {
	Filter *jira.Filter
}

// keyMap is the picker's key bindings.
type keyMap struct {
	popup.KeyMap
	nextTab, prevTab, left, right, blur key.Binding
}

func newKeyMap() keyMap {
	k := keys.Get()
	return keyMap{
		KeyMap:  popup.NewKeyMap(k.Close).WithLead(k.NextTab, k.PrevTab),
		nextTab: k.NextTab, prevTab: k.PrevTab, left: k.Left, right: k.Right, blur: k.Dismiss,
	}
}

// helpView renders a help footer in the given width.
func helpView(km popup.KeyMap, width int) string {
	h := styles.Help()
	h.SetWidth(max(0, width))
	return h.View(km)
}

type tab struct {
	name    string
	empty   string
	filters []jira.Filter
}

// Picker lists the board's quick filters and assignees on separate tabs. Each
// tab starts with an entry that clears the filter.
type Picker struct {
	keys          keyMap
	cursor        int
	selectField   *huh.Select[int]
	form          *huh.Form
	tabs          []tab
	tab           int
	active        *jira.Filter
	width, height int
}

// New builds a picker that opens on the tab and row of the active filter.
func New(quick, assignees []jira.Filter, active *jira.Filter) *Picker {
	p := &Picker{
		keys: newKeyMap(),
		tabs: []tab{
			{name: "Quick filters", empty: "This board has no quick filters.", filters: quick},
			{name: "Assignees", empty: "No assignees on this board.", filters: assignees},
		},
		active: active,
	}
	// Open on the tab holding the active filter, else the first with options.
	p.tab = p.tabWithActive()
	p.buildForm()
	return p
}

func (p *Picker) tabWithActive() int {
	for i, t := range p.tabs {
		if p.activeIn(t) > 0 {
			return i
		}
	}
	if len(p.tabs[0].filters) == 0 && len(p.tabs[1].filters) > 0 {
		return 1
	}
	return 0
}

// activeIn returns the row of the active filter in t, or 0 (the clear entry).
func (p *Picker) activeIn(t tab) int {
	for i, f := range t.filters {
		if p.active != nil && f == *p.active {
			return i + 1
		}
	}
	return 0
}

func (p *Picker) switchTab(delta int) {
	p.tab = (p.tab + delta + len(p.tabs)) % len(p.tabs)
	p.buildForm()
}

func (p *Picker) buildForm() {
	t := p.tabs[p.tab]
	p.cursor = p.activeIn(t)
	opts := []huh.Option[int]{huh.NewOption("All issues", 0)}
	for i, f := range t.filters {
		opts = append(opts, huh.NewOption(f.Name, i+1))
	}
	p.selectField = huh.NewSelect[int]().Title("Select a filter").Options(opts...).Value(&p.cursor)
	if len(t.filters) == 0 {
		p.selectField.Description(t.empty)
	}
	p.form = pane.NewForm(p.selectField)
	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(key.WithDisabled())
	km.Select.Filter.SetHelp("/", "search")
	km.Select.SetFilter.SetHelp("esc", "stop search")
	km.Select.ClearFilter.SetHelp("esc", "clear search")
	km.Select.Submit.SetHelp("enter", "apply")
	p.form.WithKeyMap(km)
}

// Filtering reports whether the user is typing a search.
func (p *Picker) Filtering() bool { return p.selectField.GetFiltering() }

// Init implements tea.Model.
func (p *Picker) Init() tea.Cmd { return p.form.Init() }

// Update implements tea.Model.
func (p *Picker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		p.width, p.height = size.Width, size.Height
		return p, nil
	}
	if km, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(km, p.keys.blur) && p.consumesEsc():
			_, cmd := p.form.Update(msg)
			return p, cmd
		case !p.Filtering() && key.Matches(km, p.keys.Close()):
			return p, popup.Close()
		case key.Matches(km, p.keys.nextTab) || (!p.Filtering() && key.Matches(km, p.keys.right)):
			p.switchTab(1)
			return p, p.form.Init()
		case key.Matches(km, p.keys.prevTab) || (!p.Filtering() && key.Matches(km, p.keys.left)):
			p.switchTab(-1)
			return p, p.form.Init()
		}
	}
	_, cmd := p.form.Update(msg)
	if p.form.State == huh.StateCompleted {
		var sel *jira.Filter
		if p.cursor > 0 {
			sel = &p.tabs[p.tab].filters[p.cursor-1]
		}
		return p, popup.CloseWith(SelectedMsg{Filter: sel})
	}
	return p, cmd
}

func (p *Picker) consumesEsc() bool {
	return key.Matches(tea.KeyPressMsg{Code: tea.KeyEsc}, p.selectField.KeyBinds()...)
}

// View implements tea.Model.
func (p *Picker) View() tea.View {
	tabs := make([]string, len(p.tabs))
	for i, t := range p.tabs {
		style := styles.Dim().Padding(0, 1)
		if i == p.tab {
			style = styles.Title().Padding(0, 1).Underline(true)
		}
		tabs[i] = style.Render(t.name)
	}
	header := []string{strings.Join(tabs, ""), ""}
	p.form.WithWidth(p.width).WithHeight(max(1, p.height-len(header)-1))
	body := strings.Split(p.form.View(), "\n")
	km := p.keys.WithForm(p.selectField.KeyBinds(), p.consumesEsc())
	return tea.NewView(popup.Stack(p.height, header, body, helpView(km, p.width)))
}
