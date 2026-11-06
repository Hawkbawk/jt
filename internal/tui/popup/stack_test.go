package popup

import (
	"strings"
	"testing"
)

func TestStackPinsFooterToBottom(t *testing.T) {
	lines := strings.Split(Stack(6, []string{"title", ""}, []string{"a"}, "footer"), "\n")
	if len(lines) != 6 || lines[0] != "title" || lines[1] != "" || lines[5] != "footer" {
		t.Fatalf("unexpected layout: %q", lines)
	}
}

func TestStackClipsLongBody(t *testing.T) {
	body := []string{"1", "2", "3", "4", "5", "6"}
	lines := strings.Split(Stack(5, []string{"title"}, body, "footer"), "\n")
	if len(lines) != 5 || lines[3] != "3" || lines[4] != "footer" {
		t.Fatalf("body should clip to the space between header and footer: %q", lines)
	}
}

func TestFitHeight(t *testing.T) {
	for _, tc := range []struct {
		in     string
		height int
		want   string
	}{
		{"a\nb\nc", 2, "a\nb"},
		{"a\nb", 2, "a\nb"},
		{"a", 3, "a\n\n"},
		{"a\n\nb\n", 3, "a\n\nb"},
		{"a", 0, ""},
	} {
		if got := fitHeight(tc.in, tc.height); got != tc.want {
			t.Errorf("fitHeight(%q, %d) = %q, want %q", tc.in, tc.height, got, tc.want)
		}
	}
}
