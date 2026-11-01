// Package usersearch is the field the people pickers are built on: a search
// box above the list of people it finds. It looks people up as the query
// changes.
package usersearch

import (
	"fmt"
	"io"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/hawkbawk/jt/internal/jira"
)

// Func finds the people that match query.
type Func func(query string) ([]jira.Option, error)

// Delay is how long a query waits for another keystroke before it is sent.
var Delay = 300 * time.Millisecond

// ListHeight is how many rows a list of people shows.
const ListHeight = 8

// List is how a picker lists people and what its keys do to them.
type List struct {
	// Rows is who to list for query. found is who matches it, or nobody
	// before anything is typed.
	Rows func(query string, found []jira.Option) []jira.Option
	// Checked marks the people picked. Leave it nil for a list without
	// checkboxes.
	Checked func(jira.Option) bool
	// Toggle is what space does to the person under the cursor. Leave it nil
	// when space does nothing.
	Toggle func(jira.Option)
	// Choose is what enter does to the person under the cursor, before the
	// form moves on.
	Choose func(jira.Option)
	// Start is the id of who the cursor starts on.
	Start string
}

// Field is a search box above the list of people it finds. Its rows only
// change when the query does, so toggling people keeps the list in place.
type Field struct {
	title  string
	search Func
	delay  time.Duration
	list   List

	input   textinput.Model
	listing bool
	rows    []jira.Option
	cursor  int
	top     int
	seq     int
	// found is who matched the query, once searching is done. Until then it
	// is who matched an earlier one.
	found     []jira.Option
	searching bool
	// err is why the search for the query failed.
	err error

	focused   bool
	width     int
	theme     huh.Theme
	hasDarkBg bool
}

// searchMsg says a query has waited long enough to be sent.
type searchMsg struct {
	field *Field
	seq   int
}

// foundMsg brings back who matched a query.
type foundMsg struct {
	field  *Field
	query  string
	people []jira.Option
	err    error
}

// New makes an empty search box titled title, over a list that works like l.
// It looks people up with search.
func New(title string, search Func, l List) *Field {
	in := textinput.New()
	in.Placeholder = "Search by name or email"
	f := &Field{title: title, search: search, delay: Delay, list: l, input: in}
	f.refresh()
	for i, o := range f.rows {
		if o.ID == l.Start {
			f.cursor = i
		}
	}
	f.scroll()
	return f
}

func (f *Field) query() string { return f.input.Value() }

// refresh lists people for the query, with the cursor back at the top.
func (f *Field) refresh() {
	var found []jira.Option
	if f.query() != "" {
		found = f.found
	}
	f.rows = dedupe(f.list.Rows(f.query(), found))
	f.cursor, f.top = 0, 0
}

func dedupe(people []jira.Option) []jira.Option {
	var out []jira.Option
	seen := map[string]bool{}
	for _, o := range people {
		if !seen[o.ID] {
			seen[o.ID] = true
			out = append(out, o)
		}
	}
	return out
}

// queryChanged waits to look up the new query, keeping the people listed
// until the answer comes back. Clearing the query lists people again at once.
func (f *Field) queryChanged() tea.Cmd {
	f.seq++
	f.err = nil
	f.searching = f.query() != ""
	if !f.searching {
		f.refresh()
		return nil
	}
	msg := searchMsg{field: f, seq: f.seq}
	if f.delay <= 0 {
		return f.lookup(msg)
	}
	return tea.Tick(f.delay, func(time.Time) tea.Msg { return msg })
}

// lookup sends the query, unless a newer one has replaced it.
func (f *Field) lookup(msg searchMsg) tea.Cmd {
	if msg.seq != f.seq {
		return nil
	}
	q, search := f.query(), f.search
	return func() tea.Msg {
		people, err := search(q)
		return foundMsg{field: f, query: q, people: people, err: err}
	}
}

func (f *Field) answer(msg foundMsg) {
	if !f.searching || msg.query != f.query() {
		return
	}
	f.searching = false
	if msg.err != nil {
		f.err = msg.err
		return
	}
	f.found = msg.people
	f.refresh()
}

// status says why the list of people is empty, or that it may change.
func (f *Field) status() string {
	q := f.query()
	if q == "" {
		return ""
	}
	switch {
	case f.err != nil:
		return "Error: " + f.err.Error()
	case f.searching:
		return "Searching…"
	case len(f.found) == 0:
		return "No matching people."
	}
	return ""
}

var (
	toList   = key.NewBinding(key.WithKeys("enter", "tab", "down"), key.WithHelp("enter", "pick"))
	toSearch = key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "search"))
	up       = key.NewBinding(key.WithKeys("up", "k", "ctrl+p"), key.WithHelp("↑", "up"))
	down     = key.NewBinding(key.WithKeys("down", "j", "ctrl+n"), key.WithHelp("↓", "down"))
	toggle   = key.NewBinding(key.WithKeys("space", "x"), key.WithHelp("space", "toggle"))
	choose   = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "done"))
)

// Init implements huh.Field.
func (f *Field) Init() tea.Cmd { return nil }

// Update implements huh.Field.
func (f *Field) Update(msg tea.Msg) (huh.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		f.hasDarkBg = msg.IsDark()
	case searchMsg:
		if msg.field == f {
			return f, f.lookup(msg)
		}
	case foundMsg:
		if msg.field == f {
			f.answer(msg)
		}
	case tea.KeyPressMsg:
		if f.listing {
			return f, f.updateList(msg)
		}
		return f, f.updateSearch(msg)
	default:
		if !f.listing {
			return f, f.edit(msg)
		}
	}
	return f, nil
}

func (f *Field) updateSearch(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(msg, toList):
		if len(f.rows) > 0 {
			f.listing = true
			f.input.Blur()
		}
		return nil
	case key.Matches(msg, toSearch):
		return huh.PrevField
	}
	return f.edit(msg)
}

// edit passes msg to the search box, looking people up if the query changed.
func (f *Field) edit(msg tea.Msg) tea.Cmd {
	before := f.query()
	var cmd tea.Cmd
	f.input, cmd = f.input.Update(msg)
	if f.query() == before {
		return cmd
	}
	return tea.Batch(cmd, f.queryChanged())
}

func (f *Field) updateList(msg tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(msg, toSearch) || len(f.rows) == 0:
		f.listing = false
		return f.input.Focus()
	case key.Matches(msg, up):
		f.cursor = max(0, f.cursor-1)
	case key.Matches(msg, down):
		f.cursor = min(len(f.rows)-1, f.cursor+1)
	case key.Matches(msg, toggle) && f.list.Toggle != nil:
		f.list.Toggle(f.rows[f.cursor])
	case key.Matches(msg, choose):
		if f.list.Choose != nil {
			f.list.Choose(f.rows[f.cursor])
		}
		return huh.NextField
	}
	f.scroll()
	return nil
}

// scroll keeps the cursor in view.
func (f *Field) scroll() {
	f.top = min(f.top, f.cursor)
	f.top = max(f.top, f.cursor-ListHeight+1)
}

func (f *Field) styles() *huh.FieldStyles {
	theme := f.theme
	if theme == nil {
		theme = huh.ThemeFunc(huh.ThemeCharm)
	}
	if f.focused {
		return &theme.Theme(f.hasDarkBg).Focused
	}
	return &theme.Theme(f.hasDarkBg).Blurred
}

// View implements huh.Field.
func (f *Field) View() string {
	s := f.styles()
	st := f.input.Styles()
	st.Cursor.Color = s.TextInput.Cursor.GetForeground()
	st.Focused.Prompt = s.TextInput.Prompt
	st.Focused.Text = s.TextInput.Text
	st.Focused.Placeholder = s.TextInput.Placeholder
	f.input.SetStyles(st)

	lines := []string{s.Title.Render(f.title), f.input.View()}
	if status := f.status(); status != "" {
		lines = append(lines, s.Description.Render(status))
	}
	for i := f.top; i < min(len(f.rows), f.top+ListHeight); i++ {
		lines = append(lines, f.renderRow(s, i))
	}
	return s.Base.Width(f.width).Render(strings.Join(lines, "\n"))
}

func (f *Field) renderRow(s *huh.FieldStyles, i int) string {
	o := f.rows[i]
	cursor := s.SelectSelector.String()
	if f.list.Checked != nil {
		cursor = s.MultiSelectSelector.String()
	}
	parts := []string{strings.Repeat(" ", lipgloss.Width(cursor))}
	if f.listing && i == f.cursor {
		parts[0] = cursor
	}
	style := s.UnselectedOption
	if f.list.Checked != nil {
		if f.list.Checked(o) {
			parts = append(parts, s.SelectedPrefix.String())
			style = s.SelectedOption
		} else {
			parts = append(parts, s.UnselectedPrefix.String())
		}
	} else if f.listing && i == f.cursor {
		style = s.SelectedOption
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, append(parts, style.Render(o.Name))...)
}

// Focus implements huh.Field.
func (f *Field) Focus() tea.Cmd {
	f.focused = true
	if f.listing {
		return nil
	}
	return f.input.Focus()
}

// Blur implements huh.Field.
func (f *Field) Blur() tea.Cmd {
	f.focused = false
	f.input.Blur()
	return nil
}

// KeyBinds implements huh.Field.
func (f *Field) KeyBinds() []key.Binding {
	if !f.listing {
		return []key.Binding{toList, toSearch}
	}
	binds := []key.Binding{up, down}
	if f.list.Toggle != nil {
		binds = append(binds, toggle)
	}
	return append(binds, toSearch, choose)
}

// WithTheme implements huh.Field.
func (f *Field) WithTheme(theme huh.Theme) huh.Field {
	if f.theme == nil {
		f.theme = theme
	}
	return f
}

// WithWidth implements huh.Field.
func (f *Field) WithWidth(width int) huh.Field {
	f.width = width
	prompt := lipgloss.Width(f.input.Styles().Focused.Prompt.Render(f.input.Prompt))
	f.input.SetWidth(width - f.styles().Base.GetHorizontalFrameSize() - prompt - 1)
	return f
}

// Error implements huh.Field.
func (f *Field) Error() error { return nil }

// Run implements huh.Field.
func (f *Field) Run() error { return huh.Run(f) }

// RunAccessible implements huh.Field.
func (f *Field) RunAccessible(w io.Writer, _ io.Reader) error {
	_, err := fmt.Fprintln(w, "Searching for people isn't supported in accessible mode.")
	return err
}

// Skip implements huh.Field.
func (f *Field) Skip() bool { return false }

// Zoom implements huh.Field.
func (f *Field) Zoom() bool { return false }

// WithKeyMap implements huh.Field. The field keeps its own keys.
func (f *Field) WithKeyMap(*huh.KeyMap) huh.Field { return f }

// WithHeight implements huh.Field. The list is always ListHeight rows.
func (f *Field) WithHeight(int) huh.Field { return f }

// WithPosition implements huh.Field.
func (f *Field) WithPosition(huh.FieldPosition) huh.Field { return f }

// GetKey implements huh.Field.
func (f *Field) GetKey() string { return "" }

// GetValue implements huh.Field. The pickers bind their own values.
func (f *Field) GetValue() any { return nil }
