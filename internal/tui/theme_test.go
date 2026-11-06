package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tint "github.com/lrstanley/bubbletint/v2"

	"github.com/hawkbawk/jt/internal/components/themepicker"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
	"github.com/hawkbawk/jt/internal/tui/tuitest"
)

// useTint makes t current for the test and restores the previous tint after.
func useTint(t *testing.T, tt *tint.Tint) {
	t.Helper()
	prev := tint.Current()
	t.Cleanup(func() { tint.SetTint(prev) })
	tint.SetTint(tt)
}

func openThemePopup(t *testing.T, fs *tuitest.Settings) BoardModel {
	t.Helper()
	m := newTestModel(t, "TEST", &tuitest.Backend{})
	m.Settings = fs
	m = tuitest.Drain(t, m, m.Init())
	m = tuitest.PressDrain(t, m, "t")
	if _, ok := m.Popup.Content().(*themepicker.Picker); !ok {
		t.Fatalf("t should open the theme popup, got %T", m.Popup.Content())
	}
	return m
}

func TestThemePopupFiltersAppliesAndSaves(t *testing.T) {
	useTint(t, tint.TintNord)

	fs := &tuitest.Settings{}
	m := openThemePopup(t, fs)
	m = tuitest.PressDrain(t, m, append(append([]string{"/"}, strings.Split("catppuccin_mocha", "")...), "enter", "enter")...)

	if m.Popup.IsOpen() || styles.Current() != tint.TintCatppuccinMocha {
		t.Fatalf("popup=%v current=%v, want closed with Catppuccin Mocha applied", m.Popup.Content(), styles.Current().ID)
	}
	if len(fs.Saved) != 1 || fs.Saved[0] != "catppuccin_mocha" {
		t.Fatalf("saved = %v", fs.Saved)
	}
}

func TestThemePopupEscClearsFilterBeforeClosing(t *testing.T) {
	useTint(t, tint.TintNord)

	fs := &tuitest.Settings{}
	m := openThemePopup(t, fs)
	m = tuitest.PressDrain(t, m, "/", "x", "enter", "esc")
	p, ok := m.Popup.Content().(*themepicker.Picker)
	if !ok || p.Filtered() {
		t.Fatalf("the first esc should clear the filter and keep the popup open, popup=%T", m.Popup.Content())
	}
	m = tuitest.PressDrain(t, m, "esc")
	if m.Popup.IsOpen() || styles.Current() != tint.TintNord || len(fs.Saved) != 0 {
		t.Fatalf("esc should close without changing the theme, popup=%T saved=%v", m.Popup.Content(), fs.Saved)
	}
}

func TestThemePopupResetsToTerminal(t *testing.T) {
	useTint(t, tint.TintNord)

	fs := &tuitest.Settings{}
	m := openThemePopup(t, fs)
	m = tuitest.PressDrain(t, m, "/", "r")
	if p, ok := m.Popup.Content().(*themepicker.Picker); !ok || !p.Filtering() {
		t.Fatalf("r while filtering should type into the filter, popup=%T", m.Popup.Content())
	}

	m = tuitest.PressDrain(t, m, "esc")
	m, cmd := press(t, m, "r")
	if styles.Current() != tint.TintNord || cmd == nil {
		t.Fatal("r should read the terminal's theme in a command, not in Update")
	}
	m = tuitest.Drain(t, m, cmd)
	if m.Popup.IsOpen() {
		t.Fatal("r should close the popup")
	}
	if styles.Current() != styles.Default(context.Background()) {
		t.Fatalf("current = %v, want the default tint", styles.Current().ID)
	}
	if len(fs.Saved) != 1 || fs.Saved[0] != "" {
		t.Fatalf("saved = %q, want an empty theme so jt follows the terminal", fs.Saved)
	}
}

func TestThemeSaveErrorShowsInStatus(t *testing.T) {
	useTint(t, tint.TintNord)

	m := openThemePopup(t, &tuitest.Settings{Err: errors.New("read-only")})
	m = tuitest.PressDrain(t, m, "j", "enter")
	if m.Status == "" {
		t.Fatal("expected the save error in the status line")
	}
}

func TestThemeChangesRestyleEverything(t *testing.T) {
	useTint(t, tint.TintNord)

	m := newTestModel(t, "TEST", &tuitest.Backend{})
	m = tuitest.Drain(t, m, m.Init())

	next, _ := m.Update(themepicker.SelectedMsg{Tint: tint.TintGruvboxLightHard})
	m = next.(BoardModel)
	if m.Spinner.Style.GetForeground() != tint.TintGruvboxLightHard.Blue {
		t.Error("picking a tint should restyle the spinner")
	}

	// The terminal's theme changing arrives as a tuimsg.ThemeChanged.
	tint.SetTint(tint.TintCatppuccinMocha)
	next, _ = m.Update(tuimsg.ThemeChanged{})
	m = next.(BoardModel)
	if m.Spinner.Style.GetForeground() != tint.TintCatppuccinMocha.Blue {
		t.Error("a tint change from outside jt should restyle the spinner")
	}
	if v := m.View(); v.BackgroundColor != tint.TintCatppuccinMocha.Bg || v.ForegroundColor != tint.TintCatppuccinMocha.Fg {
		t.Error("the view should paint the tint's background and foreground")
	}
}
