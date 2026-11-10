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
	"github.com/hawkbawk/jt/internal/fakejira"
	"github.com/hawkbawk/jt/internal/icons"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/keys"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/termimage"
	"github.com/hawkbawk/jt/internal/tui/board"
	"github.com/hawkbawk/jt/internal/tui/search"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
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

	// fake runs the screens on in-memory demo data. It's only allowed in
	// development builds.
	fake                          bool
	fakeSeed                      uint64
	fakeUserCount, fakeIssueCount int
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
	rootCmd.PersistentFlags().BoolVar(&fake, "fake", false, "use in-memory demo data instead of Jira, without reading or writing settings")
	rootCmd.PersistentFlags().Uint64Var(&fakeSeed, "fake-seed", 0, "seed for the --fake data, to repeat a run")
	rootCmd.PersistentFlags().IntVar(&fakeUserCount, "fake-users", 0, "number of people in the --fake data")
	rootCmd.PersistentFlags().IntVar(&fakeIssueCount, "fake-issues", 0, "number of DEMO issues in the --fake data; OPS gets a third as many")
	if version != "" {
		for _, name := range []string{"fake", "fake-seed", "fake-users", "fake-issues"} {
			_ = rootCmd.PersistentFlags().MarkHidden(name)
		}
	}
}

// Execute runs the command named by the arguments, opening the board when
// there isn't one.
func Execute() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fake = fakeRequested(os.Args[1:])
	theme := ""
	if !fake {
		cfg, cfgErr = config.Load()
		theme = cfg.Theme
		if cfgErr != nil {
			theme = styles.FallbackID()
		}
	}
	themeErr = styles.Start(ctx, theme)
	if cfgErr == nil && !fake {
		icons.SetEnabled(cfg.Icons)
	}
	keys.Set(keys.Default())

	err := fang.Execute(ctx, rootCmd,
		fang.WithVersion(version),
		fang.WithColorSchemeFunc(styles.CLIColorScheme),
	)
	if err != nil {
		os.Exit(1)
	}
}

// fakeRequested reports whether --fake was passed. It runs before cobra parses
// the command line, since fake mode skips loading the settings file that the
// theme is read from.
func fakeRequested(args []string) bool {
	allowlist := rootCmd.FParseErrWhitelist
	rootCmd.FParseErrWhitelist.UnknownFlags = true
	defer func() { rootCmd.FParseErrWhitelist = allowlist }()
	if rootCmd.ParseFlags(args) != nil {
		return false
	}
	on, _ := rootCmd.PersistentFlags().GetBool("fake")
	return on
}

// sessionSettings keeps choices for the current session only.
type sessionSettings struct{}

func (sessionSettings) SaveTheme(string) error { return nil }

func (sessionSettings) SaveBoard(int) error { return nil }

func (sessionSettings) SaveProject(string) error { return nil }

func runBoard(*cobra.Command, []string) error {
	backend, settings, err := openBackend()
	if err != nil {
		return err
	}
	m := board.New(backend, settings, commandsForSession(fake, cfg.Commands)...)
	if themeErr != nil {
		m = m.WithWarning(themeErr.Error())
	}
	return run(m)
}

// screenBackend is the Jira backend the board and search screens run on.
type screenBackend interface {
	board.Backend
	search.Backend
}

// openBackend returns the Jira backend and settings the screens run on: demo
// data with --fake, otherwise the connection from the settings file.
func openBackend() (screenBackend, board.Settings, error) {
	if fake {
		if version != "" {
			return nil, nil, errors.New("the --fake flag is only available in development builds")
		}
		backend := fakejira.New(fakeOptions()...)
		fmt.Fprintf(os.Stderr, "fake data seed: %d (repeat it with --fake-seed)\n", backend.Seed())
		return backend, sessionSettings{}, nil
	}
	if cfgErr != nil {
		return nil, nil, cfgErr
	}
	client, err := connect(cfg.Jira)
	if err != nil {
		return nil, nil, err
	}
	return client, cfg, nil
}

func commandsForSession(isFake bool, configured []config.Command) []config.Command {
	if isFake {
		return fakejira.DemoCommands()
	}
	return configured
}

// fakeOptions turns the --fake-* flags into options for the fake backend.
func fakeOptions() []fakejira.Option {
	var opts []fakejira.Option
	if fakeSeed != 0 {
		opts = append(opts, fakejira.WithSeed(fakeSeed))
	}
	size := fakejira.DefaultSize
	if fakeUserCount > 0 {
		size.Users = fakeUserCount
	}
	if fakeIssueCount > 0 {
		size.DemoIssues = fakeIssueCount
		size.OpsIssues = max(fakeIssueCount/3, 1)
	}
	return append(opts, fakejira.WithSize(size))
}

// run shows m until it quits, following changes to the terminal's theme.
func run(m tea.Model) error {
	p := tea.NewProgram(graphicsProbe{m})
	// Send blocks until the program reads the message, and a tint can change
	// from inside Update, so it runs on its own goroutine.
	defer tint.Subscribe(func(ev tint.Event) {
		if ev.Kind == tint.CurrentChanged {
			go p.Send(tuimsg.ThemeChanged{})
		}
	})()

	_, err := p.Run()
	if termimage.Supported() {
		_, _ = fmt.Fprint(os.Stdout, termimage.DeleteAll())
	}
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
