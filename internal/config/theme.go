package config

import (
	"fmt"
	"strings"
)

// Theme is one of the supported color themes. The zero value is ThemeBase.
type Theme int

// The supported themes.
const (
	ThemeBase Theme = iota
	ThemeCharm
	ThemeDracula
	ThemeBase16
	ThemeCatppuccin
	ThemeGruvbox
)

var themeNames = [...]string{"base", "charm", "dracula", "base16", "catppuccin", "gruvbox"}

// Themes returns every supported theme.
func Themes() []Theme {
	ts := make([]Theme, len(themeNames))
	for i := range ts {
		ts[i] = Theme(i)
	}
	return ts
}

func (t Theme) String() string {
	if t < 0 || int(t) >= len(themeNames) {
		return fmt.Sprintf("Theme(%d)", int(t))
	}
	return themeNames[t]
}

// UnmarshalText parses a theme name case-insensitively. viper's decoder uses
// it, so an unknown name fails config loading.
func (t *Theme) UnmarshalText(text []byte) error {
	name := strings.ToLower(string(text))
	for i, n := range themeNames {
		if n == name {
			*t = Theme(i)
			return nil
		}
	}
	return fmt.Errorf("unknown theme %q (available: %s)", text, strings.Join(themeNames[:], ", "))
}
