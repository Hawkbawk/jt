package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hawkbawk/jt/internal/setup"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Set up the connection to Jira",
	Long: `Set up the connection to Jira.

init asks for your Jira site, email and API token, checks them, and lets you
pick a default project and board. The settings go in jt's settings file and
the API token goes in your OS keyring. Run it again to change any of them. It asks before
replacing settings you already have.`,
	Args: cobra.NoArgs,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, _ []string) error {
	if cfgErr != nil {
		return cfgErr
	}
	return setup.Run(cfg, cmd.OutOrStdout())
}
