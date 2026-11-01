// Package styles holds the lipgloss styles jt draws with. They're derived from
// bubbletint's current tint and rebuilt as a unit when it changes, so the
// accessors always return styles that match each other. They return copies,
// so callers can derive from them freely.
package styles

import (
	"image/color"
	"sync/atomic"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/fang/v2"
	"charm.land/glamour/v2"
	"charm.land/glamour/v2/ansi"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/lrstanley/bubbletint/glamourtint/v2"
	tint "github.com/lrstanley/bubbletint/v2"
)

// set is one tint's styles. It is never modified after it's built.
type set struct {
	tint *tint.Tint

	title, dim, errStyle, warn, key, header lipgloss.Style
	columnHeader, focusedColumnHeader       lipgloss.Style
	card, selectedCard, popup, input        lipgloss.Style

	highlight, muted color.Color
	markdown         ansi.StyleConfig
}

var current atomic.Pointer[set]

func init() {
	tint.NewDefaultRegistry()
}

// get returns the styles for the current tint, building them the first time
// that tint is seen.
func get() *set {
	t := tint.Current()
	if s := current.Load(); s != nil && s.tint == t {
		return s
	}
	s := build(t)
	current.Store(s)
	return s
}

func build(t *tint.Tint) *set {
	// Subtle and muted text are the foreground faded toward the background,
	// which stays readable whichever ANSI slot a tint puts its grays in.
	fade := lipgloss.Blend1D(101, t.Bg, t.Fg)
	accent, subtle, muted, danger := color.Color(t.Blue), fade[70], fade[85], color.Color(t.Red)
	highlight := color.Color(t.Yellow)
	if !t.Dark {
		highlight = tint.Darken(highlight, 0.25)
	}

	columnHeader := lipgloss.NewStyle().Bold(true).Padding(0, 1).Foreground(subtle)
	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(subtle).
		Padding(0, 1)
	return &set{
		tint:                t,
		title:               lipgloss.NewStyle().Bold(true).Foreground(accent),
		dim:                 lipgloss.NewStyle().Foreground(subtle),
		errStyle:            lipgloss.NewStyle().Foreground(danger),
		warn:                lipgloss.NewStyle().Foreground(highlight),
		key:                 lipgloss.NewStyle().Bold(true).Foreground(accent),
		header:              lipgloss.NewStyle().Bold(true).Padding(0, 1),
		highlight:           highlight,
		muted:               muted,
		markdown:            glamourtint.StyleConfig(t),
		columnHeader:        columnHeader,
		focusedColumnHeader: columnHeader.Foreground(highlight),
		card:                card,
		selectedCard:        card.Border(lipgloss.ThickBorder()).BorderForeground(highlight),
		popup: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(0, 1),
		input: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(subtle),
	}
}

// Current returns the tint the styles are drawn from.
func Current() *tint.Tint { return get().tint }

// Background is the color painted over the terminal's own background.
func Background() color.Color { return get().tint.Bg }

// Foreground is the color painted over the terminal's own text color.
func Foreground() color.Color { return get().tint.Fg }

// Title styles headings and the app name.
func Title() lipgloss.Style { return get().title }

// Dim styles secondary text and hints.
func Dim() lipgloss.Style { return get().dim }

// Error styles error messages.
func Error() lipgloss.Style { return get().errStyle }

// Warn styles warnings and unsaved-change notices.
func Warn() lipgloss.Style { return get().warn }

// Key styles issue keys.
func Key() lipgloss.Style { return get().key }

// Header styles the top bar.
func Header() lipgloss.Style { return get().header }

// ColumnHeader styles a board column's title.
func ColumnHeader() lipgloss.Style { return get().columnHeader }

// FocusedColumnHeader styles the focused column's title.
func FocusedColumnHeader() lipgloss.Style { return get().focusedColumnHeader }

// Card styles an issue card.
func Card() lipgloss.Style { return get().card }

// SelectedCard styles the focused issue card.
func SelectedCard() lipgloss.Style { return get().selectedCard }

// Popup styles the frame around popups.
func Popup() lipgloss.Style { return get().popup }

// Input styles the border around text inputs.
func Input() lipgloss.Style { return get().input }

// Highlight is the color of the focused or selected item.
func Highlight() color.Color { return get().highlight }

// Muted is the color of secondary text.
func Muted() color.Color { return get().muted }

// Help returns a help footer drawn in the dim style.
func Help() help.Model {
	h := help.New()
	d := Dim()
	h.Styles.ShortKey = d
	h.Styles.ShortDesc = d
	h.Styles.ShortSeparator = d
	h.Styles.Ellipsis = d
	return h
}

// FormTheme returns the huh theme for edit forms.
func FormTheme() huh.Theme {
	t := get().tint
	return huh.ThemeFunc(func(bool) *huh.Styles { return form(t) })
}

// CLIColorScheme returns fang's colors for help and error output, drawn from
// the current tint.
func CLIColorScheme(lipgloss.LightDarkFunc) fang.ColorScheme {
	t := get().tint
	fade := lipgloss.Blend1D(101, t.Bg, t.Fg)
	gray := fade[70]
	return fang.ColorScheme{
		Base:           t.Fg,
		Title:          t.Blue,
		Description:    t.Fg,
		Codeblock:      fade[8],
		Program:        t.Cyan,
		DimmedArgument: gray,
		Comment:        gray,
		Flag:           t.Green,
		FlagDefault:    gray,
		Command:        t.Purple,
		QuotedString:   t.Yellow,
		Argument:       t.Fg,
		Help:           gray,
		Dash:           gray,
		ErrorHeader:    [2]color.Color{t.Bg, t.Red},
		ErrorDetails:   t.Red,
	}
}

// TextArea returns textarea styles with a muted placeholder and blurred text.
func TextArea() textarea.Styles {
	s := get()
	ts := textarea.DefaultStyles(s.tint.Dark)
	ts.Cursor.Blink = false
	ts.Focused.Placeholder = ts.Focused.Placeholder.Foreground(s.muted)
	ts.Blurred.Placeholder = ts.Blurred.Placeholder.Foreground(s.muted)
	ts.Blurred.Text = ts.Blurred.Text.Foreground(s.muted)
	return ts
}

// TextInput returns text input styles with a muted placeholder.
func TextInput() textinput.Styles {
	s := get()
	ts := textinput.DefaultStyles(s.tint.Dark)
	ts.Cursor.Blink = false
	ts.Focused.Prompt = lipgloss.NewStyle()
	ts.Blurred.Prompt = lipgloss.NewStyle()
	ts.Focused.Placeholder = ts.Focused.Placeholder.Foreground(s.muted)
	ts.Blurred.Placeholder = ts.Blurred.Placeholder.Foreground(s.muted)
	ts.Blurred.Text = ts.Blurred.Text.Foreground(s.muted)
	return ts
}

// Markdown renders md wrapped to width in the tint's colors.
func Markdown(md string, width int) string {
	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(get().markdown),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return md
	}
	out, err := r.Render(md)
	if err != nil {
		return md
	}
	return out
}
