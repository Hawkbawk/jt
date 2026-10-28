package tui

import (
	"errors"
	"testing"

	"github.com/hawkbawk/jt/internal/config"
	"github.com/hawkbawk/jt/internal/styles"
)

func TestThemePopupAppliesAndSaves(t *testing.T) {
	t.Cleanup(func() { styles.Use(config.ThemeTerminal) })
	styles.Use(config.ThemeTerminal)

	fs := &fakeSettings{}
	m := newTestModel(t, "TEST", &fakeBackend{})
	m.settings = fs
	m = drain(t, m, m.Init())

	m, _ = press(t, m, "t")
	if _, ok := m.popup.(*themePopup); !ok {
		t.Fatalf("t should open the theme popup, got %T", m.popup)
	}
	m, _ = press(t, m, "j")
	m = pressDrain(t, m, "enter")

	want := config.Themes()[1]
	if m.popup != nil || styles.Current() != want {
		t.Fatalf("popup=%v current=%v, want closed with %v applied", m.popup, styles.Current(), want)
	}
	if len(fs.saved) != 1 || fs.saved[0] != want {
		t.Fatalf("saved = %v", fs.saved)
	}
}

func TestThemeSaveErrorShowsInStatus(t *testing.T) {
	t.Cleanup(func() { styles.Use(config.ThemeTerminal) })

	fs := &fakeSettings{err: errors.New("read-only")}
	m := newTestModel(t, "TEST", &fakeBackend{})
	m.settings = fs
	m = drain(t, m, m.Init())

	m, _ = press(t, m, "t", "j")
	m = pressDrain(t, m, "enter")
	if m.status == "" {
		t.Fatal("expected the save error in the status line")
	}
}
