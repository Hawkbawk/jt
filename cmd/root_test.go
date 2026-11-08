package cmd

import (
	"reflect"
	"testing"

	"github.com/hawkbawk/jt/internal/config"
	"github.com/hawkbawk/jt/internal/fakejira"
)

func TestCommandsForSession(t *testing.T) {
	configured := []config.Command{{Name: "Configured", Command: "custom"}}
	if got := commandsForSession(false, configured); !reflect.DeepEqual(got, configured) {
		t.Fatalf("configured commands = %#v, want %#v", got, configured)
	}
	if got, want := commandsForSession(true, configured), fakejira.DemoCommands(); !reflect.DeepEqual(got, want) {
		t.Fatalf("fake commands = %#v, want %#v", got, want)
	}
}
