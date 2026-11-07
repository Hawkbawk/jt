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
	p.Update(p.Init()().(tea.BatchMsg)[0]())
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

func TestLoadReportsProjectsToHost(t *testing.T) {
	p := New(fakeSource{}, nil, true)
	_, cmd := p.Update(p.Init()().(tea.BatchMsg)[0]())
	if msg, ok := cmd().(LoadedMsg); !ok || len(msg.Projects) != 2 {
		t.Fatalf("host was given %v", cmd())
	}
}

func TestFavoriteReportsHighlightedProject(t *testing.T) {
	p := loaded(t, fakeSource{})
	_, cmd := p.Update(tea.KeyPressMsg{Code: 'f', Text: "f"})
	if cmd == nil {
		t.Fatal("f did nothing")
	}
	if got, ok := cmd().(FavoriteMsg); !ok || got.Key != "ALPHA" {
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
