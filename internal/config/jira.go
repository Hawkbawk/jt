package config

import (
	"errors"
	"strings"

	"github.com/zalando/go-keyring"
)

// keyringService is the name jt's API tokens are stored under in the OS
// keyring.
const keyringService = "jt"

// Jira holds the settings jt needs to talk to a Jira Cloud site. jt signs in
// with basic auth, using the login email and an API token.
type Jira struct {
	// Server is the site URL, without a trailing slash.
	Server string `mapstructure:"server"`
	Login  string `mapstructure:"login"`
	// Project is the key of the project to open on start.
	Project string `mapstructure:"project"`
	// BoardID is the board to open for Project. Zero picks one.
	BoardID int `mapstructure:"board_id"`
}

// Configured reports whether enough is set to connect.
func (j Jira) Configured() bool {
	return j.Server != "" && j.Login != ""
}

func (j Jira) settings() map[string]any {
	return map[string]any{
		"server":   trimServer(j.Server),
		"login":    j.Login,
		"project":  j.Project,
		"board_id": j.BoardID,
	}
}

func trimServer(s string) string {
	return strings.TrimSuffix(s, "/")
}

// ErrNoToken is returned by Token when the keyring has no token for the
// login.
var ErrNoToken = errors.New("no API token in the keyring")

// Token reads the API token for login from the OS keyring.
func Token(login string) (string, error) {
	t, err := keyring.Get(keyringService, login)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNoToken
	}
	return t, err
}

// SaveToken stores the API token for login in the OS keyring.
func SaveToken(login, token string) error {
	return keyring.Set(keyringService, login, token)
}
