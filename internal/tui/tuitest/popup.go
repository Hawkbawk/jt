package tuitest

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/popup"
)

// PopupView is m's view as a popup shows it, with the title and footer the
// host draws, width wide and without styling.
func PopupView(m tea.Model, width int) string {
	return ansi.Strip(popup.Chrome(m, width, popup.WithTitleStyle(styles.Title), popup.WithHelp(styles.Help)))
}
