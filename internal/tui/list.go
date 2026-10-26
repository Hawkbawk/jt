package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// choiceList tracks the cursor of a vertical list of choices, and the first
// row shown when the list is scrolled.
type choiceList struct {
	cursor int
	top    int
}

// handle moves the cursor for an up or down key over n rows and reports
// whether the key was one.
func (l *choiceList) handle(msg tea.KeyPressMsg, n int) bool {
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

// renderWindow renders the rows that fit in height, scrolling just enough to
// keep the cursor in view.
func (l *choiceList) renderWindow(labels []string, innerW, height int) []string {
	height = max(height, 1)
	l.top = max(0, min(l.top, l.cursor, len(labels)-height), l.cursor-height+1)
	return l.render(labels, innerW)[l.top:min(len(labels), l.top+height)]
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
	return popupStyle.Width(innerW + popupStyle.GetHorizontalFrameSize()).Render(strings.Join(lines, "\n"))
}
