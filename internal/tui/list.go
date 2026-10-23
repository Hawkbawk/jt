package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// choiceList tracks the cursor of a vertical list of choices.
type choiceList struct {
	cursor int
}

// handle moves the cursor for an up or down key over n rows and reports
// whether the key was one.
func (l *choiceList) handle(msg tea.KeyMsg, n int) bool {
	switch {
	case key.Matches(msg, keys.down):
		l.cursor = min(l.cursor+1, max(n-1, 0))
	case key.Matches(msg, keys.up):
		l.cursor = max(l.cursor-1, 0)
	default:
		return false
	}
	return true
}

// render returns one line per label, marking the cursor row.
func (l choiceList) render(labels []string, innerW int) []string {
	lines := make([]string, len(labels))
	for i, label := range labels {
		label = ansi.Truncate(label, innerW-2, "…")
		if i == l.cursor {
			lines[i] = lipgloss.NewStyle().Foreground(highlight).Bold(true).Render("› " + label)
		} else {
			lines[i] = "  " + label
		}
	}
	return lines
}

// renderTabs draws a tab bar with the active tab underlined.
func renderTabs(names []string, active int) string {
	parts := make([]string, len(names))
	for i, name := range names {
		style := dimStyle.Padding(0, 1)
		if i == active {
			style = titleStyle.Padding(0, 1).Underline(true)
		}
		parts[i] = style.Render(name)
	}
	return strings.Join(parts, "")
}

// popupInner returns the content area of a popup with the given outer size.
func popupInner(width, height int) (int, int) {
	return width - popupStyle.GetHorizontalFrameSize(), height - popupStyle.GetVerticalFrameSize()
}

// renderPopup draws a popup of the given outer size. The body is padded or
// clipped to fill the space between header and footer, so the popup never
// changes size.
func renderPopup(width, height int, header, body []string, footer string) string {
	innerW, innerH := popupInner(width, height)
	bodyH := max(1, innerH-len(header)-1)
	content := lipgloss.NewStyle().Height(bodyH).MaxHeight(bodyH).Render(strings.Join(body, "\n"))
	lines := append(append([]string{}, header...), content, footer)
	return popupStyle.Width(innerW + popupStyle.GetHorizontalPadding()).Render(strings.Join(lines, "\n"))
}
