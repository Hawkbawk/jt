package popup

import (
	"slices"
	"testing"

	"charm.land/bubbles/v2/key"
)

func helpKeys(k KeyMap) []string {
	var out []string
	for _, b := range k.ShortHelp() {
		out = append(out, b.Help().Key)
	}
	return out
}

func TestKeyMapHelpOrderAndClose(t *testing.T) {
	bind := func(k string) key.Binding { return key.NewBinding(key.WithKeys(k), key.WithHelp(k, k)) }
	base := NewKeyMap(bind("esc")).WithLead(bind("a"), bind("b"))
	form := []key.Binding{bind("x")}

	if got := helpKeys(base.WithForm(form, false)); !slices.Equal(got, []string{"a", "b", "x", "esc"}) {
		t.Fatalf("help should list lead, form, then close, got %v", got)
	}
	if got := helpKeys(base.WithForm(form, true)); !slices.Equal(got, []string{"a", "b", "x"}) {
		t.Fatalf("close should hide while the form uses it, got %v", got)
	}
	if got := helpKeys(base.WithCloseShown(false)); !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("close should hide when not shown, got %v", got)
	}
	if got := helpKeys(base); !slices.Equal(got, []string{"a", "b", "esc"}) {
		t.Fatalf("With methods should leave the original alone, got %v", got)
	}
}
