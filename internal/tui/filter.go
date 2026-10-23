package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/hawkbawk/jt/internal/jira"
)

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
	choiceList
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
	p.cursor = p.activeRow()
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

func (p *filterPopup) setSize(width, height int) {
	p.width, p.height = width, height
}

func (p *filterPopup) switchTab(delta int) {
	p.tab = (p.tab + delta + len(p.tabs)) % len(p.tabs)
	p.cursor = p.activeRow()
}

func (p *filterPopup) update(msg tea.Msg) (popup, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}
	filters := p.tabs[p.tab].filters
	switch {
	case key.Matches(km, keys.close):
		return nil, nil
	case key.Matches(km, keys.nextTab, keys.right):
		p.switchTab(1)
	case key.Matches(km, keys.prevTab, keys.left):
		p.switchTab(-1)
	case p.handle(km, len(filters)+1):
	case key.Matches(km, keys.accept):
		var sel *jira.Filter
		if p.cursor > 0 {
			sel = &filters[p.cursor-1]
		}
		return nil, func() tea.Msg { return filterSelectedMsg{filter: sel} }
	}
	return p, nil
}

func (p *filterPopup) view(string) string {
	innerW, _ := popupInner(p.width, p.height)
	t := p.tabs[p.tab]
	names := make([]string, len(p.tabs))
	for i, t := range p.tabs {
		names[i] = t.name
	}
	header := []string{renderTabs(names, p.tab), ""}
	var body []string
	if len(t.filters) == 0 {
		body = []string{dimStyle.Render(t.empty)}
	} else {
		body = p.render(append([]string{"All issues"}, filterNames(t.filters)...), innerW)
	}
	return renderPopup(p.width, p.height, header, body, dimStyle.Render("tab switch · j/k select · enter apply · esc cancel"))
}

func filterNames(fs []jira.Filter) []string {
	names := make([]string, len(fs))
	for i, f := range fs {
		names[i] = f.Name
	}
	return names
}
