// Package standalone runs a component built for a modal as a full screen,
// without the board or search behind it.
package standalone

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/modal"
	"github.com/hawkbawk/jt/internal/tui/screen"
)

// Model hosts content the way the modal host does: it draws the title and
// footer, sizes the content for the room left, and quits when the content
// closes or reports that it finished with a ClosesModal message.
type Model struct {
	content tea.Model
	quit    screen.QuitGuard
	layout  screen.Layout
	width   int
}

// New returns a screen hosting content.
func New(content tea.Model) Model {
	return Model{content: content, layout: screen.NewLayout(nil)}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return m.content.Init() }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m.forward(tea.WindowSizeMsg{Width: msg.Width, Height: max(1, msg.Height-m.chromeLines())})
	case tea.KeyPressMsg:
		if consumed, quit := m.quit.Handle(msg); consumed {
			if quit {
				return m, tea.Quit
			}
			return m, nil
		}
	case modal.Closes:
		return m, tea.Quit
	}
	return m.forward(msg)
}

func (m Model) forward(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	m.content, cmd = m.content.Update(msg)
	return m, cmd
}

// chromeLines is the number of lines the title and footer take.
func (m Model) chromeLines() int {
	n := 0
	if _, ok := m.content.(modal.Titled); ok {
		n++
	}
	if _, ok := m.content.(modal.Footed); ok {
		n++
	}
	return n
}

// View implements tea.Model.
func (m Model) View() tea.View {
	out := modal.Chrome(m.content, m.width, modal.WithTitleStyle(styles.Title), modal.WithHelp(styles.Help))
	if m.quit.Confirming {
		if i := strings.LastIndex(out, "\n"); i >= 0 {
			out = out[:i+1]
		}
		out += m.quit.View()
	}
	return m.layout.WithView(out)
}
