package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hawkbawk/jt/internal/components/issuedetail"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/standalone"
)

var viewCmd = &cobra.Command{
	Use:     "view <issue>",
	Aliases: []string{"v", "show", "display"},
	Short:   "View an issue.",
	Long: `View an issue's details. The issue is a key like ABC-123, or just a number
like 123 for an issue in the default project.`,
	Example: "  jt view ABC-123\n  jt view 123",
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
	return run(standalone.New(issuedetail.NewLoaded(backend, detail, styles.Markdown)))
}
