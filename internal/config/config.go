// Package config loads and saves jt's JSON configuration from the XDG config directory.
package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Config is jt's persisted configuration.
type Config struct {
	// DefaultProject is the project key opened when jt starts.
	DefaultProject string `json:"default_project,omitempty"`
	// Boards optionally pins a board ID to a project key.
	Boards map[string]int `json:"boards,omitempty"`
}

// Home returns $XDG_CONFIG_HOME, falling back to ~/.config per the XDG spec.
func Home() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(dir) {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

// DefaultPath returns $XDG_CONFIG_HOME/jt/config.json.
func DefaultPath() string {
	return filepath.Join(Home(), "jt", "config.json")
}

// Load reads the config at path. A missing file yields an empty config.
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var c Config
	if err := json.NewDecoder(f).Decode(&c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Save writes the config to path, creating parent directories as needed.
func (c *Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// BoardFor returns the pinned board ID for a project, if any.
func (c *Config) BoardFor(project string) (int, bool) {
	id, ok := c.Boards[project]
	return id, ok
}
