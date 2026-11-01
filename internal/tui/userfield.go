package tui

import (
	"charm.land/huh/v2"

	"github.com/hawkbawk/jt/internal/components/usermultiselect"
	"github.com/hawkbawk/jt/internal/components/usersearch"
	"github.com/hawkbawk/jt/internal/components/userselect"
	"github.com/hawkbawk/jt/internal/jira"
)

// searchUsers looks up people who can be put on the issue.
func searchUsers(b Backend, issueKey string) usersearch.Func {
	return func(query string) ([]jira.Option, error) { return b.SearchUsers(issueKey, query) }
}

// isUser reports whether the field picks people by searching for them.
func (v *fieldValue) isUser() bool {
	return v.Kind == jira.FieldUser || v.Kind == jira.FieldMultiUser
}

// peoplePicker builds the fields that edit a user field.
type peoplePicker interface {
	Field() huh.Field
}

// newPeoplePicker builds the picker that edits a user field. The people on
// the field when it loaded stay listed so they can be put back.
func (v *fieldValue) newPeoplePicker(search usersearch.Func) peoplePicker {
	if v.Kind == jira.FieldMultiUser {
		return usermultiselect.New(v.Name, search).Value(&v.multi).Listed(v.Chosen...)
	}
	return userselect.New(v.Name, search).Value(&v.single).Required(v.Required).Listed(v.Chosen...)
}
