// Command jt is a terminal kanban board for Jira.
package main

import (
	"context"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	tint "github.com/lrstanley/bubbletint/v2"

	"github.com/hawkbawk/jt/internal/config"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "jt:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	themeErr := styles.Start(ctx, cfg.Theme)

	client, err := jira.New()
	if err != nil {
		return err
	}

	m := tui.New(client, cfg)
	if themeErr != nil {
		m = m.WithWarning(themeErr.Error())
	}
	p := tea.NewProgram(m)
	// Send blocks until the program reads the message, and a tint can change
	// from inside Update, so it runs on its own goroutine.
	defer tint.Subscribe(func(ev tint.Event) {
		if ev.Kind == tint.CurrentChanged {
			go p.Send(tui.ThemeChangedMsg{})
		}
	})()

	_, err = p.Run()
	return err
}
