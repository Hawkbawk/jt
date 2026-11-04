package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/popup"
	"github.com/hawkbawk/jt/internal/tui/popup/pane"
)

var filterTabHelp = key.NewBinding(key.WithKeys("tab", "shift+tab"), key.WithHelp("tab/shift+tab", "switch"))

// filterSelectedMsg reports the chosen filter. A nil filter clears it.
type filterSelectedMsg struct {
	filter *jira.Filter
}

type filterTab struct {
	name    string
	empty   string
	filters []jira.Filter
}

// filterPopup lists the board's quick filters and assignees on separate tabs.
// Each tab starts with an entry that clears the filter.
type filterPopup struct {
	cursor        int
	selectField   *huh.Select[int]
	form          *huh.Form
	tabs          []filterTab
	tab           int
	active        *jira.Filter
	width, height int
}

func newFilterPopup(quick, assignees []jira.Filter, active *jira.Filter) *filterPopup {
	p := &filterPopup{
		tabs: []filterTab{
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

func (p *filterPopup) tabWithActive() int {
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
func (p *filterPopup) activeIn(t filterTab) int {
	for i, f := range t.filters {
		if p.active != nil && f == *p.active {
			return i + 1
		}
	}
	return 0
}

func (p *filterPopup) activeRow() int {
	return p.activeIn(p.tabs[p.tab])
}

func (p *filterPopup) switchTab(delta int) {
	p.tab = (p.tab + delta + len(p.tabs)) % len(p.tabs)
	p.buildForm()
}

func (p *filterPopup) buildForm() {
	t := p.tabs[p.tab]
	p.cursor = p.activeRow()
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

// Init implements tea.Model.
func (p *filterPopup) Init() tea.Cmd { return p.form.Init() }

// Update implements tea.Model.
func (p *filterPopup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		p.width, p.height = size.Width, size.Height
		return p, nil
	}
	if km, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(km, keys.blur) && p.consumesEsc():
			_, cmd := p.form.Update(msg)
			return p, cmd
		case !p.selectField.GetFiltering() && key.Matches(km, keys.close):
			return p, popup.Close()
		case key.Matches(km, keys.nextTab) || (!p.selectField.GetFiltering() && key.Matches(km, keys.right)):
			p.switchTab(1)
			return p, p.form.Init()
		case key.Matches(km, keys.prevTab) || (!p.selectField.GetFiltering() && key.Matches(km, keys.left)):
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
		return p, popup.CloseWith(filterSelectedMsg{filter: sel})
	}
	return p, cmd
}

func (p *filterPopup) consumesEsc() bool {
	return key.Matches(tea.KeyPressMsg{Code: tea.KeyEsc}, p.selectField.KeyBinds()...)
}

// View implements tea.Model.
func (p *filterPopup) View() tea.View {
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
	bindings := append(helpKeys{filterTabHelp}, p.selectField.KeyBinds()...)
	if !p.consumesEsc() {
		bindings = append(bindings, keys.close)
	}
	return tea.NewView(popup.Stack(p.height, header, body, helpView(bindings, p.width)))
}
