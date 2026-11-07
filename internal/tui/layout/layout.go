// Package layout holds what jt's components draw with: a stack that pins a
// footer under a header and body, the help key map, titles and status lines.
// Nothing in it knows where a component is shown.
package layout

import "github.com/hawkbawk/jt/internal/styles"

// ChromeLines is what a component draws around its body: a title and a blank line
// above it, and the help footer below.
const ChromeLines = 3

// Header is the title a component draws above its body.
func Header(title, issueKey string) []string {
	return []string{styles.Title().Render(title + " · " + issueKey), ""}
}

// StatusLine shows the error, the in-progress text or the hint, in that order.
func StatusLine(spin string, innerW int, err error, applying bool, progress, hint string) string {
	switch {
	case err != nil:
		return styles.Error().Width(innerW).Render("Error: " + err.Error())
	case applying:
		return spin + " " + progress
	}
	return styles.Dim().Render(hint)
}
