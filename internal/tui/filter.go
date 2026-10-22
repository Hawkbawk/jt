package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

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
	tabs   []filterTab
	tab    int
	cursor int
	active *jira.Filter
	width  int
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

func (p *filterPopup) setSize(width, _ int) {
	p.width = min(width, 64)
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
	case key.Matches(km, keys.down):
		p.cursor = min(p.cursor+1, len(filters))
	case key.Matches(km, keys.up):
		p.cursor = max(p.cursor-1, 0)
	case key.Matches(km, keys.accept):
		var sel *jira.Filter
		if p.cursor > 0 {
			sel = &filters[p.cursor-1]
		}
		return nil, func() tea.Msg { return filterSelectedMsg{filter: sel} }
	}
	return p, nil
}

func (p *filterPopup) tabBar() string {
	parts := make([]string, len(p.tabs))
	for i, t := range p.tabs {
		style := dimStyle.Padding(0, 1)
		if i == p.tab {
			style = titleStyle.Padding(0, 1).Underline(true)
		}
		parts[i] = style.Render(t.name)
	}
	return strings.Join(parts, "")
}

func (p *filterPopup) view(string) string {
	innerW := p.width - popupStyle.GetHorizontalFrameSize()
	t := p.tabs[p.tab]
	lines := []string{p.tabBar(), ""}
	if len(t.filters) == 0 {
		lines = append(lines, dimStyle.Render(t.empty))
	} else {
		labels := append([]string{"All issues"}, filterNames(t.filters)...)
		for i, l := range labels {
			l = ansi.Truncate(l, innerW-2, "…")
			if i == p.cursor {
				lines = append(lines, lipgloss.NewStyle().Foreground(highlight).Bold(true).Render("› "+l))
			} else {
				lines = append(lines, "  "+l)
			}
		}
	}
	lines = append(lines, "", dimStyle.Render("tab switch · j/k select · enter apply · esc cancel"))
	return popupStyle.Width(innerW + popupStyle.GetHorizontalPadding()).Render(strings.Join(lines, "\n"))
}

func filterNames(fs []jira.Filter) []string {
	names := make([]string, len(fs))
	for i, f := range fs {
		names[i] = f.Name
	}
	return names
}
