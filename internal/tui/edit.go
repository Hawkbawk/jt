package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/tui/popup"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
)

// issueEditedMsg is how the move and assign popups report a change to an issue.
type issueEditedMsg struct {
	key    string
	status string
}

var _ tuimsg.Finished = issueEditedMsg{}

// IssueKey implements tuimsg.Finished.
func (m issueEditedMsg) IssueKey() string { return m.key }

// Status implements tuimsg.Finished.
func (m issueEditedMsg) Status() string { return m.status }

// closeEdited closes the popup after a change to an issue.
func closeEdited(issueKey, status string) tea.Cmd {
	return popup.CloseWith(issueEditedMsg{key: issueKey, status: status})
}
