package tui

import "github.com/charmbracelet/lipgloss"

var (
	accent    = lipgloss.AdaptiveColor{Light: "#5A3FC0", Dark: "#A08CFF"}
	subtle    = lipgloss.AdaptiveColor{Light: "#8A8A8A", Dark: "#6C6C6C"}
	highlight = lipgloss.AdaptiveColor{Light: "#D97706", Dark: "#F5B041"}
	muted     = lipgloss.AdaptiveColor{Light: "#4A4A4A", Dark: "#B0B0B0"}
	danger    = lipgloss.AdaptiveColor{Light: "#C0392B", Dark: "#FF6B6B"}

	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(accent)
	dimStyle    = lipgloss.NewStyle().Foreground(subtle)
	errorStyle  = lipgloss.NewStyle().Foreground(danger)
	warnStyle   = lipgloss.NewStyle().Foreground(highlight)
	keyStyle    = lipgloss.NewStyle().Bold(true).Foreground(accent)
	headerStyle = lipgloss.NewStyle().Bold(true).Padding(0, 1)

	columnHeaderStyle        = lipgloss.NewStyle().Bold(true).Padding(0, 1).Foreground(subtle)
	focusedColumnHeaderStyle = columnHeaderStyle.Foreground(highlight)

	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(subtle).
			Padding(0, 1)
	selectedCardStyle = cardStyle.BorderForeground(highlight)

	popupStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(0, 1)
)

var inputBorder = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(subtle)
