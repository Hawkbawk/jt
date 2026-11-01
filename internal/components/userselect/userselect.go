// Package userselect picks one person, found with a search box above the list.
package userselect

import (
	"charm.land/huh/v2"

	"github.com/hawkbawk/jt/internal/components/usersearch"
	"github.com/hawkbawk/jt/internal/jira"
)

// nobody is the row for picking no one.
var nobody = jira.Option{Name: "(none)"}

// Picker is a search box above the list of people it finds.
type Picker struct {
	title    string
	search   usersearch.Func
	required bool
	value    *jira.Option
	listed   []jira.Option
}

// New makes a picker titled title that looks people up with search.
func New(title string, search usersearch.Func) *Picker {
	return &Picker{title: title, search: search, value: new(jira.Option)}
}

// Value binds the person picked, which is the zero Option for nobody.
func (p *Picker) Value(o *jira.Option) *Picker {
	p.value = o
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
	p.listed = people
	return p
}

// Field builds a new search box over the list of people it finds. Enter
// picks the person under the cursor.
func (p *Picker) Field() huh.Field {
	return usersearch.New(p.title, p.search, usersearch.List{
		Rows:   p.rows,
		Choose: p.choose,
		Start:  p.value.ID,
	})
}

func (p *Picker) choose(o jira.Option) {
	if o == nobody {
		o = jira.Option{}
	}
	*p.value = o
}

// rows lists the people listed and picked until something is typed, then
// the search results.
func (p *Picker) rows(query string, found []jira.Option) []jira.Option {
	if query != "" {
		return found
	}
	var people []jira.Option
	if !p.required {
		people = append(people, nobody)
	}
	people = append(people, p.listed...)
	if p.value.ID != "" {
		people = append(people, *p.value)
	}
	return people
}
