// Package tuimsg holds the messages and contracts that jt's screens, the
// components they show and whatever hosts those components all share. None of
// it knows how a component is shown.
package tuimsg

import tea "charm.land/bubbletea/v2"

// ThemeChanged reports that the current tint changed outside of jt, for
// example because the terminal's theme was edited.
type ThemeChanged struct{}

// Finished is implemented by the message a component ends with after it
// changed an issue. The screen behind it reloads the issue and shows the
// status.
type Finished interface {
	// IssueKey is the issue that changed.
	IssueKey() string
	// Status is what to tell the user happened.
	Status() string
}

// Final is implemented by a component's last message: its result, or
// Cancelled. Whatever shows the component stops showing it when the message
// arrives, then hands the message to its own parent.
type Final interface {
	Final()
}

// Cancelled is the Final message of a component that was dismissed without a
// result.
type Cancelled struct{}

// Final implements Final.
func (Cancelled) Final() {}

// Cancel returns the command that dismisses a component.
func Cancel() tea.Cmd {
	return func() tea.Msg { return Cancelled{} }
}

// End returns the command that ends a component with msg as its result.
func End(msg Final) tea.Cmd {
	return func() tea.Msg { return msg }
}

// Requests lets a component's host wrap the requests it makes, such as network
// loads. Components embed it and pass each request through Request. A host
// that can show a component after its responses stop mattering, like a popup
// that's been replaced, uses this to drop them. Without a host, Request
// returns the request unchanged.
type Requests struct {
	wrap func(tea.Cmd) tea.Cmd
}

// WrapRequests sets how the component's requests are wrapped.
func (r *Requests) WrapRequests(wrap func(tea.Cmd) tea.Cmd) { r.wrap = wrap }

// Request returns cmd wrapped for the host.
func (r Requests) Request(cmd tea.Cmd) tea.Cmd {
	if r.wrap == nil || cmd == nil {
		return cmd
	}
	return r.wrap(cmd)
}
