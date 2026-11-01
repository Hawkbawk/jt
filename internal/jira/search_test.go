package jira

import (
	"testing"
	"time"
)

func TestSearchQueryJQL(t *testing.T) {
	day := time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		q    SearchQuery
		want string
	}{
		{"empty", SearchQuery{}, ""},
		{"only sort", SearchQuery{Sort: SortCreated}, "ORDER BY created ASC"},
		{
			"everything",
			SearchQuery{
				Project: "DEMO", IssueType: "Bug", Sprint: SprintActive, Keys: []string{"DEMO-1", "DEMO-2"},
				Created: TimeRange{From: day}, Updated: TimeRange{To: day.Add(23*time.Hour + 59*time.Minute)},
				Sort: SortUpdated, Descending: true,
			},
			`project = "DEMO" AND issuetype = "Bug" AND sprint in openSprints() AND key in (DEMO-1, DEMO-2) AND created >= "2026-03-04 00:00" AND updated <= "2026-03-04 23:59" ORDER BY updated DESC`,
		},
		{"not in sprint", SearchQuery{Sprint: SprintNotActive, Keys: []string{"A-1"}}, "sprint not in openSprints() AND key = A-1"},
		{"quotes", SearchQuery{IssueType: `Sub "task"`}, `issuetype = "Sub \"task\""`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.q.String(); got != tt.want {
				t.Errorf("JQL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseKeys(t *testing.T) {
	got, err := ParseKeys("demo-1, DEMO-22 abc-3")
	if err != nil || len(got) != 3 || got[0] != "DEMO-1" || got[2] != "ABC-3" {
		t.Errorf("ParseKeys = %v, %v", got, err)
	}
	if _, err := ParseKeys("DEMO-1, nope"); err == nil {
		t.Error("expected an error for a bad key")
	}
	if got, err := ParseKeys("  "); err != nil || got != nil {
		t.Errorf("blank = %v, %v", got, err)
	}
}
