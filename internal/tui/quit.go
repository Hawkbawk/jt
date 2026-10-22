package tui

// quitGuard implements the quit keys: ctrl+d exits at once, while ctrl+c asks
// for confirmation first.
type quitGuard struct {
	confirming bool
}

// handle reports whether key was consumed and whether the app should exit.
func (q *quitGuard) handle(key string) (consumed, quit bool) {
	switch key {
	case "ctrl+d":
		return true, true
	case "ctrl+c":
		if q.confirming {
			return true, true
		}
		q.confirming = true
		return true, false
	}
	if !q.confirming {
		return false, false
	}
	switch key {
	case "y", "Y":
		return true, true
	case "n", "N", "esc":
		q.confirming = false
	}
	return true, false
}

func (q quitGuard) view() string {
	return errorStyle.Bold(true).Padding(0, 1).Render("Quit jt? (y/n, ctrl+c again to quit)")
}
