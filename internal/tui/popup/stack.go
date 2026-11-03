package popup

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Stack lays out content in a space of the given height: header lines on top,
// the footer on the last line, and the body between them, padded or clipped to
// fill the space.
func Stack(height int, header, body []string, footer string) string {
	bodyH := max(1, height-len(header)-1)
	content := lipgloss.NewStyle().Height(bodyH).MaxHeight(bodyH).Render(strings.Join(body, "\n"))
	lines := append(append([]string{}, header...), content, footer)
	return strings.Join(lines, "\n")
}
