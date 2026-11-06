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
	if height <= 0 {
		return ""
	}
	lines := strings.Count(s, "\n") + 1
	switch {
	case lines < height:
		return s + strings.Repeat("\n", height-lines)
	case lines == height:
		return s
	}
	end := -1
	for range height {
		end += 1 + strings.IndexByte(s[end+1:], '\n')
	}
	return s[:end]
}
