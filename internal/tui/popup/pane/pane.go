// Package pane holds what popups that act on an issue share: the spinner and
// request IDs, the form theme, the title and the status line.
package pane

import (
	"sync/atomic"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/hawkbawk/jt/internal/styles"
)

// ChromeLines is what a pane draws around its body: a title and a blank line
// above it, and the help footer below.
const ChromeLines = 3

// Header is the title a pane draws above its body.
func Header(title, issueKey string) []string {
	return []string{styles.Title().Render(title + " · " + issueKey), ""}
}

// NewSpinner returns a themed spinner.
func NewSpinner() spinner.Model {
	return spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(styles.Title()))
}

// Tick advances s while busy. The tick chain ends once nothing is busy, so
// whoever becomes busy again has to start it with s.Tick.
func Tick(s *spinner.Model, msg spinner.TickMsg, busy bool) tea.Cmd {
	if !busy {
		return nil
	}
	var cmd tea.Cmd
	*s, cmd = s.Update(msg)
	return cmd
}

// Restart adds the spinner's tick to cmd when a change made a pane busy that
// wasn't, since the tick chain ends whenever nothing is busy.
func Restart(s spinner.Model, wasBusy, busy bool, cmd tea.Cmd) tea.Cmd {
	if !wasBusy && busy {
		return tea.Batch(cmd, s.Tick)
	}
	return cmd
}

var lastID atomic.Int64

// NewID returns an ID for a pane to put on the requests it makes. A response
// that arrives after the pane closed, perhaps with another pane open for the
// same issue by then, carries the old ID and gets ignored.
func NewID() int64 { return lastID.Add(1) }

// StatusLine shows the error, the in-progress text or the hint, in that order.
func StatusLine(spin string, innerW int, err error, applying bool, progress, hint string) string {
	switch {
	case err != nil:
		return styles.Error().Width(innerW).Render("Error: " + err.Error())
	case applying:
		return spin + " " + progress
	}
	return styles.Dim().Render(hint)
}

// NewForm puts fields in a themed form that leaves ctrl+c to the app.
func NewForm(fields ...huh.Field) *huh.Form {
	return NewGroupsForm(huh.NewGroup(fields...))
}

// NewGroupsForm is NewForm for fields spread over groups.
func NewGroupsForm(groups ...*huh.Group) *huh.Form {
	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(key.WithDisabled())
	return huh.NewForm(groups...).
		WithTheme(styles.FormTheme()).
		WithKeyMap(km).
		WithShowHelp(false)
}
