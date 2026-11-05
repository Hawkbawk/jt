package search

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/charmbracelet/x/ansi"

	"github.com/hawkbawk/jt/internal/jira"
)

func TestResultListTruncatesLongSummary(t *testing.T) {
	seed := uint64(time.Now().UnixNano())
	if env := os.Getenv("JT_TEST_SEED"); env != "" {
		var err error
		if seed, err = strconv.ParseUint(env, 10, 64); err != nil {
			t.Fatalf("JT_TEST_SEED: %v", err)
		}
	}
	t.Logf("faker seed: %d (set JT_TEST_SEED to replay)", seed)
	faker := gofakeit.New(seed)
	var long strings.Builder
	for long.Len() < 400 {
		long.WriteString(faker.Sentence())
		long.WriteByte(' ')
	}
	issues := []jira.Issue{
		{Key: "A-1", Type: "Bug", Status: "Open", Summary: "short"},
		{Key: "A-2", Type: "Task", Status: "In Progress", Summary: long.String()},
	}
	for width := 70; width <= 200; width++ {
		var l resultList
		l.resize(width, 10)
		l.set(issues)
		lines := strings.Split(ansi.Strip(l.view()), "\n")
		if len(lines) != 4 {
			t.Fatalf("width %d: got %d lines, want header, separator and 2 rows", width, len(lines))
		}
		for i, line := range lines {
			if got := ansi.StringWidth(line); got > width {
				t.Fatalf("width %d: line %d is %d cells wide: %q", width, i, got, line)
			}
		}
		if !strings.Contains(lines[3], "…") {
			t.Fatalf("width %d: long summary not truncated: %q", width, lines[3])
		}
	}
}

func TestResultListRestore(t *testing.T) {
	issues := []jira.Issue{{Key: "A-1"}, {Key: "A-2"}, {Key: "A-3"}}
	tests := []struct {
		name  string
		after []jira.Issue
		key   string
		index int
		want  string
	}{
		{"follows the issue", []jira.Issue{{Key: "A-3"}, {Key: "A-1"}, {Key: "A-2"}}, "A-2", 1, "A-2"},
		{"issue gone keeps the row", []jira.Issue{{Key: "A-1"}, {Key: "A-3"}}, "A-2", 1, "A-3"},
		{"issue gone at the end clamps", []jira.Issue{{Key: "A-1"}, {Key: "A-2"}}, "A-3", 2, "A-2"},
		{"no results", nil, "A-1", 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var l resultList
			l.resize(100, 10)
			l.set(issues)
			l.set(tt.after)
			l.restore(tt.key, tt.index)
			got := ""
			if sel := l.selected(); sel != nil {
				got = sel.Key
			}
			if got != tt.want {
				t.Errorf("selected = %q, want %q", got, tt.want)
			}
		})
	}
}
