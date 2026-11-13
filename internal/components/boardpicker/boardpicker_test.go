package boardpicker

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/tui/modal"
)

type source []jira.BoardSummary

func (s source) Boards(string) ([]jira.BoardSummary, error) { return s, nil }

func loaded(t *testing.T) *Picker {
	t.Helper()
	p := New(source{{ID: 1, Name: "One"}, {ID: 2, Name: "Two"}}, "A", 2)
	p.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	p.Update(p.Init()().(tea.BatchMsg)[0]())
	return p
}

func TestStartsOnCurrentBoard(t *testing.T) {
	p := loaded(t)
	if !p.Loaded() || p.Len() != 2 {
		t.Fatalf("loaded=%v len=%d", p.Loaded(), p.Len())
	}
	_, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	sel, ok := cmd().(SelectedMsg)
	if !ok || sel.Board.ID != 2 || sel.Project != "A" {
		t.Fatalf("selected = %+v", sel)
	}
}

func TestQAndEscapeClose(t *testing.T) {
	for _, k := range []tea.KeyPressMsg{{Code: 'q', Text: "q"}, {Code: tea.KeyEsc}} {
		p := loaded(t)
		_, cmd := p.Update(k)
		if cmd == nil {
			t.Fatalf("%v should close the picker", k)
		}
		if _, ok := cmd().(modal.Closes); !ok {
			t.Fatalf("%v should close the picker, got %T", k, cmd())
		}
	}
}
