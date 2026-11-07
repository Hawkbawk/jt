package widget

import (
	"testing"

	"charm.land/bubbles/v2/spinner"
)

func TestSpinnerTicksOnlyWhileBusy(t *testing.T) {
	s := NewSpinner()
	if s.Sync(false) != nil {
		t.Fatal("an idle spinner started")
	}
	start := s.Sync(true)
	if start == nil {
		t.Fatal("a busy spinner didn't start")
	}
	if s.Sync(true) != nil {
		t.Fatal("a ticking spinner started a second chain")
	}
	tick := start().(spinner.TickMsg)
	if s.Update(tick, true) == nil {
		t.Fatal("a busy spinner stopped ticking")
	}
	if s.Update(spinner.TickMsg{ID: s.ID() + 1}, false) != nil || !s.ticking {
		t.Fatal("another spinner's tick changed this one")
	}
	if s.Update(tick, false) != nil {
		t.Fatal("an idle spinner kept ticking")
	}
	if s.Sync(true) == nil {
		t.Fatal("a spinner that stopped didn't start again")
	}
}
