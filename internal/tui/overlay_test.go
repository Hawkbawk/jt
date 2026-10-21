package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestOverlayCentersForeground(t *testing.T) {
	bg := strings.Join([]string{"..........", "..........", "..........", ".........."}, "\n")
	got := ansi.Strip(overlay(bg, "ab\ncd", 10))
	want := strings.Join([]string{"..........", "....ab....", "....cd....", ".........."}, "\n")
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestOverlayPadsShortBackgroundLines(t *testing.T) {
	got := ansi.Strip(overlay("\n\n", "x", 5))
	if lines := strings.Split(got, "\n"); lines[1] != "  x" {
		t.Fatalf("got %q", lines[1])
	}
}
