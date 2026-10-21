package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// overlay draws fg centered on top of bg, which is width columns wide.
func overlay(bg, fg string, width int) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")
	fgWidth := lipgloss.Width(fg)

	x := max(0, (width-fgWidth)/2)
	y := max(0, (len(bgLines)-len(fgLines))/2)

	for i, line := range fgLines {
		row := y + i
		if row >= len(bgLines) {
			break
		}
		b := bgLines[row]
		left := ansi.Truncate(b, x, "")
		if w := ansi.StringWidth(left); w < x {
			left += strings.Repeat(" ", x-w)
		}
		right := ansi.TruncateLeft(b, x+fgWidth, "")
		bgLines[row] = left + ansi.ResetStyle + line + ansi.ResetStyle + right
	}
	return strings.Join(bgLines, "\n")
}

// popupSize returns the outer popup dimensions for a body area.
func popupSize(width, height int) (int, int) {
	return max(20, width*4/5), max(6, height*4/5)
}
