package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

func load(t *testing.T, contents string) (Config, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "jt.yml")
	if contents != "" {
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("JT_CONFIG_FILE", path)
	return Load()
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name, contents string
		want           string
		wantErr        bool
	}{
		{"missing file follows the terminal", "", "", false},
		{"missing theme follows the terminal", "other: 1\n", "", false},
		{"tint ID", "theme: gruvbox_dark_hard\n", "gruvbox_dark_hard", false},
		{"malformed", "theme: [\n", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := load(t, tt.contents)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && cfg.Theme != tt.want {
				t.Errorf("theme = %v, want %v", cfg.Theme, tt.want)
			}
		})
	}
}

func TestLoadCommands(t *testing.T) {
	cfg, err := load(t, `commands:
  - name: "Open issue"
    command: code
    args:
      - "{{.Key}}"
      - "{{.URL}}"
  - command: gh
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Commands) != 2 {
		t.Fatalf("commands = %#v", cfg.Commands)
	}
	if got := cfg.Commands[0]; got.DisplayName() != "Open issue" || got.Command != "code" || !slices.Equal(got.Args, []string{"{{.Key}}", "{{.URL}}"}) {
		t.Errorf("first command = %#v", got)
	}
	if got := cfg.Commands[1].DisplayName(); got != "gh" || cfg.Commands[1].Command != "gh" {
		t.Errorf("unnamed command display name = %q, want gh", got)
	}
}

func TestLoadKeyBindings(t *testing.T) {
	cfg, err := load(t, `keys:
  accept: space
  dismiss:
    - ...
    - d
`)
	if err != nil {
		t.Fatal(err)
	}
	bindings, ok := cfg.Keys.(map[string]any)
	if !ok {
		t.Fatalf("keys = %#v", cfg.Keys)
	}
	if bindings["accept"] != "space" {
		t.Errorf("accept keys = %v", bindings["accept"])
	}
	if got, ok := bindings["dismiss"].([]any); !ok || !slices.Equal(got, []any{"...", "d"}) {
		t.Errorf("dismiss keys = %v", bindings["dismiss"])
	}
}

func TestLoadMalformedKeyBindingsWithoutFailing(t *testing.T) {
	cfg, err := load(t, "keys: invalid\njira:\n  project: ABC\n")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Keys != "invalid" || cfg.Jira.Project != "ABC" {
		t.Fatalf("keys = %v, project = %s", cfg.Keys, cfg.Jira.Project)
	}
}

func TestLoadCreatesDefaultFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "jt.yml")
	t.Setenv("JT_CONFIG_FILE", path)

	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(b), `theme: ""`) {
		t.Fatalf("default file = %q, %v", b, err)
	}
}

func TestSaveThemeKeepsOtherKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jt.yml")
	if err := os.WriteFile(path, []byte("theme: charm\nother: kept\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JT_CONFIG_FILE", path)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveTheme("gruvbox_dark_hard"); err != nil {
		t.Fatal(err)
	}

	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), "other: kept") || !strings.Contains(string(b), "theme: gruvbox_dark_hard") {
		t.Errorf("saved file = %q", b)
	}
}

func TestSaveBoardKeepsOtherKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jt.yml")
	if err := os.WriteFile(path, []byte("theme: charm\njira:\n  project: ABC\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JT_CONFIG_FILE", path)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveBoard(42); err != nil {
		t.Fatal(err)
	}

	b, _ := os.ReadFile(path)
	for _, want := range []string{"theme: charm", "project: ABC", "board_id: 42"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("saved file = %q, missing %q", b, want)
		}
	}
}

func TestSaveThemeIsSafeConcurrently(t *testing.T) {
	t.Setenv("JT_CONFIG_FILE", filepath.Join(t.TempDir(), "jt.yml"))
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for _, th := range []string{"dracula", "nord", "gruvbox_dark_hard", "catppuccin_mocha"} {
		wg.Go(func() {
			if err := cfg.SaveTheme(th); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
}

func TestLoadExplainsWhenDefaultFileCannotBeCreated(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	if f, err := os.Create(filepath.Join(dir, "probe")); err == nil {
		_ = f.Close()
		t.Skip("directory is writable despite chmod, probably running as root")
	}
	path := filepath.Join(dir, "jt.yml")
	t.Setenv("JT_CONFIG_FILE", path)

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"can't create the settings file " + path, "permission denied", "JT_CONFIG_FILE"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q is missing %q", err, want)
		}
	}
	if strings.Count(err.Error(), path) != 1 {
		t.Errorf("error repeats the path: %q", err)
	}
}

func TestLoadExplainsWhenFileCannotBeRead(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JT_CONFIG_FILE", filepath.Join(blocker, "jt.yml"))

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "can't read the settings file") {
		t.Fatalf("err = %v", err)
	}
}

func TestSaveProjectForgetsBoardOnlyWhenProjectChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jt.yml")
	if err := os.WriteFile(path, []byte("jira:\n  project: ABC\n  board_id: 42\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JT_CONFIG_FILE", path)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveProject("ABC"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); !strings.Contains(string(b), "board_id: 42") {
		t.Errorf("same project dropped the board: %q", b)
	}

	if err := cfg.SaveProject("XYZ"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	for _, want := range []string{"project: XYZ", "board_id: 0"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("saved file = %q, missing %q", b, want)
		}
	}
}

func TestLoadMigratesLegacyFile(t *testing.T) {
	tests := []struct {
		name          string
		legacy, newer string
		want          string
	}{
		{"copies the old file", "theme: old\n", "", "old"},
		{"keeps an existing new file", "theme: old\n", "theme: new\n", "new"},
		{"no files", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", "")
			t.Setenv("JT_CONFIG_FILE", "")

			osDir, err := os.UserConfigDir()
			if err != nil {
				t.Fatal(err)
			}
			legacy := filepath.Join(osDir, "jt.yaml")
			path := filepath.Join(home, ".config", "jt", "jt.yaml")
			write := func(p, contents string) {
				if contents == "" {
					return
				}
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			write(legacy, tt.legacy)
			write(path, tt.newer)

			cfg, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Path() != path {
				t.Errorf("Path = %q, want %q", cfg.Path(), path)
			}
			if cfg.Theme != tt.want {
				t.Errorf("theme = %q, want %q", cfg.Theme, tt.want)
			}
			if b, err := os.ReadFile(legacy); tt.legacy != "" && (err != nil || string(b) != tt.legacy) {
				t.Errorf("legacy file = %q, %v; want it untouched", b, err)
			}
		})
	}
}
