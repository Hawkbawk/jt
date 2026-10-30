package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestLoadJira(t *testing.T) {
	cfg, err := load(t, `jira:
  server: https://example.atlassian.net/
  login: me@example.com
  project: ABC
  board_id: 42
`)
	if err != nil {
		t.Fatal(err)
	}
	j := cfg.Jira
	if !j.Configured() || j.Server != "https://example.atlassian.net" || j.Project != "ABC" || j.BoardID != 42 {
		t.Errorf("jira = %+v", j)
	}
}

func TestLoadWithoutJiraIsNotConfigured(t *testing.T) {
	cfg, err := load(t, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Jira.Configured() {
		t.Errorf("jira = %+v", cfg.Jira)
	}
}

func TestSaveJiraRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jt.yml")
	if err := os.WriteFile(path, []byte("theme: charm\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JT_CONFIG_FILE", path)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	want := Jira{
		Server:  "https://example.atlassian.net",
		Login:   "me@example.com",
		Project: "ABC",
		BoardID: 7,
	}
	withSlash := want
	withSlash.Server += "/"
	if err := cfg.SaveJira(withSlash); err != nil {
		t.Fatal(err)
	}

	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Jira != want || got.Theme != "charm" {
		t.Errorf("reloaded = %+v, theme %q", got.Jira, got.Theme)
	}
}

func TestToken(t *testing.T) {
	keyring.MockInit()

	if _, err := Token("me@example.com"); !errors.Is(err, ErrNoToken) {
		t.Fatalf("err = %v, want ErrNoToken", err)
	}
	if err := SaveToken("me@example.com", "secret"); err != nil {
		t.Fatal(err)
	}
	if got, err := Token("me@example.com"); err != nil || got != "secret" {
		t.Errorf("token = %q, %v", got, err)
	}
}
