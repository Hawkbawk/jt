package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/hawkbawk/jt/internal/tui/popup"
)

// View implements tea.Model.
func (m BoardModel) View() tea.View {
	return m.withView(m.render())
}

func (m BoardModel) render() string {
	if m.width == 0 {
		return ""
	}
	body := m.board.view(m.width, m.bodyHeight(), m.spinner.View())
	switch {
	case m.picker.open:
		body = popup.Overlay(body, m.picker.view(m.spinner.View()), m.width)
	default:
		body = m.popup.Overlay(body)
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.headerView(m.board.headerParts(m.spinner.View())), body, m.footerView(keys))
}
