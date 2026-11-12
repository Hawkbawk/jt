package fieldform

import (
	"charm.land/bubbles/v2/key"

	"github.com/hawkbawk/jt/internal/keys"
)

// HelpMode picks which of the help footers a KeyMap shows.
type HelpMode int

// The help footers a KeyMap shows.
const (
	HelpList HelpMode = iota
	HelpFilter
	HelpField
	HelpUserField
	HelpDiscard
	HelpSave
)

// HelpPane is the mode of the footer under the modal's list and forms.
const HelpPane HelpMode = -1

// KeyMap is the key bindings of a modal that edits fields. Mode picks the
// help it shows.
type KeyMap struct {
	Close, Blur, Confirm, Deny, Accept, Send, Undo key.Binding
	choose, apply, cancel                          key.Binding
	filter, next, back, toggle                     key.Binding

	// SaveLabel names what Send does in the help.
	SaveLabel string
	// Extra is more keys to list while browsing fields.
	Extra []key.Binding

	mode HelpMode
	form []key.Binding
}

// NewKeyMap returns the bindings with Send described as save.
func NewKeyMap() KeyMap {
	k := keys.Get()
	return KeyMap{
		Close:     k.Close,
		Blur:      k.Dismiss,
		Confirm:   k.Confirm,
		Deny:      k.Deny,
		Accept:    k.Accept,
		Send:      k.Send,
		Undo:      keys.Describe(k.Undo, "undo"),
		choose:    keys.Describe(k.ListNavigate, "select"),
		apply:     keys.Describe(k.Accept, "apply"),
		cancel:    keys.Describe(k.Dismiss, "cancel"),
		filter:    keys.Describe(k.ListFilter, "filter"),
		next:      keys.Describe(k.Accept, "next"),
		back:      keys.Describe(k.PrevTab, "back to search"),
		toggle:    keys.Describe(k.Toggle, "toggles"),
		SaveLabel: "save",
	}
}

// WithMode returns k showing the help for m.
func (k KeyMap) WithMode(m HelpMode) KeyMap {
	k.mode = m
	return k
}

func (k KeyMap) withForm(binds []key.Binding) KeyMap {
	k.form = binds
	return k
}

// ShortHelp implements help.KeyMap.
func (k KeyMap) ShortHelp() []key.Binding {
	save := keys.Describe(k.Send, k.SaveLabel)
	switch k.mode {
	case HelpSave:
		return []key.Binding{save}
	case HelpDiscard:
		return []key.Binding{keys.Describe(k.Confirm, "discard"), keys.Describe(k.Deny, "keep editing"), save}
	case HelpUserField:
		return []key.Binding{k.next, k.back, k.toggle, keys.Describe(k.Blur, "back")}
	case HelpField:
		return append([]key.Binding{keys.Describe(k.Accept, "done"), keys.Describe(k.Blur, "back")}, k.form...)
	case HelpFilter:
		return []key.Binding{keys.Describe(k.Accept, "apply filter"), keys.Describe(k.Blur, "clear"), save}
	case HelpList:
		return append([]key.Binding{keys.Describe(k.Accept, "edit"), k.Undo, k.filter}, append(k.Extra, keys.Describe(k.Blur, "close"), save)...)
	}
	return []key.Binding{k.choose, k.apply, k.cancel}
}

// FullHelp implements help.KeyMap.
func (k KeyMap) FullHelp() [][]key.Binding { return [][]key.Binding{k.ShortHelp()} }
