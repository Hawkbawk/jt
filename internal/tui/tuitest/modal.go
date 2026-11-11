package tuitest

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/tui/modal"
)

// ModalView is m's view as a modal shows it, with the title and footer the
// host draws, width wide and without styling.
func ModalView(m tea.Model, width int) string {
	return ansi.Strip(modal.Chrome(m, width, modal.WithTitleStyle(styles.Title), modal.WithHelp(styles.Help)))
}
