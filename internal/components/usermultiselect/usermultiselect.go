// Package usermultiselect picks several people, found with a search box above
// the list.
package usermultiselect

import (
	"slices"

	"charm.land/huh/v2"

	"github.com/hawkbawk/jt/internal/components/usersearch"
	"github.com/hawkbawk/jt/internal/jira"
)

// Picker is a search box above the list of people it finds.
type Picker struct {
	title  string
	search usersearch.Func
	value  *[]jira.Option
	listed []jira.Option
}

// New makes a picker titled title that looks people up with search.
func New(title string, search usersearch.Func) *Picker {
	return &Picker{title: title, search: search, value: new([]jira.Option)}
}

// Value binds the people picked.
func (p *Picker) Value(people *[]jira.Option) *Picker {
	p.value = people
	return p
}

// Listed is who shows before anything is typed, besides the people picked.
// Any of them may already be picked.
func (p *Picker) Listed(people ...jira.Option) *Picker {
	p.listed = people
	return p
}

// Field builds a new search box over the list of people it finds. Space
// toggles the person under the cursor.
func (p *Picker) Field() huh.Field {
	return usersearch.New(p.title, p.search, usersearch.List{
		Rows:    p.rows,
		Checked: func(o jira.Option) bool { return jira.IndexID(*p.value, o.ID) >= 0 },
		Toggle:  p.toggle,
	})
}

func (p *Picker) toggle(o jira.Option) {
	if i := jira.IndexID(*p.value, o.ID); i >= 0 {
		*p.value = slices.Delete(slices.Clone(*p.value), i, i+1)
		return
	}
	*p.value = append(slices.Clone(*p.value), o)
}

// rows lists the people listed and picked until something is typed, then
// the search results. Picked people stay in the list either way, so they can
// be taken off whatever is typed.
func (p *Picker) rows(query string, found []jira.Option) []jira.Option {
	if query != "" {
		return slices.Concat(*p.value, found)
	}
	return slices.Concat(p.listed, *p.value)
}
