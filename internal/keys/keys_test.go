package keys

import (
	"slices"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
)

func TestArrowLabels(t *testing.T) {
	t.Cleanup(func() { Set(Default()) })

	d := Default()
	if got := d.Scroll.Help().Key; got != "↑↓" {
		t.Fatalf("scroll label = %q", got)
	}
	if d.ListUp.Help().Key != "↑" || d.ListDown.Help().Key != "↓" {
		t.Fatalf("list labels = %q, %q", d.ListUp.Help().Key, d.ListDown.Help().Key)
	}

	d.Up = key.NewBinding(key.WithKeys("w"))
	if got := d.Derive().ListUp.Help().Key; got != "w" {
		t.Fatalf("a rebind without an arrow should show its key, got %q", got)
	}
}

func TestCommandAndCommentBindings(t *testing.T) {
	m := Default()
	if !slices.Contains(m.Commands.Keys(), "C") || !slices.Contains(m.Comments.Keys(), "c") {
		t.Fatalf("commands keys = %v, comments keys = %v", m.Commands.Keys(), m.Comments.Keys())
	}
}

func TestSetRefreshesDerivedBindings(t *testing.T) {
	t.Cleanup(func() { Set(Default()) })

	m := Default()
	m.Quit = key.NewBinding(key.WithKeys("Q"), key.WithHelp("q", "quit"))
	m.Accept = key.NewBinding(key.WithKeys("space"), key.WithHelp("enter", "accept"))
	m.Down = key.NewBinding(key.WithKeys("n"))
	Set(m)

	got := Get()
	if !slices.Contains(got.Close.Keys(), "Q") || slices.Contains(got.Close.Keys(), "q") {
		t.Fatalf("Close should follow Quit, has %v", got.Close.Keys())
	}

	if !slices.Contains(got.ViewResult.Keys(), "space") {
		t.Fatalf("ViewResult should follow Accept, has %v", got.ViewResult.Keys())
	}
	if got.Accept.Help().Key != "space" {
		t.Fatalf("help should show the rebound key, got %q", got.Accept.Help().Key)
	}
	if got.ListNavigate.Help().Key != "n/k" {
		t.Fatalf("summary label should follow its bindings, got %q", got.ListNavigate.Help().Key)
	}
	if d := Describe(got.ListNavigate, "select"); d.Help().Key != "n/k" || d.Help().Desc != "select" {
		t.Fatalf("Describe should keep the label, got %+v", d.Help())
	}
}

func TestWithOverridesPreservesDefaultsAndDerives(t *testing.T) {
	m, err := Default().WithOverrides(map[string]any{
		"accept":  "space",
		"dismiss": []any{"...", "d"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(m.Accept.Keys(), []string{"space"}) {
		t.Errorf("accept keys = %v", m.Accept.Keys())
	}
	if !slices.Equal(m.Dismiss.Keys(), []string{"esc", "d"}) {
		t.Errorf("dismiss keys = %v", m.Dismiss.Keys())
	}
	if !slices.Contains(m.Close.Keys(), "d") || !slices.Contains(m.Close.Keys(), "q") {
		t.Errorf("Close should include custom dismiss and default quit keys, got %v", m.Close.Keys())
	}
	if !slices.Contains(m.View.Keys(), "v") {
		t.Errorf("untouched view keys = %v, want default", m.View.Keys())
	}
	if m.Accept.Help().Key != "space" {
		t.Errorf("accept help key = %q, want space", m.Accept.Help().Key)
	}
}

func TestWithOverridesInvalidEntriesKeepDefaults(t *testing.T) {
	m, err := Default().WithOverrides(map[string]any{
		"acceppt": "space",
		"accept":  []any{"space", "not-a-key"},
		"dismiss": []any{"...", 42},
		"view":    "x",
	})
	if err == nil {
		t.Fatal("expected warnings")
	}
	for _, want := range []string{"acceppt", "keys.accept", "not-a-key", "keys.dismiss"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("warning %q missing %q", err, want)
		}
	}
	if !slices.Equal(m.Accept.Keys(), []string{"enter"}) ||
		!slices.Equal(m.Dismiss.Keys(), []string{"esc"}) ||
		!slices.Equal(m.View.Keys(), []string{"x"}) {
		t.Fatalf("accept=%v dismiss=%v view=%v", m.Accept.Keys(), m.Dismiss.Keys(), m.View.Keys())
	}
}

func TestWithOverridesMarkerOrderAndDuplicates(t *testing.T) {
	m, err := Default().WithOverrides(map[string]any{
		"left": []any{"x", "...", "h", "..."},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(m.Left.Keys(), []string{"x", "h", "left"}) {
		t.Fatalf("left = %v", m.Left.Keys())
	}
}

func TestWithOverridesInvalidSettings(t *testing.T) {
	for _, settings := range []any{"bad", map[string]any{"accept": []any{}}, map[string]any{"accept": nil}} {
		m, err := Default().WithOverrides(settings)
		if err == nil || !slices.Equal(m.Accept.Keys(), []string{"enter"}) {
			t.Errorf("settings=%v, accept=%v, err=%v", settings, m.Accept.Keys(), err)
		}
	}
}

func TestValidKey(t *testing.T) {
	for _, k := range []string{"x", "+", "é", "space", "enter", "shift+tab", "ctrl+alt+x", "f12", "ctrl++"} {
		if !validKey(k) {
			t.Errorf("valid key %q rejected", k)
		}
	}
	for _, k := range []string{"", " ", "ctrl+", "ctrl+ctrl+x", "wrong", "shift+ctrl+x"} {
		if validKey(k) {
			t.Errorf("invalid key %q accepted", k)
		}
	}
}
