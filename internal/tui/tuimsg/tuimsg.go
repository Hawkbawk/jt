// Package tuimsg holds messages that the screens and the components they host
// both handle.
//
// Components in a modal reach the screen behind it with messages that have the
// modal host's marker methods: ForParent sends a message to the screen, and
// ClosesModal closes the modal first.
package tuimsg

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/command"
	"github.com/hawkbawk/jt/internal/config"
	"github.com/hawkbawk/jt/internal/jira"
)

// Closed closes the modal a component is in.
type Closed struct{}

// ClosesModal marks Closed for the modal host.
func (Closed) ClosesModal() {}

// Close returns the command that closes the modal a component is in.
func Close() tea.Cmd { return Send(Closed{}) }

// Send returns the command that produces msg.
func Send(msg tea.Msg) tea.Cmd { return func() tea.Msg { return msg } }

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

// ForParent marks CommandSelected for the modal host.
func (CommandSelected) ForParent() {}

// ClosesModal marks CommandSelected for the modal host.
func (CommandSelected) ClosesModal() {}

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
	Stderr  string
}

// Status returns the command's result for the status line.
func (m CommandFinished) Status() string {
	if m.Err != nil {
		status := fmt.Sprintf("running %s: %v", m.Command.DisplayName(), m.Err)
		if stderr := strings.TrimSpace(m.Stderr); stderr != "" {
			return status + ": " + stderr
		}
		return status
	}
	return "Ran " + m.Command.DisplayName()
}

// Finished is implemented by the message a component sends after it changed an
// issue. The screen behind the modal reloads the issue and shows the status.
type Finished interface {
	// ForParent sends the message to the screen.
	ForParent()
	// IssueKey is the issue that changed.
	IssueKey() string
	// Status is what to tell the user happened.
	Status() string
}
