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
		{"text", SearchQuery{Text: `login "bug"`, Project: "DEMO"}, `textfields ~ "login \"bug\"" AND project = "DEMO"`},
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

func TestParseSearchText(t *testing.T) {
	tests := []struct {
		in, project, key, text string
		wantErr                bool
	}{
		{" 12345 ", "demo", "DEMO-12345", "", false},
		{"12345", "", "", "", true},
		{"ops-7", "DEMO", "OPS-7", "", false},
		{"login page", "DEMO", "", "login page", false},
		{"DEMO-1 broken", "DEMO", "", "DEMO-1 broken", false},
		{"", "DEMO", "", "", false},
	}
	for _, tt := range tests {
		key, text, err := ParseSearchText(tt.in, tt.project)
		if key != tt.key || text != tt.text || (err != nil) != tt.wantErr {
			t.Errorf("ParseSearchText(%q, %q) = %q, %q, %v", tt.in, tt.project, key, text, err)
		}
	}
}

func TestParseKey(t *testing.T) {
	tests := []struct {
		in, project, want string
		wantErr           bool
	}{
		{in: "demo-12", want: "DEMO-12"},
		{in: " 12 ", project: "ops", want: "OPS-12"},
		{in: "12", wantErr: true},
		{in: "0", project: "OPS", wantErr: true},
		{in: "DEMO-1 DEMO-2", wantErr: true},
		{in: "nope", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, tt := range tests {
		got, err := ParseKey(tt.in, tt.project)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("ParseKey(%q, %q) = %q, %v", tt.in, tt.project, got, err)
		}
	}
}
