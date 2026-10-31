// Package cmd holds jt's cobra commands.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	tea "charm.land/bubbletea/v2"
	"charm.land/fang/v2"
	"charm.land/lipgloss/v2"
	tint "github.com/lrstanley/bubbletint/v2"
	"github.com/spf13/cobra"

	"github.com/hawkbawk/jt/internal/config"
	"github.com/hawkbawk/jt/internal/fakejira"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui"
)

// version is set at build time with
// -ldflags "-X github.com/hawkbawk/jt/cmd.version=...". Builds without one are
// development builds.
var version string

var (
	// cfg is loaded before any command runs so help and errors can use the
	// theme. Commands that need the settings report cfgErr themselves, so a
	// broken settings file doesn't break --help, completion or man.
	cfg      config.Config
	cfgErr   error
	themeErr error

	// fake runs the board on in-memory demo data. It's only allowed in
	// development builds.
	fake bool
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

func init() {
	rootCmd.Flags().BoolVar(&fake, "fake", false, "use in-memory demo data instead of Jira, without reading or writing settings")
	if version != "" {
		_ = rootCmd.Flags().MarkHidden("fake")
	}
}

// Execute runs the command named by the arguments, opening the board when
// there isn't one.
func Execute() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	initialize := sync.OnceFunc(func() {
		theme := ""
		if !fake {
			cfg, cfgErr = config.Load()
			theme = cfg.Theme
			if cfgErr != nil {
				theme = styles.FallbackID()
			}
		}
		themeErr = styles.Start(ctx, theme)
	})
	cobra.OnInitialize(initialize)

	err := fang.Execute(ctx, rootCmd,
		fang.WithVersion(version),
		fang.WithColorSchemeFunc(func(lightDark lipgloss.LightDarkFunc) fang.ColorScheme {
			// Help and parse errors bypass Cobra's initializers.
			initialize()
			return styles.CLIColorScheme(lightDark)
		}),
	)
	if err != nil {
		os.Exit(1)
	}
}

// sessionSettings keeps choices for the current session only.
type sessionSettings struct{}

func (sessionSettings) SaveTheme(string) error { return nil }

func runBoard(*cobra.Command, []string) error {
	var (
		backend  tui.Backend
		settings tui.Settings
	)
	if fake {
		if version != "" {
			return errors.New("the --fake flag is only available in development builds")
		}
		backend, settings = fakejira.New(), sessionSettings{}
	} else {
		if cfgErr != nil {
			return cfgErr
		}
		client, err := connect(cfg.Jira)
		if err != nil {
			return err
		}
		backend, settings = client, cfg
	}

	m := tui.New(backend, settings)
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

	_, err := p.Run()
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
