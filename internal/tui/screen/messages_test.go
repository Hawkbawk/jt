package screen

import (
	"github.com/hawkbawk/jt/internal/components/boardpicker"
	"github.com/hawkbawk/jt/internal/components/filterpicker"
	"github.com/hawkbawk/jt/internal/components/issueassign"
	"github.com/hawkbawk/jt/internal/components/issuecomments"
	"github.com/hawkbawk/jt/internal/components/issueedit"
	"github.com/hawkbawk/jt/internal/components/issuemove"
	"github.com/hawkbawk/jt/internal/components/projectpicker"
	"github.com/hawkbawk/jt/internal/components/themepicker"
	"github.com/hawkbawk/jt/internal/tui/modal"
	"github.com/hawkbawk/jt/internal/tui/tuimsg"
)

// The modal host finds these by their marker methods, so a misspelled one
// would leave the message with the content. These fail to compile instead.
type result interface {
	modal.ForParent
	modal.Closes
}

var (
	_ modal.Closes = tuimsg.Closed{}

	_ result = tuimsg.CommandSelected{}
	_ result = boardpicker.SelectedMsg{}
	_ result = filterpicker.SelectedMsg{}
	_ result = issueassign.Assigned{}
	_ result = issueedit.Edited{}
	_ result = issuemove.Moved{}
	_ result = projectpicker.SelectedMsg{}
	_ result = themepicker.SelectedMsg{}

	_ modal.ForParent = boardpicker.FavoriteMsg{}
	_ modal.ForParent = issuecomments.Sent{}
	_ modal.ForParent = projectpicker.FavoriteMsg{}
	_ modal.ForParent = projectpicker.LoadedMsg{}
)
