package pane

import (
	"charm.land/bubbles/v2/list"

	"github.com/hawkbawk/jt/internal/styles"
)

// Label is a list row that shows one line of text, which is also what a
// search matches.
type Label string

// Title implements list.DefaultItem.
func (l Label) Title() string { return string(l) }

// Description implements list.DefaultItem.
func (Label) Description() string { return "" }

// FilterValue implements list.Item.
func (l Label) FilterValue() string { return string(l) }

// LabelDelegate is the themed delegate for rows of one line with no gap. Set
// it again with SetDelegate when the theme changes.
func LabelDelegate() list.DefaultDelegate {
	d := styles.ListDelegate()
	d.ShowDescription = false
	d.SetSpacing(0)
	return d
}

// Labels returns a row for each of names.
func Labels(names []string) []list.Item {
	items := make([]list.Item, len(names))
	for i, n := range names {
		items[i] = Label(n)
	}
	return items
}

// NewLabelList returns a list of one-line rows with no title, status bar,
// help or quit keys. The caller draws those and handles quitting itself.
func NewLabelList(items []list.Item, width, height int) list.Model {
	l := list.New(items, LabelDelegate(), width, height)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.DisableQuitKeybindings()
	return l
}
