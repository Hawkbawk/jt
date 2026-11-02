package tui

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/styles"
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
		copyText: copyToClipboard,
		openURL:  openInBrowser,
	}
}

// handle returns the command for msg on the given issue. Keys that open a
// popup also return it, with the command that loads its content.
func (a issueActions) handle(msg tea.KeyPressMsg, iss jira.Issue) (popup, tea.Cmd) {
	b := a.backend
	switch {
	case key.Matches(msg, keys.view):
		return newTextPopup("Details", iss.Key, renderDetail, a.md), loadIssue(b, iss.Key)
	case key.Matches(msg, keys.comments):
		return newCommentsPopup(b, iss.Key, a.md), loadIssue(b, iss.Key)
	case key.Matches(msg, keys.move):
		return newMovePopup(b, iss), loadTransitions(b, iss)
	case key.Matches(msg, keys.assign):
		p := newAssignPane(b, iss)
		return newAssignPopup(p), p.init()
	case key.Matches(msg, keys.edit):
		return newEditPopup(b, iss), loadFields(b, iss)
	case key.Matches(msg, keys.yank):
		return nil, runAction(a.copyText, b.IssueURL(iss.Key), "Copied URL for "+iss.Key, "copying URL")
	case key.Matches(msg, keys.open):
		return nil, runAction(a.openURL, b.IssueURL(iss.Key), "Opened "+iss.Key, "opening browser")
	}
	return nil, nil
}

// issueEdit is the outcome of a change made to an issue from a popup.
type issueEdit struct {
	key    string
	status string
	err    error
}

// editResult describes a transitionDoneMsg, assignDoneMsg or fieldsSavedMsg.
func editResult(msg tea.Msg) issueEdit {
	switch msg := msg.(type) {
	case transitionDoneMsg:
		return issueEdit{key: msg.key, status: "Moved " + msg.key + " to " + msg.to, err: msg.err}
	case assignDoneMsg:
		status := "Assigned " + msg.issueKey + " to " + msg.to
		if msg.cleared {
			status = "Unassigned " + msg.issueKey
		}
		return issueEdit{key: msg.issueKey, status: status, err: msg.err}
	case fieldsSavedMsg:
		return issueEdit{key: msg.key, status: fmt.Sprintf("Updated %d %s on %s", msg.count, plural(msg.count, "field"), msg.key), err: msg.err}
	}
	panic(fmt.Sprintf("unexpected edit result %T", msg))
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
