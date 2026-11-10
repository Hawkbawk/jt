package themepicker

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	tint "github.com/lrstanley/bubbletint/v2"

	"github.com/hawkbawk/jt/internal/icons"
)

func TestListsEveryTintAndStartsOnCurrent(t *testing.T) {
	prev := tint.Current()
	t.Cleanup(func() { tint.SetTint(prev) })
	tint.SetTint(tint.TintNord)

	p := New()
	if got, want := p.Len(), len(tint.Tints()); got != want {
		t.Errorf("listed %d tints, want all %d", got, want)
	}
	sel, _ := p.list.SelectedItem().(item)
	if p.Selected() != tint.TintNord || !strings.HasSuffix(sel.Title(), icons.Check) {
		t.Errorf("selected = %q, want the current tint, checked", sel.Title())
	}
}

func TestApplyClosesWithSelection(t *testing.T) {
	p := New()
	_, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter should close the popup with the tint")
	}
}

func TestResetReadsTerminalThemeInCommand(t *testing.T) {
	p := New()
	_, cmd := p.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if cmd == nil {
		t.Fatal("r should return a command to read the terminal's theme")
	}
	if _, cmd := p.Update(resetMsg{tint: tint.TintNord}); cmd == nil {
		t.Fatal("the terminal's answer should close the popup")
	}
}
