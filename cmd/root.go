// Package cmd holds jt's cobra commands.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"charm.land/fang/v2"
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

var (
	// cfg is loaded before any command runs so help and errors can use the
	// theme. Commands that need the settings report cfgErr themselves, so a
	// broken settings file doesn't break --help, completion or man.
	cfg      config.Config
	cfgErr   error
	themeErr error
)

var rootCmd = &cobra.Command{
	Use:   "jt",
	Short: "jt is a terminal kanban board for Jira.",
	Long: `jt is a terminal kanban board for Jira.

Environment:
  JT_CONFIG_FILE    Path to the jt settings file`,
	Args: cobra.NoArgs,
	RunE: runBoard,
}

// Execute runs the command named by the arguments, opening the board when
// there isn't one.
func Execute() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg, cfgErr = config.Load()
	theme := cfg.Theme
	if cfgErr != nil {
		theme = styles.FallbackID()
	}
	themeErr = styles.Start(ctx, theme)

	err := fang.Execute(ctx, rootCmd,
		fang.WithVersion(version),
		fang.WithColorSchemeFunc(styles.CLIColorScheme),
	)
	if err != nil {
		os.Exit(1)
	}
}

func runBoard(*cobra.Command, []string) error {
	if cfgErr != nil {
		return cfgErr
	}
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
		return nil, errors.New("not connected to Jira yet, run 'jt init' to set it up")
	}
	token, err := config.Token(cfg.Login)
	if err != nil {
		return nil, fmt.Errorf("reading the API token for %s: %w (run 'jt init' to save one)", cfg.Login, err)
	}
	return jira.New(cfg, token), nil
}
