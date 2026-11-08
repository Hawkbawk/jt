// Package tuimsg holds messages that the screens and the components they host
// both handle.
package tuimsg

import (
	"fmt"

	"github.com/hawkbawk/jt/internal/command"
	"github.com/hawkbawk/jt/internal/config"
	"github.com/hawkbawk/jt/internal/jira"
)

// ThemeChanged reports that the current tint changed outside of jt, for
// example because the terminal's theme was edited.
type ThemeChanged struct{}

// CommandSelected is the command and issue context selected from a screen.
type CommandSelected struct {
	Command config.Command
	Issue   jira.Issue
	Project string
	Board   string
}

// CommandLoaded carries the complete ticket context for a command.
type CommandLoaded struct {
	Command config.Command
	Data    command.Data
	Err     error
}

// CommandFinished reports the result of running a configured command.
type CommandFinished struct {
	Command config.Command
	Err     error
}

// Status returns the command's result for the status line.
func (m CommandFinished) Status() string {
	if m.Err != nil {
		return fmt.Sprintf("running %s: %v", m.Command.DisplayName(), m.Err)
	}
	return "Ran " + m.Command.DisplayName()
}

// Finished is implemented by the message a component closes its popup with
// after it changed an issue. The screen behind the popup reloads the issue and
// shows the status.
type Finished interface {
	// IssueKey is the issue that changed.
	IssueKey() string
	// Status is what to tell the user happened.
	Status() string
}
