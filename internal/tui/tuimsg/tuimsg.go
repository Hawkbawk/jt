// Package tuimsg holds messages that the screens and the components they host
// both handle.
package tuimsg

// ThemeChanged reports that the current tint changed outside of jt, for
// example because the terminal's theme was edited.
type ThemeChanged struct{}
