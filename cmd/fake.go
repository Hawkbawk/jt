//go:build dev

package cmd

import (
	"github.com/hawkbawk/jt/internal/fakejira"
	"github.com/hawkbawk/jt/internal/tui"
)

func init() {
	rootCmd.Flags().BoolVar(&fake, "fake", false, "use in-memory demo data instead of Jira, without reading or writing settings")
	newFakeBackend = func() tui.Backend { return fakejira.New() }
}
