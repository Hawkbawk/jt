package keys

import (
	"slices"
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
