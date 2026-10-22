package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const helpText = "hjkl navigate · g/G top/bottom · v view · c comments · m move · y yank URL · o open · ctrl+p projects · q quit"

// View implements tea.Model.
func (m Model) View() string {
	if m.width == 0 {
		return ""
	}
	body := m.board.view(m.width, m.bodyHeight(), m.spinner.View())
	switch {
	case m.picker.open:
		body = overlay(body, m.picker.view(m.spinner.View()), m.width)
	case m.popup != nil:
		body = overlay(body, m.popup.view(m.spinner.View()), m.width)
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.headerView(), body, m.footerView())
}

func (m Model) headerView() string {
	parts := append([]string{titleStyle.Render("jt")}, m.board.headerParts(m.spinner.View())...)
	return headerStyle.Width(m.width).Render(strings.Join(parts, dimStyle.Render(" · ")))
}

func (m Model) footerView() string {
	if m.quit.confirming {
		return m.quit.view()
	}
	help := helpText
	if m.status != "" {
		help = m.status + dimStyle.Render(" · ") + help
	}
	return dimStyle.Padding(0, 1).Width(m.width).Render(help)
}
