package styles

import (
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/hawkbawk/jt/internal/config"
)

// shade is a color for light and dark terminal backgrounds. Values are hex
// strings or ANSI color numbers.
type shade struct{ light, dark string }

func (s shade) pick(dark bool) string {
	if dark {
		return s.dark
	}
	return s.light
}

// palette holds the colors the board itself uses. Markdown takes its colors
// from it too, and forms from the theme's huh styles.
type palette struct {
	accent, subtle, highlight, muted, danger shade
}

type theme struct {
	palette palette
	form    func(isDark bool) *huh.Styles
}

// themes is read-only. Use lookupTheme to read it.
var themes = map[config.Theme]theme{
	config.ThemeBase: {
		palette: palette{
			accent:    shade{"0", "15"},
			subtle:    shade{"8", "8"},
			highlight: shade{"0", "15"},
			muted:     shade{"8", "7"},
			danger:    shade{"1", "9"},
		},
		form: huh.ThemeBase,
	},
	config.ThemeCharm: {
		palette: palette{
			accent:    shade{"#5A56E0", "#7571F9"},
			subtle:    shade{"#8A8A8A", "#6C6C6C"},
			highlight: shade{"#D14FB8", "#F780E2"},
			muted:     shade{"#4A4A4A", "#B0B0B0"},
			danger:    shade{"#FF4672", "#ED567A"},
		},
		form: huh.ThemeCharm,
	},
	config.ThemeDracula: {
		palette: palette{
			accent:    shade{"#BD93F9", "#BD93F9"},
			subtle:    shade{"#6272A4", "#6272A4"},
			highlight: shade{"#F1FA8C", "#F1FA8C"},
			muted:     shade{"#F8F8F2", "#F8F8F2"},
			danger:    shade{"#FF5555", "#FF5555"},
		},
		form: huh.ThemeDracula,
	},
	config.ThemeBase16: {
		palette: palette{
			accent:    shade{"6", "6"},
			subtle:    shade{"8", "8"},
			highlight: shade{"3", "3"},
			muted:     shade{"8", "7"},
			danger:    shade{"9", "9"},
		},
		form: huh.ThemeBase16,
	},
	config.ThemeCatppuccin: {
		palette: palette{
			accent:    shade{"#8839EF", "#CBA6F7"},
			subtle:    shade{"#9CA0B0", "#6C7086"},
			highlight: shade{"#FE640B", "#FAB387"},
			muted:     shade{"#6C6F85", "#A6ADC8"},
			danger:    shade{"#D20F39", "#F38BA8"},
		},
		form: huh.ThemeCatppuccin,
	},
	config.ThemeGruvbox: {
		palette: palette{
			accent:    shade{"#8F3F71", "#D3869B"},
			subtle:    shade{"#928374", "#928374"},
			highlight: shade{"#B57614", "#FABD2F"},
			muted:     shade{"#504945", "#BDAE93"},
			danger:    shade{"#9D0006", "#FB4934"},
		},
		form: themeGruvbox,
	},
}

// lookupTheme returns the definition for t, or base for an unknown value.
func lookupTheme(t config.Theme) theme {
	if def, ok := themes[t]; ok {
		return def
	}
	return themes[config.ThemeBase]
}

// themeGruvbox returns a huh theme based on the gruvbox color scheme.
func themeGruvbox(isDark bool) *huh.Styles {
	t := huh.ThemeBase(isDark)
	ld := lipgloss.LightDark(isDark)

	var (
		fg      = ld(lipgloss.Color("#3C3836"), lipgloss.Color("#EBDBB2"))
		border  = ld(lipgloss.Color("#BDAE93"), lipgloss.Color("#504945"))
		gray    = lipgloss.Color("#928374")
		red     = ld(lipgloss.Color("#9D0006"), lipgloss.Color("#FB4934"))
		green   = ld(lipgloss.Color("#79740E"), lipgloss.Color("#B8BB26"))
		yellow  = ld(lipgloss.Color("#B57614"), lipgloss.Color("#FABD2F"))
		purple  = ld(lipgloss.Color("#8F3F71"), lipgloss.Color("#D3869B"))
		aqua    = ld(lipgloss.Color("#427B58"), lipgloss.Color("#8EC07C"))
		blurBg  = ld(lipgloss.Color("#EBDBB2"), lipgloss.Color("#3C3836"))
		buttonF = ld(lipgloss.Color("#FBF1C7"), lipgloss.Color("#282828"))
	)

	t.Focused.Base = t.Focused.Base.BorderForeground(border)
	t.Focused.Card = t.Focused.Base
	t.Focused.Title = t.Focused.Title.Foreground(yellow).Bold(true)
	t.Focused.NoteTitle = t.Focused.NoteTitle.Foreground(yellow).Bold(true)
	t.Focused.Directory = t.Focused.Directory.Foreground(aqua)
	t.Focused.File = t.Focused.File.Foreground(fg)
	t.Focused.Description = t.Focused.Description.Foreground(gray)
	t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(red)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(red)
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(purple)
	t.Focused.NextIndicator = t.Focused.NextIndicator.Foreground(purple)
	t.Focused.PrevIndicator = t.Focused.PrevIndicator.Foreground(purple)
	t.Focused.Option = t.Focused.Option.Foreground(fg)
	t.Focused.MultiSelectSelector = t.Focused.MultiSelectSelector.Foreground(purple)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(green)
	t.Focused.SelectedPrefix = t.Focused.SelectedPrefix.Foreground(green)
	t.Focused.UnselectedOption = t.Focused.UnselectedOption.Foreground(fg)
	t.Focused.UnselectedPrefix = t.Focused.UnselectedPrefix.Foreground(gray)
	t.Focused.FocusedButton = t.Focused.FocusedButton.Foreground(buttonF).Background(yellow).Bold(true)
	t.Focused.BlurredButton = t.Focused.BlurredButton.Foreground(fg).Background(blurBg)
	t.Focused.Next = t.Focused.FocusedButton

	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(yellow)
	t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(gray)
	t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(purple)
	t.Focused.TextInput.Text = t.Focused.TextInput.Text.Foreground(fg)

	t.Blurred = t.Focused
	t.Blurred.Base = t.Blurred.Base.BorderStyle(lipgloss.HiddenBorder())
	t.Blurred.Card = t.Blurred.Base
	t.Blurred.NextIndicator = lipgloss.NewStyle()
	t.Blurred.PrevIndicator = lipgloss.NewStyle()

	t.Group.Title = t.Focused.Title
	t.Group.Description = t.Focused.Description
	return t
}
