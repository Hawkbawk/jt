package tui

import (
	"os"

	"charm.land/lipgloss/v2"
)

var darkBackground = lipgloss.HasDarkBackground(os.Stdin, os.Stdout)

var lightDark = lipgloss.LightDark(darkBackground)

var (
	accent    = lightDark(lipgloss.Color("#5A3FC0"), lipgloss.Color("#A08CFF"))
	subtle    = lightDark(lipgloss.Color("#8A8A8A"), lipgloss.Color("#6C6C6C"))
	highlight = lightDark(lipgloss.Color("#D97706"), lipgloss.Color("#F5B041"))
	muted     = lightDark(lipgloss.Color("#4A4A4A"), lipgloss.Color("#B0B0B0"))
	danger    = lightDark(lipgloss.Color("#C0392B"), lipgloss.Color("#FF6B6B"))

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
