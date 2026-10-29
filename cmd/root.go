// Package cmd holds jt's cobra commands.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime/debug"

	tea "charm.land/bubbletea/v2"
	tint "github.com/lrstanley/bubbletint/v2"
	"github.com/spf13/cobra"

	"github.com/hawkbawk/jt/internal/config"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui"
)

// version is set at build time with
// -ldflags "-X github.com/hawkbawk/jt/cmd.version=...".
var version string

var rootCmd = &cobra.Command{
	Use:   "jt",
	Short: "jt is a terminal kanban board for Jira.",
	Long: `jt is a terminal kanban board for Jira.

Environment:
  JT_CONFIG_FILE    Path to the jt settings file`,
	Version: buildVersion(),
	Args:    cobra.NoArgs,
	RunE:    runBoard,
}

// Execute runs the command named by the arguments, opening the board when
// there isn't one.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func buildVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

func runBoard(*cobra.Command, []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	themeErr := styles.Start(ctx, cfg.Theme)

	client, err := connect(cfg.Jira)
	if err != nil {
		return err
	}

	m := tui.New(client, cfg)
	if themeErr != nil {
		m = m.WithWarning(themeErr.Error())
	}
	p := tea.NewProgram(m)
	// Send blocks until the program reads the message, and a tint can change
	// from inside Update, so it runs on its own goroutine.
	defer tint.Subscribe(func(ev tint.Event) {
		if ev.Kind == tint.CurrentChanged {
			go p.Send(tui.ThemeChangedMsg{})
		}
	})()

	_, err = p.Run()
	return err
}

// connect builds a Jira client from the settings and the token in the
// keyring.
func connect(cfg config.Jira) (*jira.Client, error) {
	if !cfg.Configured() {
		return nil, errors.New("jira.server and jira.login aren't set in jt's settings file")
	}
	token, err := config.Token(cfg.Login)
	if err != nil {
		return nil, fmt.Errorf("reading the API token for %s: %w", cfg.Login, err)
	}
	return jira.New(cfg, token), nil
}
