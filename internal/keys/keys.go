// Package keys holds every key binding jt matches against or shows in a help
// footer, in one Map. Screens and popups build their own key maps from it, so
// the same key means the same thing everywhere, and rebinding a key means
// changing it here.
package keys

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/evertras/bubble-table/table"
)

// Map is every binding jt uses. Rebind by changing the base bindings of a Map
// and passing it to Set, which refreshes the bindings derived from them.
//
// Bindings that share a default key but do different things (Favorite and
// Filter, Reset and Refresh) stay separate so they can be rebound apart.
type Map struct {
	// Base bindings.
	Left, Right, Up, Down, Top, Bottom key.Binding

	View, Comments, Commands, Edit, Move, Assign, Yank, Open key.Binding
	Filter, Theme, Refresh, Projects, Boards, Quit           key.Binding
	Favorite, Reset, Undo                                    key.Binding

	Dismiss, ForceQuit, Interrupt, Confirm, Deny key.Binding
	Accept, Send, Toggle, NewLine                key.Binding
	NextTab, PrevTab, ListFilter                 key.Binding
	SwitchMode, FocusResults                     key.Binding

	// Derived by Derive; changes to them are overwritten.
	Close                                          key.Binding // Dismiss, plus Quit
	Navigate, ListNavigate, TopBottom, Scroll      key.Binding // help summaries of movement
	Back, ViewResult                               key.Binding
	ToList, ToSearch, ListUp, ListDown, ToggleUser key.Binding // the user search field

	// Table is the results table's key map. It leaves out the keys jt uses
	// itself: g/G jump through the results list, and the table's filter and
	// horizontal paging aren't wanted.
	Table table.KeyMap
}

// Default returns the bindings jt ships with.
func Default() Map {
	m := Map{
		Left:   key.NewBinding(key.WithKeys("h", "left")),
		Right:  key.NewBinding(key.WithKeys("l", "right")),
		Up:     key.NewBinding(key.WithKeys("k", "up")),
		Down:   key.NewBinding(key.WithKeys("j", "down")),
		Top:    key.NewBinding(key.WithKeys("g")),
		Bottom: key.NewBinding(key.WithKeys("G")),

		View:     key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "view")),
		Comments: key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "comments")),
		Commands: key.NewBinding(key.WithKeys("C"), key.WithHelp("C", "commands")),
		Edit:     key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
		Move:     key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "move")),
		Assign:   key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "assign")),
		Yank:     key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "yank URL")),
		Open:     key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open")),
		Filter:   key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "filter")),
		Theme:    key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "theme")),
		Refresh:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Projects: key.NewBinding(key.WithKeys("ctrl+p"), key.WithHelp("ctrl+p", "projects")),
		Boards:   key.NewBinding(key.WithKeys("ctrl+b"), key.WithHelp("ctrl+b", "boards")),
		Quit:     key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		Favorite: key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "favorite")),
		Reset:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reset")),
		Undo:     key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "undo")),

		Dismiss:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close")),
		ForceQuit: key.NewBinding(key.WithKeys("ctrl+d")),
		Interrupt: key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
		Confirm:   key.NewBinding(key.WithKeys("y", "Y"), key.WithHelp("y", "yes")),
		Deny:      key.NewBinding(key.WithKeys("n", "N", "esc"), key.WithHelp("n", "no")),
		Accept:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "accept")),
		Send:      key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "send")),
		Toggle:    key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "toggle")),
		NewLine:   key.NewBinding(key.WithKeys("ctrl+j"), key.WithHelp("ctrl+j", "new line")),

		NextTab:    key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next")),
		PrevTab:    key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev")),
		ListFilter: key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),

		SwitchMode:   key.NewBinding(key.WithKeys("ctrl+t"), key.WithHelp("ctrl+t", "basic/advanced")),
		FocusResults: key.NewBinding(key.WithKeys("ctrl+o"), key.WithHelp("ctrl+o", "results")),

		Table: table.KeyMap{
			RowDown:   key.NewBinding(key.WithKeys("down", "j")),
			RowUp:     key.NewBinding(key.WithKeys("up", "k")),
			PageDown:  key.NewBinding(key.WithKeys("pgdown", "ctrl+d")),
			PageUp:    key.NewBinding(key.WithKeys("pgup", "ctrl+u")),
			PageFirst: key.NewBinding(key.WithKeys("home")),
			PageLast:  key.NewBinding(key.WithKeys("end")),
		},
	}
	return m.Derive()
}

// Derive rebuilds the bindings that combine others, from the base bindings.
func (m Map) Derive() Map {
	for _, b := range m.base() {
		if k := b.Keys(); len(k) > 0 {
			b.SetHelp(k[0], b.Help().Desc)
		}
	}
	m.Close = join(m.Dismiss, m.Quit)

	m.Navigate = summary(label(m.Left)+label(m.Down)+label(m.Up)+label(m.Right), "navigate", m.Left, m.Down, m.Up, m.Right)
	m.ListNavigate = summary(label(m.Down)+"/"+label(m.Up), "navigate", m.Down, m.Up)
	m.TopBottom = summary(label(m.Top)+"/"+label(m.Bottom), "top/bottom", m.Top, m.Bottom)
	m.Scroll = summary(arrow(m.Up)+arrow(m.Down), "scroll", m.Up, m.Down)

	m.Back = join(Describe(m.Dismiss, "edit search"), m.ListFilter, m.PrevTab)
	m.ViewResult = summary(label(m.Accept)+"/"+label(m.View), "view", m.Accept, m.View)

	m.ToList = summary(label(m.Accept), "pick", m.Accept, m.NextTab, key.NewBinding(key.WithKeys("down")))
	m.ToSearch = Describe(m.PrevTab, "search")
	m.ListUp = summary(arrow(m.Up), "up", m.Up, key.NewBinding(key.WithKeys("ctrl+p")))
	m.ListDown = summary(arrow(m.Down), "down", m.Down, key.NewBinding(key.WithKeys("ctrl+n")))
	m.ToggleUser = join(Describe(m.Toggle, "toggle"), key.NewBinding(key.WithKeys("x")))
	return m
}

// WithOverrides applies valid settings and reports invalid ones without
// changing their bindings. "..." expands to the binding's existing keys.
func (m Map) WithOverrides(settings any) (Map, error) {
	if settings == nil {
		return m.Derive(), nil
	}
	overrides, ok := settings.(map[string]any)
	if !ok {
		return m.Derive(), fmt.Errorf("keys must be a mapping of binding names to keys")
	}
	bindings := map[string]*key.Binding{
		"left": &m.Left, "right": &m.Right, "up": &m.Up, "down": &m.Down,
		"top": &m.Top, "bottom": &m.Bottom,
		"view": &m.View, "comments": &m.Comments, "commands": &m.Commands,
		"edit": &m.Edit, "move": &m.Move, "assign": &m.Assign, "yank": &m.Yank,
		"open": &m.Open, "filter": &m.Filter, "theme": &m.Theme, "refresh": &m.Refresh,
		"projects": &m.Projects, "boards": &m.Boards, "quit": &m.Quit,
		"favorite": &m.Favorite, "reset": &m.Reset, "undo": &m.Undo,
		"dismiss": &m.Dismiss, "force_quit": &m.ForceQuit, "interrupt": &m.Interrupt,
		"confirm": &m.Confirm, "deny": &m.Deny, "accept": &m.Accept, "send": &m.Send,
		"toggle": &m.Toggle, "new_line": &m.NewLine, "next_tab": &m.NextTab,
		"prev_tab": &m.PrevTab, "list_filter": &m.ListFilter, "switch_mode": &m.SwitchMode,
		"focus_results": &m.FocusResults,
	}
	names := make([]string, 0, len(overrides))
	for name := range overrides {
		names = append(names, name)
	}
	sort.Strings(names)
	var warnings []string
	for _, name := range names {
		binding, ok := bindings[name]
		if !ok {
			warnings = append(warnings, fmt.Sprintf("unknown key binding %q", name))
			continue
		}
		expanded, err := expandKeys(overrides[name], binding.Keys())
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("keys.%s: %s (using default)", name, err))
			continue
		}
		*binding = key.NewBinding(
			key.WithKeys(expanded...),
			key.WithHelp(binding.Help().Key, binding.Help().Desc),
		)
	}
	if len(warnings) > 0 {
		return m.Derive(), fmt.Errorf("%s", strings.Join(warnings, "; "))
	}
	return m.Derive(), nil
}

func expandKeys(value any, defaults []string) ([]string, error) {
	var values []any
	switch v := value.(type) {
	case string:
		values = []any{v}
	case []any:
		values = v
	default:
		return nil, fmt.Errorf("expected a key or list of keys")
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("key list is empty")
	}
	var result []string
	seen := map[string]bool{}
	add := func(k string) {
		if !seen[k] {
			result = append(result, k)
			seen[k] = true
		}
	}
	for _, value := range values {
		k, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("keys must be strings")
		}
		if k == "..." {
			for _, defaultKey := range defaults {
				add(defaultKey)
			}
		} else if validKey(k) {
			add(k)
		} else {
			return nil, fmt.Errorf("invalid key %q", k)
		}
	}
	return result, nil
}

// validKey accepts the key names Bubble Tea emits, including modifiers in its
// canonical order. Generate special-key names from Bubble Tea's own codes.
func validKey(s string) bool {
	remaining := s
	var mod tea.KeyMod
	for _, m := range []struct {
		prefix string
		mod    tea.KeyMod
	}{
		{"ctrl+", tea.ModCtrl},
		{"alt+", tea.ModAlt},
		{"shift+", tea.ModShift},
		{"meta+", tea.ModMeta},
		{"hyper+", tea.ModHyper},
		{"super+", tea.ModSuper},
	} {
		if rest, ok := strings.CutPrefix(remaining, m.prefix); ok {
			remaining = rest
			mod |= m.mod
		}
	}
	if utf8.RuneCountInString(remaining) == 1 {
		r, _ := utf8.DecodeRuneInString(remaining)
		return unicode.IsGraphic(r) && !unicode.IsSpace(r) && (tea.Key{Code: r, Mod: mod}).Keystroke() == s
	}
	for _, code := range []rune{tea.KeyEnter, tea.KeyTab, tea.KeyBackspace, tea.KeyEsc, tea.KeySpace} {
		if (tea.Key{Code: code, Mod: mod}).Keystroke() == s {
			return true
		}
	}
	for code := tea.KeyUp; code <= tea.KeyIsoLevel5Shift; code++ {
		if (tea.Key{Code: code, Mod: mod}).Keystroke() == s {
			return true
		}
	}
	return false
}

var current = Default()

// Get returns the bindings in use.
func Get() Map { return current }

// Set makes m the bindings in use, after refreshing the ones derived from its
// base bindings. Call it before building any screen.
func Set(m Map) { current = m.Derive() }

// base lists the bindings whose help label is their first key.
func (m *Map) base() []*key.Binding {
	return []*key.Binding{
		&m.Left, &m.Right, &m.Up, &m.Down, &m.Top, &m.Bottom,
		&m.View, &m.Comments, &m.Commands, &m.Edit, &m.Move, &m.Assign, &m.Yank, &m.Open,
		&m.Filter, &m.Theme, &m.Refresh, &m.Projects, &m.Boards, &m.Quit,
		&m.Favorite, &m.Reset, &m.Undo,
		&m.Dismiss, &m.Interrupt, &m.Confirm, &m.Deny, &m.Accept, &m.Send,
		&m.Toggle, &m.NewLine, &m.NextTab, &m.PrevTab, &m.ListFilter,
		&m.SwitchMode, &m.FocusResults,
	}
}

// Describe returns b with its help description replaced by desc. The label
// stays the binding's own, so help follows a rebinding.
func Describe(b key.Binding, desc string) key.Binding {
	lbl := b.Help().Key
	if lbl == "" {
		lbl = label(b)
	}
	b.SetHelp(lbl, desc)
	return b
}

// glyphs are the symbols help shows for keys that have one.
var glyphs = map[string]string{"up": "↑", "down": "↓", "left": "←", "right": "→"}

// arrow is the label for b: the glyph of its first key that has one, else its
// first key.
func arrow(b key.Binding) string {
	for _, k := range b.Keys() {
		if g, ok := glyphs[k]; ok {
			return g
		}
	}
	return label(b)
}

func label(b key.Binding) string {
	if k := b.Keys(); len(k) > 0 {
		return k[0]
	}
	return ""
}

// summary is a help-only binding for several others: it matches any of their
// keys and shows the given label and description.
func summary(lbl, desc string, from ...key.Binding) key.Binding {
	return key.NewBinding(key.WithKeys(allKeys(from...)...), key.WithHelp(lbl, desc))
}

// join is a binding that matches the keys of every binding in from, with the
// help text of first.
func join(first key.Binding, from ...key.Binding) key.Binding {
	b := key.NewBinding(key.WithKeys(allKeys(append([]key.Binding{first}, from...)...)...))
	b.SetHelp(first.Help().Key, first.Help().Desc)
	return b
}

func allKeys(from ...key.Binding) []string {
	var out []string
	seen := map[string]bool{}
	for _, b := range from {
		for _, k := range b.Keys() {
			if !seen[k] {
				seen[k] = true
				out = append(out, k)
			}
		}
	}
	return out
}
