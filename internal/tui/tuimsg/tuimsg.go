// Package tuimsg holds messages that the screens and the components they host
// both handle.
//
// Components in a modal reach the screen behind it with messages that have the
// modal host's marker methods: ForParent sends a message to the screen, and
// ClosesModal closes the modal first.
package tuimsg

import (
	"fmt"
	"reflect"
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

// Map returns cmd with fn applied to each message it produces, including the
// ones in a batch or a sequence. A component that embeds another uses it to
// give the child's messages a meaning of its own, such as a pick that would
// otherwise close the modal through ClosesModal. fn returns messages it doesn't
// change as they are, and a nil cmd stays nil.
func Map(cmd tea.Cmd, fn func(tea.Msg) tea.Msg) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			mapped := make([]tea.Cmd, len(batch))
			for i, c := range batch {
				mapped[i] = Map(c, fn)
			}
			return tea.BatchMsg(mapped)
		}
		// tea.Sequence's message is unexported, so it's recognized by its shape.
		if v := reflect.ValueOf(msg); v.IsValid() && v.Kind() == reflect.Slice &&
			v.Type().Elem() == cmdType && v.Type().PkgPath() == teaPkg {
			mapped := make([]tea.Cmd, v.Len())
			for i := range mapped {
				mapped[i] = Map(v.Index(i).Interface().(tea.Cmd), fn)
			}
			return tea.Sequence(mapped...)()
		}
		return fn(msg)
	}
}

var (
	cmdType = reflect.TypeFor[tea.Cmd]()
	teaPkg  = reflect.TypeFor[tea.QuitMsg]().PkgPath()
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
