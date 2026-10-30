// Package userselect picks one person, found with a search box above the list.
package userselect

import (
	"charm.land/huh/v2"

	"github.com/hawkbawk/jt/internal/components/usersearch"
	"github.com/hawkbawk/jt/internal/jira"
)

// Picker is a search box above the list of people it finds.
type Picker struct {
	box      *usersearch.Box
	required bool
	value    *string
	listed   []jira.Option
}

// New makes a picker titled title that looks people up with search.
func New(title string, search usersearch.Func) *Picker {
	return &Picker{box: usersearch.New(title, search), value: new(string)}
}

// Value binds the id of the person picked, which is empty for nobody.
func (p *Picker) Value(id *string) *Picker {
	p.value = id
	return p
}

// Required leaves out the choice of nobody.
func (p *Picker) Required(required bool) *Picker {
	p.required = required
	return p
}

// Listed is who shows before anything is typed, besides the person picked.
// Any of them may already be picked.
func (p *Picker) Listed(people ...jira.Option) *Picker {
	p.box.Know(people...)
	p.listed = people
	return p
}

// Picked is the person picked, or the zero Option for nobody.
func (p *Picker) Picked() jira.Option {
	if *p.value == "" {
		return jira.Option{}
	}
	return p.box.Person(*p.value)
}

// Fields builds the search box and the list of people it finds. The form they
// go in should turn off filtering, since the search box does that job.
func (p *Picker) Fields() []huh.Field {
	search := p.box.Field()
	bind := []any{p.box.Watch(func() []string { return []string{*p.value} })}
	return []huh.Field{search, huh.NewSelect[string]().
		OptionsFunc(p.options, bind).
		DescriptionFunc(p.box.Status, bind).
		Height(usersearch.ListHeight).
		Value(p.value)}
}

// options lists the people listed and picked until something is typed, then
// the search results.
func (p *Picker) options() []huh.Option[string] {
	if p.box.Query() != "" {
		return usersearch.Options(p.box.Results())
	}
	var people []jira.Option
	if !p.required {
		people = append(people, jira.Option{Name: "(none)"})
	}
	people = append(people, p.listed...)
	if *p.value != "" {
		people = append(people, p.Picked())
	}
	return usersearch.Options(people)
}
