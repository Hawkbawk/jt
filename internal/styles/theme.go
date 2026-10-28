package styles

import (
	"context"
	"errors"
	"fmt"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/lrstanley/bubbletint/sources/v2/terminal"
	tint "github.com/lrstanley/bubbletint/v2"
)

// fallback is the tint jt uses when it can't read the terminal's theme. It's
// also where the default registry starts.
var fallback = tint.TintDraculaPlus

// detect finds the source for the terminal jt runs in. Tests replace it.
var detect = func() (tint.Source, error) { return terminal.Detect() }

// Default returns the tint jt uses when no theme is picked: the terminal's
// active theme, or Dracula+ when jt can't read it. It reads the terminal's
// config, so call it off the render goroutine.
func Default(ctx context.Context) *tint.Tint {
	src, err := detect()
	if err != nil {
		return fallback
	}
	tints, err := src.Load(ctx)
	if err != nil || len(tints) == 0 {
		return fallback
	}
	// The registry holds a source's tints under its name, as Attach registers
	// them.
	if t, ok := tint.GetTint(src.Name() + ":" + tints[0].ID); ok {
		return t
	}
	return fallback
}

// Start follows the terminal's theme until ctx is done. A non-empty id picks
// that tint instead, until the user changes the terminal's theme. An error
// explains what Start fell back to; the styles work either way.
func Start(ctx context.Context, id string) error {
	var errs []error
	if src, err := detect(); err == nil {
		if _, err := tint.Attach(ctx, src); err != nil && !errors.Is(err, tint.ErrNotFound) {
			errs = append(errs, fmt.Errorf("following the terminal's theme: %w", err))
		}
	}
	if id != "" && !tint.SetTintID(id) {
		errs = append(errs, fmt.Errorf("unknown theme %q, using %s instead", id, tint.Current().DisplayName))
	}
	return errors.Join(errs...)
}

// form returns a huh theme drawn in a tint's colors.
func form(t *tint.Tint) *huh.Styles {
	s := huh.ThemeBase(t.Dark)
	fade := lipgloss.Blend1D(101, t.Bg, t.Fg)
	border, gray := fade[15], fade[70]

	s.Focused.Base = s.Focused.Base.BorderForeground(border)
	s.Focused.Card = s.Focused.Base
	s.Focused.Title = s.Focused.Title.Foreground(t.Blue).Bold(true)
	s.Focused.NoteTitle = s.Focused.NoteTitle.Foreground(t.Blue).Bold(true)
	s.Focused.Directory = s.Focused.Directory.Foreground(t.Cyan)
	s.Focused.File = s.Focused.File.Foreground(t.Fg)
	s.Focused.Description = s.Focused.Description.Foreground(gray)
	s.Focused.ErrorIndicator = s.Focused.ErrorIndicator.Foreground(t.Red)
	s.Focused.ErrorMessage = s.Focused.ErrorMessage.Foreground(t.Red)
	s.Focused.SelectSelector = s.Focused.SelectSelector.Foreground(t.Yellow)
	s.Focused.NextIndicator = s.Focused.NextIndicator.Foreground(t.Yellow)
	s.Focused.PrevIndicator = s.Focused.PrevIndicator.Foreground(t.Yellow)
	s.Focused.Option = s.Focused.Option.Foreground(t.Fg)
	s.Focused.MultiSelectSelector = s.Focused.MultiSelectSelector.Foreground(t.Yellow)
	s.Focused.SelectedOption = s.Focused.SelectedOption.Foreground(t.Green)
	s.Focused.SelectedPrefix = s.Focused.SelectedPrefix.Foreground(t.Green)
	s.Focused.UnselectedOption = s.Focused.UnselectedOption.Foreground(t.Fg)
	s.Focused.UnselectedPrefix = s.Focused.UnselectedPrefix.Foreground(gray)
	s.Focused.FocusedButton = s.Focused.FocusedButton.Foreground(t.Bg).Background(t.Blue).Bold(true)
	s.Focused.BlurredButton = s.Focused.BlurredButton.Foreground(t.Fg).Background(border)
	s.Focused.Next = s.Focused.FocusedButton

	s.Focused.TextInput.Cursor = s.Focused.TextInput.Cursor.Foreground(t.Yellow)
	s.Focused.TextInput.Placeholder = s.Focused.TextInput.Placeholder.Foreground(gray)
	s.Focused.TextInput.Prompt = s.Focused.TextInput.Prompt.Foreground(t.Yellow)
	s.Focused.TextInput.Text = s.Focused.TextInput.Text.Foreground(t.Fg)

	s.Blurred = s.Focused
	s.Blurred.Base = s.Blurred.Base.BorderStyle(lipgloss.HiddenBorder())
	s.Blurred.Card = s.Blurred.Base
	s.Blurred.NextIndicator = lipgloss.NewStyle()
	s.Blurred.PrevIndicator = lipgloss.NewStyle()

	s.Group.Title = s.Focused.Title
	s.Group.Description = s.Focused.Description
	return s
}
