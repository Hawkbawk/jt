package layout

import (
	"slices"

	"charm.land/bubbles/v2/key"
)

// KeyMap is the help footer most components share: the bindings the component
// leads with, then the focused field's own, then the dismiss binding. It leaves
// the dismiss binding out while the field uses its key. Components with their
// own bindings to match embed it.
//
// The With methods return a changed copy, so a component keeps one KeyMap and
// configures a copy of it for each render.
type KeyMap struct {
	dismiss     key.Binding
	lead, form  []key.Binding
	hideDismiss bool
}

// NewKeyMap returns a KeyMap whose dismiss binding is dismiss.
func NewKeyMap(dismiss key.Binding) KeyMap { return KeyMap{dismiss: dismiss} }

// Dismiss is the binding that ends the component without a result.
func (k KeyMap) Dismiss() key.Binding { return k.dismiss }

// WithLead sets the bindings shown before the field's own.
func (k KeyMap) WithLead(binds ...key.Binding) KeyMap {
	k.lead = slices.Clone(binds)
	return k
}

// WithForm sets the focused field's bindings. consumesDismiss says the field
// uses the dismiss key itself, which hides the dismiss binding.
func (k KeyMap) WithForm(binds []key.Binding, consumesDismiss bool) KeyMap {
	k.form = binds
	k.hideDismiss = consumesDismiss
	return k
}

// WithDismissShown sets whether the dismiss binding shows, for components
// that can't always be dismissed.
func (k KeyMap) WithDismissShown(shown bool) KeyMap {
	k.hideDismiss = !shown
	return k
}

// ShortHelp implements help.KeyMap.
func (k KeyMap) ShortHelp() []key.Binding {
	binds := slices.Concat(k.lead, k.form)
	if !k.hideDismiss {
		binds = append(binds, k.dismiss)
	}
	return binds
}

// FullHelp implements help.KeyMap.
func (k KeyMap) FullHelp() [][]key.Binding { return [][]key.Binding{k.ShortHelp()} }
