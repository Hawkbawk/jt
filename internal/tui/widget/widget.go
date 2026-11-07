// Package widget holds the stateful pieces jt's components share: a spinner
// that runs only while its owner is busy, and themed forms.
package widget

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/hawkbawk/jt/internal/styles"
)

// Spinner is a themed spinner that ticks only while its owner is busy. The
// owner passes it its ticks with Update and calls Sync after anything that can
// make it busy, which starts the ticks again if they had stopped.
type Spinner struct {
	spinner.Model
	ticking bool
}

// NewSpinner returns a themed spinner.
func NewSpinner() Spinner {
	return Spinner{Model: spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(styles.Title()))}
}

// Update advances the spinner on its own ticks while busy, and lets the ticks
// stop once it isn't.
func (s *Spinner) Update(msg spinner.TickMsg, busy bool) tea.Cmd {
	if msg.ID > 0 && msg.ID != s.ID() {
		return nil
	}
	if !busy {
		s.ticking = false
		return nil
	}
	var cmd tea.Cmd
	s.Model, cmd = s.Model.Update(msg)
	return cmd
}

// Sync starts the ticks when busy and they've stopped.
func (s *Spinner) Sync(busy bool) tea.Cmd {
	if !busy || s.ticking {
		return nil
	}
	s.ticking = true
	return s.Tick
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
