package projectpicker

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/jira"
)

type fakeSource struct{ defaultKey string }

func (fakeSource) Projects() ([]jira.Project, error) {
	return []jira.Project{{Key: "ALPHA", Name: "Alpha Project"}, {Key: "BETA", Name: "Beta Project"}}, nil
}

func (f fakeSource) DefaultProject() string { return f.defaultKey }

func loaded(t *testing.T, src Source) *Picker {
	t.Helper()
	p := New(src, nil, true)
	p.Init()
	msg := p.load()()
	p.Update(msg)
	return p
}

func TestMarksDefaultProject(t *testing.T) {
	p := loaded(t, fakeSource{defaultKey: "BETA"})
	items := p.list.Items()
	if !items[1].(item).isDefault {
		t.Fatal("default marker not shown")
	}
	if items[0].(item).isDefault {
		t.Fatal("non-default project marked as default")
	}
}

func TestFavoriteReportsHighlightedProject(t *testing.T) {
	p := loaded(t, fakeSource{})
	_, cmd := p.Update(tea.KeyPressMsg{Code: 'f', Text: "f"})
	if cmd == nil {
		t.Fatal("f did nothing")
	}
	if got := cmd(); got != (FavoriteMsg{Key: "ALPHA"}) {
		t.Fatalf("got %#v, want FavoriteMsg for ALPHA", got)
	}
}

func TestEnterReportsSelection(t *testing.T) {
	p := loaded(t, fakeSource{})
	_, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter did nothing")
	}
}

func TestSelectPutsCursorOnProjectOnceLoaded(t *testing.T) {
	p := New(fakeSource{}, nil, true)
	p.Select("BETA")
	p.Init()
	p.Update(p.load()())
	if got := p.list.Index(); got != 1 {
		t.Fatalf("cursor at %d, want 1", got)
	}
	p.Select("NOPE")
	if got := p.list.Index(); got != 1 {
		t.Fatalf("unknown key moved the cursor to %d", got)
	}
}
