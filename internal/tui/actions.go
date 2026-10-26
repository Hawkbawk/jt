package tui

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
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
		md:       glamourRenderer(),
		copyText: copyToClipboard,
		openURL:  openInBrowser,
	}
}

// handle returns the command for msg on the given issue. Keys that open a
// popup also return it, with the command that loads its content.
func (a issueActions) handle(msg tea.KeyPressMsg, issueKey string) (popup, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.view):
		return newTextPopup("Details", issueKey, renderDetail, a.md), loadIssue(a.backend, issueKey)
	case key.Matches(msg, keys.comments):
		return newCommentsPopup(a.backend, issueKey, a.md), loadIssue(a.backend, issueKey)
	case key.Matches(msg, keys.yank):
		return nil, runAction(a.copyText, a.backend.IssueURL(issueKey), "Copied URL for "+issueKey, "copying URL")
	case key.Matches(msg, keys.open):
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
