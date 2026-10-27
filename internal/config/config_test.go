package config

import (
	"os"
	"path/filepath"
	"strings"
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
		want           Theme
		wantErr        bool
	}{
		{"missing file defaults to base", "", ThemeBase, false},
		{"empty theme defaults to base", "other: 1\n", ThemeBase, false},
		{"named theme", "theme: gruvbox\n", ThemeGruvbox, false},
		{"case-insensitive", "theme: Dracula\n", ThemeDracula, false},
		{"unknown theme", "theme: nope\n", ThemeBase, true},
		{"malformed", "theme: [\n", ThemeBase, true},
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

func TestLoadCreatesDefaultFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "jt.yml")
	t.Setenv("JT_CONFIG_FILE", path)

	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(b), "theme: base") {
		t.Fatalf("default file = %q, %v", b, err)
	}
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
