package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hawkbawk/jt/internal/components/issuecreate"
	"github.com/hawkbawk/jt/internal/tui/standalone"
)

var createCmd = &cobra.Command{
	Use:     "create [project]",
	Aliases: []string{"new"},
	Short:   "Create an issue.",
	Long: `Create an issue. Without a project, you pick one from a list that starts on
the default project. Then pick the issue type and fill in the fields.`,
	Example: "  jt create\n  jt create ABC",
	Args:    cobra.MaximumNArgs(1),
	RunE:    runCreate,
}

func init() {
	rootCmd.AddCommand(createCmd)
}

func runCreate(cmd *cobra.Command, args []string) error {
	backend, _, err := openBackend()
	if err != nil {
		return err
	}
	content := issuecreate.New(backend, backend.DefaultProject())
	if len(args) == 1 {
		content = issuecreate.NewForProject(backend, strings.ToUpper(args[0]))
	}
	final, err := runModel(standalone.New(content))
	if err != nil {
		return err
	}
	if s, ok := final.(standalone.Model); ok && s.Result() != nil {
		_, err = fmt.Fprintln(cmd.OutOrStdout(), s.Result().Status())
	}
	return err
}
