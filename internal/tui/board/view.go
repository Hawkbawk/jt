package board

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// View implements tea.Model.
func (m Model) View() tea.View {
	return m.WithView(m.render())
}

func (m Model) render() string {
	if m.Width == 0 {
		return ""
	}
	body := m.board.view(m.Width, m.BodyHeight(), m.Spinner.View())
	body = m.Popup.Overlay(body)
	return lipgloss.JoinVertical(lipgloss.Left, m.HeaderView(m.board.headerParts(m.Spinner.View())), body, m.FooterView(newBoardKeys()))
}
