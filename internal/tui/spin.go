package tui

import (
	"sync/atomic"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/styles"
)

func newSpinner() spinner.Model {
	return spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(styles.Title()))
}

// tickSpinner advances s while busy. The tick chain ends once nothing is busy,
// so whoever becomes busy again has to start it with s.Tick.
func tickSpinner(s *spinner.Model, msg spinner.TickMsg, busy bool) tea.Cmd {
	if !busy {
		return nil
	}
	var cmd tea.Cmd
	*s, cmd = s.Update(msg)
	return cmd
}

var lastPopupID atomic.Int64

// newPopupID returns an ID for a popup to put on the requests it makes. A
// response that arrives after the popup closed, perhaps with another popup
// open for the same issue by then, carries the old ID and gets ignored.
func newPopupID() int64 { return lastPopupID.Add(1) }

// restartSpinner adds the spinner's tick to cmd when a change made a popup busy
// that wasn't, since the tick chain ends whenever nothing is busy.
func restartSpinner(s spinner.Model, wasBusy, busy bool, cmd tea.Cmd) tea.Cmd {
	if !wasBusy && busy {
		return tea.Batch(cmd, s.Tick)
	}
	return cmd
}
