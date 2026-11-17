// Package filterpicker is a tea.Model that lists a board's quick filters and
// assignees on separate tabs and reports the one the user picks. It's meant to
// be hosted by a modal.
package filterpicker

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/components/pane"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/keys"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
)

// SelectedMsg reports the filter the user picked. A nil Filter clears it.
type SelectedMsg struct {
	Filter *jira.Filter
}

// ForParent marks SelectedMsg for the modal host.
func (SelectedMsg) ForParent() {}

// ClosesModal marks SelectedMsg for the modal host.
func (SelectedMsg) ClosesModal() {}

// keyMap is the picker's key bindings.
type keyMap struct {
	pane.KeyMap
	nextTab, prevTab, left, right, stop  key.Binding
	tabs, navigate, apply, search, reset key.Binding
}

func newKeyMap() keyMap {
	k := keys.Get()
	apply := keys.Describe(k.Accept, "apply")
	search := keys.Describe(k.ListFilter, "search")
	reset := keys.Describe(k.Reset, "reset")
	return keyMap{
		KeyMap:  pane.NewKeyMap(k.Close),
		nextTab: k.NextTab, prevTab: k.PrevTab, left: k.Left, right: k.Right,
		stop: keys.Describe(k.Dismiss, "cancel search"),
		tabs: k.SwitchTab, navigate: k.ListNavigate, apply: apply, search: search, reset: reset,
	}
}

type tab struct {
	name    string
	empty   string
	filters []jira.Filter
	list    list.Model
}

// Picker lists the board's quick filters and assignees on separate tabs. Each
// tab starts with an entry that clears the filter.
type Picker struct {
	keys          keyMap
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
	for i := range p.tabs {
		p.tabs[i].list = p.newList(p.tabs[i])
	}
	// Open on the tab holding the active filter, else the first with options.
	p.tab = p.tabWithActive()
	return p
}

func (p *Picker) newList(t tab) list.Model {
	names := []string{"All issues"}
	for _, f := range t.filters {
		names = append(names, f.Name)
	}
	l := pane.NewLabelList(pane.Labels(names), 0, 0)
	l.Select(p.activeIn(t))
	return l
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
	t := &p.tabs[p.tab]
	t.list = p.newList(*t)
	p.sizeLists()
}

func (p *Picker) sizeLists() {
	for i := range p.tabs {
		p.tabs[i].list.SetSize(p.width, max(1, p.height-3))
	}
}

func (p *Picker) current() *list.Model { return &p.tabs[p.tab].list }

// Filtering reports whether the user is typing a search.
func (p *Picker) Filtering() bool { return p.current().FilterState() == list.Filtering }

// Init implements tea.Model.
func (p *Picker) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (p *Picker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width, p.height = msg.Width, msg.Height
		p.sizeLists()
		return p, nil
	case tuimsg.ThemeChanged:
		for i := range p.tabs {
			p.tabs[i].list.SetDelegate(pane.LabelDelegate())
		}
		return p, nil
	case tea.KeyPressMsg:
		return p.handleKey(msg)
	}
	return p, p.forward(msg)
}

func (p *Picker) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	l := p.current()
	switch {
	case key.Matches(msg, p.keys.nextTab) || (!p.Filtering() && key.Matches(msg, p.keys.right)):
		p.switchTab(1)
		return p, nil
	case key.Matches(msg, p.keys.prevTab) || (!p.Filtering() && key.Matches(msg, p.keys.left)):
		p.switchTab(-1)
		return p, nil
	case p.Filtering() && key.Matches(msg, p.keys.apply):
		return p, p.apply()
	case p.Filtering():
		return p, p.forward(msg)
	case key.Matches(msg, p.keys.Close()) && !l.IsFiltered():
		return p, tuimsg.Close()
	case key.Matches(msg, p.keys.apply):
		return p, p.apply()
	case key.Matches(msg, p.keys.reset):
		return p, tuimsg.Send(SelectedMsg{})
	}
	return p, p.forward(msg)
}

// apply reports the highlighted row. Searching with no matches has no row, so
// nothing is applied.
func (p *Picker) apply() tea.Cmd {
	l := p.current()
	if l.SelectedItem() == nil {
		return nil
	}
	var sel *jira.Filter
	if row := l.GlobalIndex(); row > 0 {
		sel = &p.tabs[p.tab].filters[row-1]
	}
	return tuimsg.Send(SelectedMsg{Filter: sel})
}

func (p *Picker) forward(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	*p.current(), cmd = p.current().Update(msg)
	return cmd
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
	t := p.tabs[p.tab]
	lines := []string{strings.Join(tabs, ""), ""}
	if len(t.filters) == 0 {
		lines = append(lines, styles.Dim().Render(t.empty))
	}
	lines = append(lines, t.list.View())
	return tea.NewView(strings.Join(lines, "\n"))
}

// Footer implements modal.Footed.
func (p *Picker) Footer() (string, help.KeyMap) {
	k := p.keys.KeyMap
	if p.Filtering() {
		return "", k.WithLead(p.keys.stop, p.keys.apply).WithCloseShown(false)
	}
	return "", k.WithLead(p.keys.navigate, p.keys.tabs, p.keys.apply, p.keys.search, p.keys.reset)
}
