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
