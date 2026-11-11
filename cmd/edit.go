package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hawkbawk/jt/internal/components/issueedit"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/tui/standalone"
)

var editCmd = &cobra.Command{
	Use:     "edit <issue>",
	Aliases: []string{"e", "modify", "update"},
	Short:   "Edit an issue.",
	Long: `Edit an issue's fields. The issue is a key like ABC-123, or just a number
like 123 for an issue in the default project.`,
	Example: "  jt edit ABC-123\n  jt edit 123",
	Args:    cobra.ExactArgs(1),
	RunE:    runEdit,
}

func init() {
	rootCmd.AddCommand(editCmd)
}

func runEdit(_ *cobra.Command, args []string) error {
	backend, _, err := openBackend()
	if err != nil {
		return err
	}
	issueKey, text, err := jira.ParseSearchText(args[0], backend.DefaultProject())
	if err != nil {
		return err
	}
	if issueKey == "" {
		return fmt.Errorf("%q isn't an issue key like ABC-123 or an issue number", text)
	}
	detail, err := backend.Issue(issueKey)
	if err != nil {
		return fmt.Errorf("couldn't load %s: %w", issueKey, err)
	}
	return run(standalone.New(issueedit.New(backend, detail.Issue)))
}
