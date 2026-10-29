package setup

import (
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/hawkbawk/jt/internal/config"
	"github.com/hawkbawk/jt/internal/jira"
)

func TestAnswersRoundTripConfig(t *testing.T) {
	j := config.Jira{Server: "https://x.atlassian.net", Login: "me@x.com", Project: "ABC", BoardID: 4}
	if got := fromConfig(j).jira(); got != j {
		t.Errorf("got %+v, want %+v", got, j)
	}
	a := answers{server: " https://x.atlassian.net ", login: " me@x.com "}
	if got := a.jira(); got.Server != "https://x.atlassian.net" || got.Login != "me@x.com" {
		t.Errorf("trimmed = %+v", got)
	}
}

func TestValidateServer(t *testing.T) {
	for s, ok := range map[string]bool{
		"https://x.atlassian.net":    true,
		" https://x.atlassian.net/ ": true,
		"http://x.atlassian.net":     false,
		"x.atlassian.net":            false,
		"ftp://jira.local":           false,
		"":                           false,
	} {
		if err := validateServer(s); (err == nil) != ok {
			t.Errorf("validateServer(%q) = %v", s, err)
		}
	}
}

func TestBoardOptionsStartWithAutomatic(t *testing.T) {
	opts := boardOptions([]jira.BoardSummary{{ID: 7, Name: "Team", Type: "scrum"}})
	if len(opts) != 2 || opts[0].Value != 0 || opts[1].Value != 7 || opts[1].Key != "Team (scrum)" {
		t.Errorf("options = %+v", opts)
	}
}

func TestSave(t *testing.T) {
	keyring.MockInit()
	t.Setenv("JT_CONFIG_FILE", filepath.Join(t.TempDir(), "jt.yml"))
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}

	a := answers{
		server: "https://x.atlassian.net", login: "me@x.com", token: "secret",
		project: "ABC", boardID: 3,
	}
	if err := save(cfg, a); err != nil {
		t.Fatal(err)
	}

	reloaded, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Jira != a.jira() {
		t.Errorf("saved %+v, want %+v", reloaded.Jira, a.jira())
	}
	if tok, err := config.Token("me@x.com"); err != nil || tok != "secret" {
		t.Errorf("token = %q, %v", tok, err)
	}
}
