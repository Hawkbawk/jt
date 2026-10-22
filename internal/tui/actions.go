package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// actionDoneMsg reports the result of a yank or open.
type actionDoneMsg struct {
	status string
	err    error
}

// display returns the status line text for the result.
func (m actionDoneMsg) display() string {
	if m.err != nil {
		return errorStyle.Render(m.err.Error())
	}
	return m.status
}

// issueActions maps board keys to actions on the focused issue.
type issueActions struct {
	backend  Backend
	md       markdownRenderer
	copyText func(string) error
	openURL  func(string) error
}

func newIssueActions(b Backend) issueActions {
	return issueActions{
		backend:  b,
		md:       glamourRenderer(glamourStyle()),
		copyText: copyToClipboard,
		openURL:  openInBrowser,
	}
}

// handle returns the command for key on the given issue. Keys that open a
// popup also return it, with the command that loads its content.
func (a issueActions) handle(key, issueKey string) (popup, tea.Cmd) {
	switch key {
	case "v":
		return newTextPopup("Details", issueKey, renderDetail, a.md), loadIssue(a.backend, issueKey)
	case "c":
		return newTextPopup("Comments", issueKey, renderComments, a.md), loadIssue(a.backend, issueKey)
	case "m":
		return newMovePopup(a.backend, issueKey), loadTransitions(a.backend, issueKey)
	case "y":
		return nil, runAction(a.copyText, a.backend.IssueURL(issueKey), "Copied URL for "+issueKey, "copying URL")
	case "o":
		return nil, runAction(a.openURL, a.backend.IssueURL(issueKey), "Opened "+issueKey, "opening browser")
	}
	return nil, nil
}

// runAction calls fn with url off the update loop and reports the outcome.
func runAction(fn func(string) error, url, status, doing string) tea.Cmd {
	return func() tea.Msg {
		if err := fn(url); err != nil {
			return actionDoneMsg{err: fmt.Errorf("%s: %w", doing, err)}
		}
		return actionDoneMsg{status: status}
	}
}
