// Command jt is a terminal kanban board for Jira.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "jt:", err)
		os.Exit(1)
	}
}

func run() error {
	client, err := jira.New()
	if err != nil {
		return err
	}

	_, err = tea.NewProgram(tui.New(client), tea.WithAltScreen()).Run()
	return err
}
