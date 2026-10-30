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
	box    *usersearch.Box
	value  *[]string
	listed []jira.Option
}

// New makes a picker titled title that looks people up with search.
func New(title string, search usersearch.Func) *Picker {
	return &Picker{box: usersearch.New(title, search), value: new([]string)}
}

// Value binds the ids of the people picked.
func (p *Picker) Value(ids *[]string) *Picker {
	p.value = ids
	return p
}

// Listed is who shows before anything is typed, besides the people picked.
// Any of them may already be picked.
func (p *Picker) Listed(people ...jira.Option) *Picker {
	p.box.Know(people...)
	p.listed = people
	return p
}

// Picked is the people picked so far.
func (p *Picker) Picked() []jira.Option {
	people := make([]jira.Option, len(*p.value))
	for i, id := range *p.value {
		people[i] = p.box.Person(id)
	}
	return people
}

// Fields builds the search box and the list of people it finds. The form they
// go in should turn off filtering, since the search box does that job.
func (p *Picker) Fields() []huh.Field {
	search := p.box.Field()
	bind := []any{p.box.Watch(func() []string { return *p.value })}
	return []huh.Field{search, huh.NewMultiSelect[string]().
		OptionsFunc(p.options, bind).
		DescriptionFunc(p.box.Status, bind).
		Height(usersearch.ListHeight).
		Value(p.value)}
}

// options lists the people listed and picked until something is typed, then
// the search results. Picked people stay in the list either way, since a
// multi-select drops anything that isn't in it.
func (p *Picker) options() []huh.Option[string] {
	if p.box.Query() != "" {
		return usersearch.Options(slices.Concat(p.Picked(), p.box.Results()))
	}
	return usersearch.Options(slices.Concat(p.listed, p.Picked()))
}
