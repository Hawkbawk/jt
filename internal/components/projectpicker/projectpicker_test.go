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
	msg := Load(src, p.id)()
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

func TestIgnoresOtherPickersResponses(t *testing.T) {
	p := New(fakeSource{}, nil, true)
	p.Update(LoadedMsg{ID: p.id + 1, Projects: []jira.Project{{Key: "X"}}})
	if p.Loaded() {
		t.Fatal("picker used a response meant for another")
	}
}

func TestEnterReportsSelection(t *testing.T) {
	p := loaded(t, fakeSource{})
	_, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter did nothing")
	}
}
