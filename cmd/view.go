package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/view"
)

var viewCmd = &cobra.Command{
	Use:     "v <issue>",
	Aliases: []string{"view"},
	Short:   "View an issue.",
	Long: `View an issue's details. The issue is a key like ABC-123, or just a number
like 123 for an issue in the default project.`,
	Example: "  jt v ABC-123\n  jt v 123",
	Args:    cobra.ExactArgs(1),
	RunE:    runView,
}

func init() {
	rootCmd.AddCommand(viewCmd)
}

func runView(_ *cobra.Command, args []string) error {
	backend, _, err := openBackend()
	if err != nil {
		return err
	}
	issueKey, err := jira.ParseKey(args[0], backend.DefaultProject())
	if err != nil {
		return err
	}
	detail, err := backend.Issue(issueKey)
	if err != nil {
		return fmt.Errorf("couldn't load %s: %w", issueKey, err)
	}
	return run(view.New(backend, detail, styles.Markdown))
}
