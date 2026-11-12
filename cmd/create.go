package cmd

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/hawkbawk/jt/internal/components/issuecreate"
	"github.com/hawkbawk/jt/internal/tui/standalone"
)

var createCmd = &cobra.Command{
	Use:     "create [project]",
	Aliases: []string{"new"},
	Short:   "Create an issue.",
	Long: `Create an issue. The project defaults to the default project, and you can
change it, and pick the issue type, before filling in the fields.`,
	Example: "  jt create\n  jt create ABC",
	Args:    cobra.MaximumNArgs(1),
	RunE:    runCreate,
}

func init() {
	rootCmd.AddCommand(createCmd)
}

func runCreate(_ *cobra.Command, args []string) error {
	backend, _, err := openBackend()
	if err != nil {
		return err
	}
	project := backend.DefaultProject()
	if len(args) == 1 {
		project = strings.ToUpper(args[0])
	}
	return run(standalone.New(issuecreate.New(backend, project)))
}
