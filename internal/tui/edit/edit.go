// Package edit is the screen that edits one issue's fields, without the board
// or search behind it.
package edit

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hawkbawk/jt/internal/components/issueedit"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/screen"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
)

// Model hosts the issue edit pane as a full screen. It does what the modal host
// does for the same content: draws its title and footer and quits when the
// content closes or finishes.
type Model struct {
	content *issueedit.Model
	quit    screen.QuitGuard
	layout  screen.Layout
	width   int
}

// New returns a screen editing iss.
func New(b issueedit.Backend, iss jira.Issue) Model {
	return Model{content: issueedit.New(b, iss), layout: screen.NewLayout(nil)}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return m.content.Init() }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, m.forward(tea.WindowSizeMsg{Width: msg.Width, Height: max(1, msg.Height-2)})
	case tea.KeyPressMsg:
		if consumed, quit := m.quit.Handle(msg); consumed {
			if quit {
				return m, tea.Quit
			}
			return m, nil
		}
	case tuimsg.Closed, issueedit.Edited:
		return m, tea.Quit
	}
	return m, m.forward(msg)
}

func (m Model) forward(msg tea.Msg) tea.Cmd {
	_, cmd := m.content.Update(msg)
	return cmd
}

// View implements tea.Model.
func (m Model) View() tea.View {
	body := m.content.View().Content
	status, keys := m.content.Footer()
	footer := status
	if m.quit.Confirming {
		footer = m.quit.View()
	} else if keys != nil {
		h := styles.Help()
		h.SetWidth(max(0, m.width-lipgloss.Width(status)))
		footer += h.View(keys)
	}
	return m.layout.WithView(styles.Title().Render(m.content.Title()) + "\n" + body + "\n" + footer)
}
