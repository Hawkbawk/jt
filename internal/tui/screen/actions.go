package screen

import (
	"fmt"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/hawkbawk/jt/internal/components/issueassign"
	"github.com/hawkbawk/jt/internal/components/issuecomments"
	"github.com/hawkbawk/jt/internal/components/issuedetail"
	"github.com/hawkbawk/jt/internal/components/issueedit"
	"github.com/hawkbawk/jt/internal/components/issuemove"
	"github.com/hawkbawk/jt/internal/components/issueview"
	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/keys"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/system"
	"github.com/hawkbawk/jt/internal/tui/modal"
)

// ActionDoneMsg reports the result of a yank or open.
type ActionDoneMsg struct {
	status string
	err    error
}

// Display returns the status line text for the result.
func (m ActionDoneMsg) Display() string {
	if m.err != nil {
		return styles.Error().Render(m.err.Error())
	}
	return m.status
}

// IssueActions maps keys to actions on a focused issue. Both the board and the
// search results use it.
type IssueActions struct {
	backend  IssueBackend
	MD       issueview.Markdown
	CopyText func(string) error
	OpenURL  func(string) error
}

// NewIssueActions returns the actions for issues in b.
func NewIssueActions(b IssueBackend) IssueActions {
	return IssueActions{
		backend:  b,
		MD:       styles.Markdown,
		CopyText: system.CopyToClipboard,
		OpenURL:  system.OpenInBrowser,
	}
}

// Handle returns the command for msg on the given issue, which for some keys
// opens a modal. It returns nil for keys that aren't actions.
func (a IssueActions) Handle(msg tea.KeyPressMsg, iss jira.Issue) tea.Cmd {
	b := a.backend
	switch {
	case key.Matches(msg, keys.Get().View):
		return modal.Open(issuedetail.New(b, iss.Key, a.MD))
	case key.Matches(msg, keys.Get().Comments):
		return modal.Open(issuecomments.New(b, iss.Key, a.MD))
	case key.Matches(msg, keys.Get().Move):
		return modal.Open(issuemove.New(b, iss))
	case key.Matches(msg, keys.Get().Assign):
		return modal.Open(issueassign.New(b, iss))
	case key.Matches(msg, keys.Get().Edit):
		return modal.Open(issueedit.New(b, iss))
	case key.Matches(msg, keys.Get().Yank):
		return runAction(a.CopyText, b.IssueURL(iss.Key), "Copied URL for "+iss.Key, "copying URL")
	case key.Matches(msg, keys.Get().Open):
		return runAction(a.OpenURL, b.IssueURL(iss.Key), "Opened "+iss.Key, "opening browser")
	}
	return nil
}

// runAction calls fn with url off the update loop and reports the outcome.
func runAction(fn func(string) error, url, status, doing string) tea.Cmd {
	return func() tea.Msg {
		if err := fn(url); err != nil {
			return ActionDoneMsg{err: fmt.Errorf("%s: %w", doing, err)}
		}
		return ActionDoneMsg{status: status}
	}
}
