package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// View implements tea.Model.
func (m Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	return v
}

func (m Model) render() string {
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
	prefix := ""
	if m.status != "" {
		prefix = m.status + dimStyle.Render(" · ")
	}
	h := m.help
	h.SetWidth(max(0, m.width-2-lipgloss.Width(prefix)))
	return dimStyle.Padding(0, 1).Width(m.width).Render(prefix + h.View(keys))
}
