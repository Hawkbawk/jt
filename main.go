// Command jt is a terminal kanban board for Jira.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ryanhawkins/jt/internal/config"
	"github.com/ryanhawkins/jt/internal/jira"
	"github.com/ryanhawkins/jt/internal/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "jt:", err)
		os.Exit(1)
	}
}

func run() error {
	cfgPath := config.DefaultPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading %s: %w", cfgPath, err)
	}

	client, err := jira.New(jira.CLIConfigPath(config.Home()))
	if err != nil {
		return err
	}

	_, err = tea.NewProgram(tui.New(client, cfg, cfgPath), tea.WithAltScreen()).Run()
	return err
}
