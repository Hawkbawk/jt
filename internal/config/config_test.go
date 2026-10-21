package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileReturnsEmptyConfig(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatal(err)
	}
	if c.DefaultProject != "" || c.Boards != nil {
		t.Fatalf("expected empty config, got %+v", c)
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	want := &Config{DefaultProject: "INTEROP", Boards: map[string]int{"INTEROP": 39}}
	if err := want.Save(path); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.DefaultProject != "INTEROP" {
		t.Errorf("DefaultProject = %q", got.DefaultProject)
	}
	if id, ok := got.BoardFor("INTEROP"); !ok || id != 39 {
		t.Errorf("BoardFor = %d, %v", id, ok)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestDefaultPathUsesXDGConfigHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	if got := DefaultPath(); got != "/xdg/jt/config.json" {
		t.Fatalf("DefaultPath = %s", got)
	}
}

func TestDefaultPathFallsBackToDotConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/home/me")
	if got := DefaultPath(); got != "/home/me/.config/jt/config.json" {
		t.Fatalf("DefaultPath = %s", got)
	}
}
