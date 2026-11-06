package popup

import "strings"

// Stack lays out content in a space of the given height: header lines on top,
// the footer on the last line, and the body between them, padded or clipped to
// fill the space.
func Stack(height int, header, body []string, footer string) string {
	bodyH := max(1, height-len(header)-1)
	content := fitHeight(strings.Join(body, "\n"), bodyH)
	lines := append(append([]string{}, header...), content, footer)
	return strings.Join(lines, "\n")
}

// fitHeight clips or pads s to height lines. Unlike lipgloss's Height, it
// doesn't measure or pad the width of each line, which is slow on a screen of
// styled text. The frame around the content pads the width.
func fitHeight(s string, height int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}
