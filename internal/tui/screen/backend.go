package screen

import (
	"github.com/hawkbawk/jt/internal/components/issueassign"
	"github.com/hawkbawk/jt/internal/components/issuecomments"
	"github.com/hawkbawk/jt/internal/components/issuecreate"
	"github.com/hawkbawk/jt/internal/components/issueedit"
	"github.com/hawkbawk/jt/internal/components/issuemove"
	"github.com/hawkbawk/jt/internal/components/issueview"
)

// IssueBackend is the Jira functionality the issue actions and their modals
// depend on.
type IssueBackend interface {
	issueview.Backend
	issuecomments.Backend
	issuemove.Backend
	issueassign.Backend
	issueedit.Backend
	issuecreate.Backend
	IssueURL(key string) string
}
