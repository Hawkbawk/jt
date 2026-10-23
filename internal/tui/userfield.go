package tui

import (
	"slices"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/hawkbawk/jt/internal/jira"
)

// userSearchDelay waits for a pause in typing before searching.
var userSearchDelay = 250 * time.Millisecond

// The bindings that move the cursor and the ones that only appear in the help
// line, which are matched through keys.accept and keys.blur.
var (
	userUp   = key.NewBinding(key.WithKeys("up", "ctrl+k"))
	userDown = key.NewBinding(key.WithKeys("down", "ctrl+j"))

	userSearchHelp = key.NewBinding(key.WithHelp("type", "search"))
	userMoveHelp   = key.NewBinding(key.WithHelp("↑/↓", "move"))
	userPickHelp   = key.NewBinding(key.WithHelp("enter", "choose"))
	userToggleHelp = key.NewBinding(key.WithHelp("enter", "toggle"))
	userDoneHelp   = key.NewBinding(key.WithHelp("esc", "done"))
	userCancelHelp = key.NewBinding(key.WithHelp("esc", "cancel"))
)

// userHelp is the key hints for a user field.
type userHelp []key.Binding

// ShortHelp implements help.KeyMap.
func (h userHelp) ShortHelp() []key.Binding { return h }

// FullHelp implements help.KeyMap.
func (h userHelp) FullHelp() [][]key.Binding { return [][]key.Binding{h} }

type userSearchedMsg struct {
	// from is the field that searched. Undoing a field replaces it, and the
	// replacement must not take results meant for the old one.
	from    *userField
	seq     int64
	results []jira.Option
	err     error
}

// userField picks people by searching for them as you type. Letters go to the
// search box, so only the arrow keys move the cursor.
type userField struct {
	jira.EditField
	backend  Backend
	issueKey string

	input   textinput.Model
	help    help.Model
	list    choiceList
	results []jira.Option
	// searches counts the searches started. Only the newest one's results are
	// used, and a search that waits out the debounce behind a newer one is
	// skipped.
	searches  atomic.Int64
	searching bool
	err       error
}

func newUserField(f jira.EditField, b Backend, issueKey string) *userField {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "Search by name or email"
	ti.Cursor.SetMode(cursor.CursorStatic)
	ti.Focus()
	u := &userField{EditField: f, backend: b, issueKey: issueKey, input: ti, help: newHelp()}
	if f.Kind == jira.FieldUser {
		u.list.cursor = max(0, slices.IndexFunc(u.rows(), func(o jira.Option) bool {
			return o.ID == "" && len(f.Chosen) == 0 || slices.Contains(f.Chosen, o.ID)
		}))
	}
	return u
}

func (f *userField) value() jira.EditField { return f.EditField }

// rows lists the people currently set until something is typed, then the
// search results. A single user field that isn't required can also be cleared.
func (f *userField) rows() []jira.Option {
	var rows []jira.Option
	if f.input.Value() == "" {
		if f.Kind == jira.FieldUser && !f.Required {
			rows = append(rows, jira.Option{Name: "(none)"})
		}
		for _, o := range f.Options {
			if slices.Contains(f.Chosen, o.ID) {
				rows = append(rows, o)
			}
		}
		return rows
	}
	return f.results
}

func (f *userField) update(msg tea.KeyMsg) (tea.Cmd, bool) {
	rows := f.rows()
	switch {
	case key.Matches(msg, keys.blur):
		return nil, true
	case key.Matches(msg, userUp):
		f.list.cursor = max(f.list.cursor-1, 0)
	case key.Matches(msg, userDown):
		f.list.cursor = min(f.list.cursor+1, max(len(rows)-1, 0))
	case key.Matches(msg, keys.accept):
		if f.list.cursor >= len(rows) {
			return nil, f.Kind == jira.FieldUser
		}
		return nil, f.choose(rows[f.list.cursor])
	default:
		before := f.input.Value()
		var cmd tea.Cmd
		f.input, cmd = f.input.Update(msg)
		if f.input.Value() == before {
			return cmd, false
		}
		return tea.Batch(cmd, f.search()), false
	}
	return nil, false
}

// choose applies the picked row, and reports whether editing is done.
func (f *userField) choose(o jira.Option) bool {
	if f.Kind == jira.FieldUser {
		f.Chosen = nil
		if o.ID != "" {
			f.Chosen = []string{o.ID}
			f.remember(o)
		}
		return true
	}
	if i := slices.Index(f.Chosen, o.ID); i >= 0 {
		f.Chosen = slices.Delete(f.Chosen, i, i+1)
	} else {
		f.Chosen = append(f.Chosen, o.ID)
		f.remember(o)
	}
	return false
}

// remember keeps a picked person's name, to show it once the search is gone.
func (f *userField) remember(o jira.Option) {
	if !slices.ContainsFunc(f.Options, func(x jira.Option) bool { return x.ID == o.ID }) {
		f.Options = append(f.Options, o)
	}
}

func (f *userField) search() tea.Cmd {
	query := f.input.Value()
	seq := f.searches.Add(1)
	f.list.cursor, f.err = 0, nil
	if query == "" {
		f.results, f.searching = nil, false
		return nil
	}
	f.searching = true
	b, issueKey := f.backend, f.issueKey
	return func() tea.Msg {
		time.Sleep(userSearchDelay)
		if f.searches.Load() != seq {
			return nil
		}
		results, err := b.SearchUsers(issueKey, query)
		return userSearchedMsg{from: f, seq: seq, results: results, err: err}
	}
}

// receive takes the results of a search, unless a newer one has started.
func (f *userField) receive(msg userSearchedMsg) {
	if msg.from != f || msg.seq != f.searches.Load() {
		return
	}
	f.searching, f.err, f.results = false, msg.err, msg.results
}

func (f *userField) view(innerW, height int) []string {
	border := inputBorder.BorderForeground(highlight)
	f.input.Width = innerW - border.GetHorizontalFrameSize() - 1
	lines := []string{border.Render(f.input.View())}
	height -= len(lines)

	rows := f.rows()
	labels := make([]string, len(rows))
	for i, o := range rows {
		labels[i] = o.Name
		if f.Kind == jira.FieldMultiUser {
			mark := "[ ] "
			if slices.Contains(f.Chosen, o.ID) {
				mark = "[x] "
			}
			labels[i] = mark + o.Name
		}
	}
	switch {
	case f.err != nil:
		return append(lines, errorStyle.Width(innerW).Render("Error: "+f.err.Error()))
	case f.searching:
		return append(lines, dimStyle.Render("Searching…"))
	case len(rows) == 0 && f.input.Value() != "":
		return append(lines, dimStyle.Render("No matching people."))
	}
	return append(lines, f.list.renderWindow(labels, innerW, height)...)
}

func (f *userField) hint() string {
	if f.Kind == jira.FieldMultiUser {
		return f.help.View(userHelp{userSearchHelp, userMoveHelp, userToggleHelp, userDoneHelp})
	}
	return f.help.View(userHelp{userSearchHelp, userMoveHelp, userPickHelp, userCancelHelp})
}
