package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hawkbawk/jt/internal/tui/search"
)

var searchCmd = &cobra.Command{
	Use:     "search",
	Aliases: []string{"s"},
	Short:   "Search for issues.",
	Long: `Search for issues with a form or with JQL, then view, edit, assign and
comment on the issues found.`,
	Args: cobra.NoArgs,
	RunE: runSearch,
}

func init() {
	rootCmd.AddCommand(searchCmd)
}

func runSearch(*cobra.Command, []string) error {
	backend, settings, err := openBackend()
	if err != nil {
		return err
	}
	m := search.New(backend, settings, commandsForSession(fake, cfg.Commands)...)
	if warning := startupWarning(); warning != "" {
		m = m.WithWarning(warning)
	}
	return run(m)
}
