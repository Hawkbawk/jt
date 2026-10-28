package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tint "github.com/lrstanley/bubbletint/v2"

	"github.com/hawkbawk/jt/internal/styles"
)

// useTint makes t current for the test and restores the previous tint after.
func useTint(t *testing.T, tt *tint.Tint) {
	t.Helper()
	prev := tint.Current()
	t.Cleanup(func() { tint.SetTint(prev) })
	tint.SetTint(tt)
}

func openThemePopup(t *testing.T, fs *fakeSettings) (Model, *themePopup) {
	t.Helper()
	m := newTestModel(t, "TEST", &fakeBackend{})
	m.settings = fs
	m = drain(t, m, m.Init())
	m, _ = press(t, m, "t")
	p, ok := m.popup.(*themePopup)
	if !ok {
		t.Fatalf("t should open the theme popup, got %T", m.popup)
	}
	return m, p
}

func TestThemePopupListsEveryTintAndStartsOnCurrent(t *testing.T) {
	useTint(t, tint.TintNord)

	_, p := openThemePopup(t, &fakeSettings{})
	if got, want := len(p.list.Items()), len(tint.Tints()); got != want {
		t.Errorf("listed %d tints, want all %d", got, want)
	}
	sel, _ := p.list.SelectedItem().(tintItem)
	if sel.Tint != tint.TintNord || !strings.HasSuffix(sel.Title(), "✓") {
		t.Errorf("selected = %q, want the current tint, checked", sel.Title())
	}
}

func TestThemePopupFiltersAppliesAndSaves(t *testing.T) {
	useTint(t, tint.TintNord)

	fs := &fakeSettings{}
	m, _ := openThemePopup(t, fs)
	m = pressDrain(t, m, append(append([]string{"/"}, strings.Split("catppuccin_mocha", "")...), "enter", "enter")...)

	if m.popup != nil || styles.Current() != tint.TintCatppuccinMocha {
		t.Fatalf("popup=%v current=%v, want closed with Catppuccin Mocha applied", m.popup, styles.Current().ID)
	}
	if len(fs.saved) != 1 || fs.saved[0] != "catppuccin_mocha" {
		t.Fatalf("saved = %v", fs.saved)
	}
}

func TestThemePopupEscClearsFilterBeforeClosing(t *testing.T) {
	useTint(t, tint.TintNord)

	fs := &fakeSettings{}
	m, _ := openThemePopup(t, fs)
	m = pressDrain(t, m, "/", "x", "enter", "esc")
	p, ok := m.popup.(*themePopup)
	if !ok || p.list.IsFiltered() {
		t.Fatalf("the first esc should clear the filter and keep the popup open, popup=%T", m.popup)
	}
	m = pressDrain(t, m, "esc")
	if m.popup != nil || styles.Current() != tint.TintNord || len(fs.saved) != 0 {
		t.Fatalf("esc should close without changing the theme, popup=%T saved=%v", m.popup, fs.saved)
	}
}

func TestThemePopupResetsToTerminal(t *testing.T) {
	useTint(t, tint.TintNord)

	fs := &fakeSettings{}
	m, _ := openThemePopup(t, fs)
	m = pressDrain(t, m, "/", "r")
	if p, ok := m.popup.(*themePopup); !ok || p.list.FilterValue() != "r" {
		t.Fatalf("r while filtering should type into the filter, popup=%T", m.popup)
	}

	m = pressDrain(t, m, "esc")
	m, cmd := press(t, m, "r")
	if m.popup != nil || styles.Current() != tint.TintNord || cmd == nil {
		t.Fatal("r should close the popup and read the terminal's theme in a command, not in Update")
	}
	m = drain(t, m, cmd)
	if styles.Current() != styles.Default(context.Background()) {
		t.Fatalf("current = %v, want the default tint", styles.Current().ID)
	}
	if len(fs.saved) != 1 || fs.saved[0] != "" {
		t.Fatalf("saved = %q, want an empty theme so jt follows the terminal", fs.saved)
	}
}

func TestThemeSaveErrorShowsInStatus(t *testing.T) {
	useTint(t, tint.TintNord)

	m, _ := openThemePopup(t, &fakeSettings{err: errors.New("read-only")})
	m = pressDrain(t, m, "j", "enter")
	if m.status == "" {
		t.Fatal("expected the save error in the status line")
	}
}

func TestThemeChangesRestyleEverything(t *testing.T) {
	useTint(t, tint.TintNord)

	m := newTestModel(t, "TEST", &fakeBackend{})
	m = drain(t, m, m.Init())

	next, _ := m.Update(themeSelectedMsg{tint: tint.TintGruvboxLightHard})
	m = next.(Model)
	if m.spinner.Style.GetForeground() != tint.TintGruvboxLightHard.Blue {
		t.Error("picking a tint should restyle the spinner")
	}

	// The terminal's theme changing arrives as a ThemeChangedMsg.
	tint.SetTint(tint.TintCatppuccinMocha)
	next, _ = m.Update(ThemeChangedMsg{})
	m = next.(Model)
	if m.spinner.Style.GetForeground() != tint.TintCatppuccinMocha.Blue {
		t.Error("a tint change from outside jt should restyle the spinner")
	}
	if v := m.View(); v.BackgroundColor != tint.TintCatppuccinMocha.Bg || v.ForegroundColor != tint.TintCatppuccinMocha.Fg {
		t.Error("the view should paint the tint's background and foreground")
	}
}
