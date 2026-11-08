package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/config"
	"github.com/hawkbawk/jt/internal/fakejira"
	"github.com/hawkbawk/jt/internal/keys"
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

func TestConfiguredKeysKeepWorkingAfterInvalidEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jt.yaml")
	if err := os.WriteFile(path, []byte("keys:\n  accept: space\n  dismiss: ['...', d]\n  view: nonexistent\n  unknown: x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JT_CONFIG_FILE", path)
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { keys.Set(keys.Default()) })
	err = configureKeys(cfg.Keys)
	if err == nil || !strings.Contains(err.Error(), "keys.view") || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("warning = %v", err)
	}
	k := keys.Get()
	if !key.Matches(tea.KeyPressMsg{Code: tea.KeySpace}, k.Accept) ||
		!key.Matches(tea.KeyPressMsg{Code: tea.KeyEsc}, k.Dismiss) ||
		!key.Matches(tea.KeyPressMsg{Code: 'd', Text: "d"}, k.Dismiss) ||
		!key.Matches(tea.KeyPressMsg{Code: 'v', Text: "v"}, k.View) {
		t.Fatalf("accept=%v dismiss=%v view=%v", k.Accept.Keys(), k.Dismiss.Keys(), k.View.Keys())
	}
}

func TestStartupWarningIncludesKeyAndThemeWarnings(t *testing.T) {
	oldKeysErr, oldThemeErr := keysErr, themeErr
	t.Cleanup(func() { keysErr, themeErr = oldKeysErr, oldThemeErr })
	keysErr = errors.New("bad key")
	themeErr = errors.New("bad theme")
	if got := startupWarning(); !strings.Contains(got, "bad key") || !strings.Contains(got, "bad theme") {
		t.Fatalf("warning = %q", got)
	}
}
