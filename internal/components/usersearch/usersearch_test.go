package usersearch

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/jira"
)

func TestSearchSkipsSupersededQueries(t *testing.T) {
	var searches []string
	f := New("People", func(query string) ([]jira.Option, error) {
		searches = append(searches, query)
		return []jira.Option{{ID: "u2", Name: "Bo"}}, nil
	}, List{Rows: func(_ string, found []jira.Option) []jira.Option { return found }})
	f.Focus()
	for _, r := range "bo" {
		f.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	if _, cmd := f.Update(searchMsg{field: f, seq: 1}); cmd != nil {
		t.Fatal("the query replaced by a later keystroke should not be sent")
	}
	_, cmd := f.Update(searchMsg{field: f, seq: 2})
	f.Update(cmd())
	if !slices.Equal(searches, []string{"bo"}) {
		t.Errorf("searches = %v", searches)
	}
	if len(f.rows) != 1 || f.rows[0].Name != "Bo" {
		t.Errorf("rows = %v", f.rows)
	}
}
