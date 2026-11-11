// Package view is the screen that shows one issue's details, without the
// board or search behind it.
package view

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hawkbawk/jt/internal/components/issuedetail"
	"github.com/hawkbawk/jt/internal/components/issueview"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/screen"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
)

// Model hosts an issue's details as a full screen. It does what the modal host
// does for the same content: draws its title and footer and quits when the
// content closes.
type Model struct {
	content *issuedetail.Model
	quit    screen.QuitGuard
	layout  screen.Layout
	width   int
	height  int
}

// New returns a screen showing detail, which the caller already loaded.
func New(b issueview.Backend, detail *jira.IssueDetail, md issueview.Markdown) Model {
	return Model{content: issuedetail.NewLoaded(b, detail, md), layout: screen.NewLayout(nil)}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return m.content.Init() }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, m.forward(tea.WindowSizeMsg{Width: msg.Width, Height: max(1, msg.Height-2)})
	case tea.KeyPressMsg:
		if consumed, quit := m.quit.Handle(msg); consumed {
			if quit {
				return m, tea.Quit
			}
			return m, nil
		}
	case tuimsg.Closed:
		return m, tea.Quit
	}
	return m, m.forward(msg)
}

// forward gives msg to the content and routes the messages it sends back, like
// the modal host does: a close message arrives as itself.
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
