// Package icons holds the Nerd Font glyphs jt draws next to the help for common
// actions. They are Font Awesome codepoints from the Nerd Fonts patch, so the
// terminal needs a Nerd Font to show them. They can be turned off in settings.
package icons

import "unicode/utf8"

// Glyphs, each named for what it stands for.
const (
	View     = ""      // nf-fa-eye
	Comments = ""      // nf-fa-comment
	Commands = ""      // nf-fa-terminal
	Edit     = ""      // nf-fa-pencil
	Move     = ""      // nf-fa-exchange
	User     = ""      // nf-fa-user
	NoUser   = ""      // nf-fa-user_o
	Jira     = ""      // nf-dev-jira
	Project  = ""      // nf-fa-folder
	Star     = ""      // nf-fa-star
	Check    = ""      // nf-fa-check
	Yank     = ""      // nf-fa-clipboard
	Open     = ""      // nf-fa-external_link
	Filter   = ""      // nf-fa-filter
	Refresh  = ""      // nf-fa-refresh
	Theme    = "\uf1fc" // nf-fa-paint_brush
	Boards   = "\uf0db" // nf-fa-columns
	Quit     = "\uf08b" // nf-fa-sign_out
	Help     = "\uf059" // nf-fa-question_circle
	Reset    = "\uf12d" // nf-fa-eraser
	Undo     = "\uf0e2" // nf-fa-undo
	Close    = "\uf00d" // nf-fa-times
	Navigate = "\uf047" // nf-fa-arrows
	Scroll   = "\uf07d" // nf-fa-arrows_v
	Search   = "\uf002" // nf-fa-search
	Go       = "\uf061" // nf-fa-arrow_right
	Next     = "\uf178" // nf-fa-long_arrow_right
	Prev     = "\uf177" // nf-fa-long_arrow_left
	Toggle   = "\uf205" // nf-fa-toggle_on
	NewLine  = "\uf149" // nf-fa-level_down
	Results  = "\uf0ca" // nf-fa-list_ul
	Mode     = "\uf0ec" // nf-fa-exchange

	// Pill ends, from the Powerline Extra symbols.
	PillLeft  = "\ue0b6"
	PillRight = "\ue0b4"
)

var enabled = true

// SetEnabled turns the icons on or off. Call it before building the key
// bindings, which read it when they're made.
func SetEnabled(on bool) { enabled = on }

// Enabled reports whether icons are on.
func Enabled() bool { return enabled }

// Or returns icon, or plain when icons are off.
func Or(icon, plain string) string {
	if !enabled {
		return plain
	}
	return icon
}

// Label puts icon before text, or returns text alone when icons are off.
func Label(icon, text string) string {
	if !enabled {
		return text
	}
	return icon + " " + text
}

// Lead returns the icon that starts s, or "" when it doesn't start with one.
func Lead(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if r >= 0xe000 && r <= 0xf8ff && n < len(s) && s[n] == ' ' {
		return s[:n]
	}
	return ""
}

// Strip returns s without the icon that starts it, for the long help, which
// shows plain text.
func Strip(s string) string {
	if ic := Lead(s); ic != "" {
		return s[len(ic)+1:]
	}
	return s
}
