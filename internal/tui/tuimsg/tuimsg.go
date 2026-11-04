// Package tuimsg holds messages that the screens and the components they host
// both handle.
package tuimsg

// ThemeChanged reports that the current tint changed outside of jt, for
// example because the terminal's theme was edited.
type ThemeChanged struct{}

// Finished is implemented by the message a component closes its popup with
// after it changed an issue. The screen behind the popup reloads the issue and
// shows the status.
type Finished interface {
	// IssueKey is the issue that changed.
	IssueKey() string
	// Status is what to tell the user happened.
	Status() string
}
