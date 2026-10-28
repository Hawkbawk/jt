// Package styles holds the colors and lipgloss styles jt draws with. The
// whole set belongs to one theme and is replaced as a unit by Use, so the
// accessors always return styles that match each other. They return copies,
// so callers can derive from them freely. Use is safe to call while other
// goroutines read.
package styles

import (
	"image/color"
	"os"
	"sync"
	"sync/atomic"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/textarea"
	"charm.land/glamour/v2"
	"charm.land/glamour/v2/ansi"
	glamourstyles "charm.land/glamour/v2/styles"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/hawkbawk/jt/internal/config"
)

// set is one theme resolved for a terminal background. It is never modified
// after it's built.
type set struct {
	theme config.Theme
	dark  bool
	def   theme

	title, dim, errStyle, warn, key, header lipgloss.Style
	columnHeader, focusedColumnHeader       lipgloss.Style
	card, selectedCard, popup, input        lipgloss.Style

	highlight, muted color.Color
	markdown         ansi.StyleConfig
}

var (
	current  atomic.Pointer[set]
	override atomic.Pointer[bool]
	detected = sync.OnceValue(func() bool {
		return lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
	})
)

func isDark() bool {
	if d := override.Load(); d != nil {
		return *d
	}
	return detected()
}

func init() {
	Use(config.ThemeTerminal)
}

// Use switches every style to theme t, matching the terminal background.
func Use(t config.Theme) {
	use(t, isDark())
}

// SetDark overrides the detected terminal background, for example with the
// answer to a background color query, and rebuilds the active theme.
func SetDark(dark bool) {
	override.Store(&dark)
	use(Current(), dark)
}

func use(t config.Theme, dark bool) {
	current.Store(build(t, dark))
}

func get() *set { return current.Load() }

func build(t config.Theme, dark bool) *set {
	def := lookupTheme(t)
	pal := def.palette
	c := func(sh shade) color.Color { return lipgloss.Color(sh.pick(dark)) }
	accent, subtle, highlight, danger := c(pal.accent), c(pal.subtle), c(pal.highlight), c(pal.danger)

	columnHeader := lipgloss.NewStyle().Bold(true).Padding(0, 1).Foreground(subtle)
	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(subtle).
		Padding(0, 1)
	selectedCard := card.Border(lipgloss.ThickBorder()).BorderForeground(highlight)
	return &set{
		theme:               t,
		dark:                dark,
		def:                 def,
		title:               lipgloss.NewStyle().Bold(true).Foreground(accent),
		dim:                 lipgloss.NewStyle().Foreground(subtle),
		errStyle:            lipgloss.NewStyle().Foreground(danger),
		warn:                lipgloss.NewStyle().Foreground(highlight),
		key:                 lipgloss.NewStyle().Bold(true).Foreground(accent),
		header:              lipgloss.NewStyle().Bold(true).Padding(0, 1),
		highlight:           highlight,
		muted:               c(pal.muted),
		markdown:            markdownConfig(pal, dark),
		columnHeader:        columnHeader,
		focusedColumnHeader: columnHeader.Foreground(highlight),
		card:                card,
		selectedCard:        selectedCard,
		popup: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Padding(0, 1),
		input: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(subtle),
	}
}

// Current returns the active theme.
func Current() config.Theme { return get().theme }

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
	return huh.ThemeFunc(get().def.form)
}

// TextArea returns textarea styles with a muted placeholder and blurred text.
func TextArea() textarea.Styles {
	s := get()
	ts := textarea.DefaultStyles(s.dark)
	ts.Cursor.Blink = false
	ts.Focused.Placeholder = ts.Focused.Placeholder.Foreground(s.muted)
	ts.Blurred.Placeholder = ts.Blurred.Placeholder.Foreground(s.muted)
	ts.Blurred.Text = ts.Blurred.Text.Foreground(s.muted)
	return ts
}

// Markdown renders md wrapped to width in the theme's colors.
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

// markdownConfig returns the standard glamour style matching the terminal
// background, minus the document text color so body text uses the terminal's
// own foreground. That keeps it readable even when background detection is
// wrong. Headings, links, code and quotes take the palette's colors.
func markdownConfig(pal palette, dark bool) ansi.StyleConfig {
	cfg := glamourstyles.LightStyleConfig
	if dark {
		cfg = glamourstyles.DarkStyleConfig
	}
	cfg.Document.Color = nil

	heading := colorPtr(pal.accent.pick(dark))
	link := colorPtr(pal.highlight.pick(dark))
	for _, h := range []*ansi.StyleBlock{&cfg.Heading, &cfg.H1, &cfg.H2, &cfg.H3, &cfg.H4, &cfg.H5, &cfg.H6} {
		h.Color = heading
		h.BackgroundColor = nil
	}
	cfg.Link.Color = link
	cfg.LinkText.Color = link
	cfg.Code.Color = link
	cfg.Code.BackgroundColor = nil
	cfg.BlockQuote.Color = colorPtr(pal.subtle.pick(dark))
	return cfg
}

// colorPtr returns a glamour color, or nil to use the terminal's foreground.
func colorPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
