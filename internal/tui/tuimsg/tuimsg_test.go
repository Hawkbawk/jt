package tuimsg

import (
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
)

type (
	before struct{}
	after  struct{}
)

func swap(msg tea.Msg) tea.Msg {
	if _, ok := msg.(before); ok {
		return after{}
	}
	return msg
}

func TestMapChangesMessagesIncludingInBatches(t *testing.T) {
	if Map(nil, swap) != nil {
		t.Fatal("a nil command should stay nil")
	}
	if got := Map(Send(before{}), swap)(); got != (after{}) {
		t.Fatalf("got %#v", got)
	}
	if got := Map(Send(Closed{}), swap)(); got != (Closed{}) {
		t.Fatalf("unmatched message changed to %#v", got)
	}
	batch, ok := Map(tea.Batch(Send(before{}), Send(Closed{})), swap)().(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatalf("expected a batch of two, got %#v", batch)
	}
	if batch[0]() != (after{}) || batch[1]() != (Closed{}) {
		t.Fatal("batch messages not mapped")
	}
}

func TestMapChangesMessagesInSequences(t *testing.T) {
	msg := Map(tea.Sequence(Send(before{}), Send(Closed{})), swap)()
	v := reflect.ValueOf(msg)
	if v.Kind() != reflect.Slice || v.Len() != 2 {
		t.Fatalf("expected a sequence of two, got %#v", msg)
	}
	first := v.Index(0).Interface().(tea.Cmd)()
	second := v.Index(1).Interface().(tea.Cmd)()
	if first != (after{}) || second != (Closed{}) {
		t.Fatalf("sequence messages not mapped: %#v, %#v", first, second)
	}
}
