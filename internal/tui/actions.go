package tui

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/components/issueedit"
	"github.com/hawkbawk/jt/internal/components/issuemove"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/system"
	"github.com/hawkbawk/jt/internal/tui/popup"
)

// actionDoneMsg reports the result of a yank or open.
type actionDoneMsg struct {
	status string
	err    error
}

// display returns the status line text for the result.
func (m actionDoneMsg) display() string {
	if m.err != nil {
		return styles.Error().Render(m.err.Error())
	}
	return m.status
}

// issueActions maps keys to actions on a focused issue. Both the board and the
// search results use it.
type issueActions struct {
	backend  Backend
	md       markdownRenderer
	copyText func(string) error
	openURL  func(string) error
}

func newIssueActions(b Backend) issueActions {
	return issueActions{
		backend:  b,
		md:       styles.Markdown,
		copyText: system.CopyToClipboard,
		openURL:  system.OpenInBrowser,
	}
}

// handle returns the command for msg on the given issue, which for some keys
// opens a popup. It returns nil for keys that aren't actions.
func (a issueActions) handle(msg tea.KeyPressMsg, iss jira.Issue) tea.Cmd {
	b := a.backend
	switch {
	case key.Matches(msg, keys.view):
		return popup.Open(newDetailPopup(b, iss.Key, a.md))
	case key.Matches(msg, keys.comments):
		return popup.Open(newCommentsPopup(b, iss.Key, a.md))
	case key.Matches(msg, keys.move):
		return popup.Open(issuemove.New(b, iss))
	case key.Matches(msg, keys.assign):
		return popup.Open(newAssignPopup(b, iss))
	case key.Matches(msg, keys.edit):
		return popup.Open(issueedit.New(b, iss))
	case key.Matches(msg, keys.yank):
		return runAction(a.copyText, b.IssueURL(iss.Key), "Copied URL for "+iss.Key, "copying URL")
	case key.Matches(msg, keys.open):
		return runAction(a.openURL, b.IssueURL(iss.Key), "Opened "+iss.Key, "opening browser")
	}
	return nil
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
