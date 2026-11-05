package popup

import (
	"slices"

	"charm.land/bubbles/v2/key"
)

// KeyMap is the help footer most popups share: the bindings the popup leads
// with, then the focused field's own, then the close binding. It leaves the
// close binding out while the field uses its key. Popups with their own
// bindings to match embed it.
//
// The With methods return a changed copy, so a popup keeps one KeyMap and
// configures a copy of it for each render.
type KeyMap struct {
	closeKey   key.Binding
	lead, form []key.Binding
	hideClose  bool
}

// NewKeyMap returns a KeyMap whose close binding is closeKey.
func NewKeyMap(closeKey key.Binding) KeyMap { return KeyMap{closeKey: closeKey} }

// Close is the binding that dismisses the popup.
func (k KeyMap) Close() key.Binding { return k.closeKey }

// WithLead sets the bindings shown before the field's own.
func (k KeyMap) WithLead(binds ...key.Binding) KeyMap {
	k.lead = slices.Clone(binds)
	return k
}

// WithForm sets the focused field's bindings. consumesClose says the field
// uses the close key itself, which hides the close binding.
func (k KeyMap) WithForm(binds []key.Binding, consumesClose bool) KeyMap {
	k.form = binds
	k.hideClose = consumesClose
	return k
}

// WithCloseShown sets whether the close binding shows, for popups that can't
// always be closed.
func (k KeyMap) WithCloseShown(shown bool) KeyMap {
	k.hideClose = !shown
	return k
}

// ShortHelp implements help.KeyMap.
func (k KeyMap) ShortHelp() []key.Binding {
	binds := slices.Concat(k.lead, k.form)
	if !k.hideClose {
		binds = append(binds, k.closeKey)
	}
	return binds
}

// FullHelp implements help.KeyMap.
func (k KeyMap) FullHelp() [][]key.Binding { return [][]key.Binding{k.ShortHelp()} }
