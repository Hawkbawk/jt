// Package pane holds what popups that act on an issue share: the spinner, the
// form theme, the title and the status line.
package pane

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/hawkbawk/jt/internal/styles"
)

// BodyLines is what a pane draws around its form: a blank line above it, and a
// blank line and the status line below.
const BodyLines = 3

// Title is a pane's title for the issue with issueKey.
func Title(title, issueKey string) string { return title + " · " + issueKey }

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
