package cmd

import (
	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/termimage"
)

// graphicsProbe finds out whether the terminal draws images, and otherwise
// leaves m alone. It's meant to wrap top-level components that support rendering images
// using the kitty graphics protocol.
type graphicsProbe struct{ tea.Model }

func (g graphicsProbe) Init() tea.Cmd {
	init := g.Model.Init()
	if q := termimage.Setup(); q != "" {
		return tea.Batch(tea.Raw(q), init)
	}
	return init
}

func (g graphicsProbe) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	termimage.Observe(msg)
	m, cmd := g.Model.Update(msg)
	return graphicsProbe{m}, cmd
}
