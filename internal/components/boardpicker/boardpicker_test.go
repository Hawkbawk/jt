package boardpicker

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/tui/popup"
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
	msg, ok := cmd().(popup.CloseMsg)
	if !ok {
		t.Fatalf("enter should close the popup, got %T", cmd())
	}
	sel, ok := msg.Then.(SelectedMsg)
	if !ok || sel.Board.ID != 2 || sel.Project != "A" {
		t.Fatalf("selected = %+v", msg.Then)
	}
}

func TestIgnoresAnotherPickersBoards(t *testing.T) {
	p := New(source{}, "A", 0)
	p.Update(LoadedMsg{ID: p.id + 1, Boards: []jira.BoardSummary{{ID: 1, Name: "One"}}})
	if p.Loaded() {
		t.Fatal("loaded boards meant for another picker")
	}
}
