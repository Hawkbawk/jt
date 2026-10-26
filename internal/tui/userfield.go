package tui

import (
	"slices"
	"sync"
	"time"

	"charm.land/huh/v2"

	"github.com/hawkbawk/jt/internal/jira"
)

// userSearch looks up people for a user field. Options funcs run on their own
// goroutines and may ask for the same query more than once, so each query is
// fetched once and the answer kept. A query only goes out once nothing newer
// has been asked for during the debounce delay.
type userSearch struct {
	backend  Backend
	issueKey string
	delay    time.Duration

	mu     sync.Mutex
	latest string
	found  map[string]searchResult
	names  map[string]jira.Option
}

type searchResult struct {
	people []jira.Option
	err    error
}

func newUserSearch(b Backend, issueKey string) *userSearch {
	return &userSearch{
		backend:  b,
		issueKey: issueKey,
		delay:    searchDelay,
		found:    map[string]searchResult{},
		names:    map[string]jira.Option{},
	}
}

func (s *userSearch) lookup(query string) searchResult {
	s.mu.Lock()
	if r, ok := s.found[query]; ok {
		s.mu.Unlock()
		return r
	}
	s.latest = query
	s.mu.Unlock()

	time.Sleep(s.delay)

	s.mu.Lock()
	defer s.mu.Unlock()
	if r, ok := s.found[query]; ok {
		return r
	}
	if s.latest != query {
		// Superseded while waiting. huh drops the answer for an old query.
		return searchResult{}
	}
	people, err := s.backend.SearchUsers(s.issueKey, query)
	for _, o := range people {
		s.names[o.ID] = o
	}
	r := searchResult{people: people, err: err}
	s.found[query] = r
	return r
}

func (s *userSearch) name(id string) (jira.Option, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.names[id]
	return o, ok
}

// isUser reports whether the field picks people by searching for them.
func (v *fieldValue) isUser() bool {
	return v.Kind == jira.FieldUser || v.Kind == jira.FieldMultiUser
}

// chosenPeople is the ids of the people picked so far.
func (v *fieldValue) chosenPeople() []string {
	if v.Kind == jira.FieldUser {
		if v.single == "" {
			return nil
		}
		return []string{v.single}
	}
	return v.multi
}

// person finds the name of a person already on the field or found by a search.
func (v *fieldValue) person(id string) jira.Option {
	if i := slices.IndexFunc(v.Options, func(o jira.Option) bool { return o.ID == id }); i >= 0 {
		return v.Options[i]
	}
	if o, ok := v.people.name(id); ok {
		return o
	}
	return jira.Option{ID: id, Name: id}
}

// rememberPeople keeps the names of the people picked, to show them once the
// search is gone.
func (v *fieldValue) rememberPeople() {
	for _, id := range v.chosenPeople() {
		if !slices.ContainsFunc(v.Options, func(o jira.Option) bool { return o.ID == id }) {
			v.Options = append(v.Options, v.person(id))
		}
	}
}

// peopleOptions lists the people currently set until something is typed, then
// the search results. Picked people stay in a multi-select's list, since it
// drops anything that isn't in it.
func (v *fieldValue) peopleOptions() []huh.Option[string] {
	var people []jira.Option
	if v.query == "" {
		if v.Kind == jira.FieldUser && !v.Required {
			people = append(people, jira.Option{Name: "(none)"})
		}
		ids := v.chosenPeople()
		if v.Kind == jira.FieldMultiUser {
			// People taken off stay listed so they can be put back.
			ids = slices.Concat(v.Chosen, ids)
		}
		for _, id := range ids {
			if !slices.ContainsFunc(people, func(p jira.Option) bool { return p.ID == id }) {
				people = append(people, v.person(id))
			}
		}
	} else {
		if v.Kind == jira.FieldMultiUser {
			for _, id := range v.multi {
				people = append(people, v.person(id))
			}
		}
		for _, o := range v.people.lookup(v.query).people {
			if !slices.ContainsFunc(people, func(p jira.Option) bool { return p.ID == o.ID }) {
				people = append(people, o)
			}
		}
	}
	opts := make([]huh.Option[string], len(people))
	for i, o := range people {
		opts[i] = huh.NewOption(o.Name, o.ID)
	}
	return opts
}

// searchStatus says why the list of people is empty.
func (v *fieldValue) searchStatus() string {
	if v.query == "" {
		return ""
	}
	r := v.people.lookup(v.query)
	switch {
	case r.err != nil:
		return "Error: " + r.err.Error()
	case len(r.people) == 0:
		return "No matching people."
	}
	return ""
}

// peopleRev is what the lists of people bind to. huh caches lists by a hash of
// their bindings and toggles the cached options in place, so binding the query
// and picks themselves would bring back stale toggles once they return to an
// earlier state. Hash gives every change of query or picks a number that is
// never reused, so nothing cached is ever picked up again.
type peopleRev struct {
	v     *fieldValue
	n     uint64
	query string
	picks []string
}

func (r *peopleRev) Hash() (uint64, error) {
	picks := r.v.chosenPeople()
	if r.v.query != r.query || !slices.Equal(picks, r.picks) {
		r.n++
		r.query, r.picks = r.v.query, slices.Clone(picks)
	}
	return r.n, nil
}

// userFields builds a search box and the list of people it finds.
func (v *fieldValue) userFields() []huh.Field {
	search := huh.NewInput().Title(v.Name).Placeholder("Search by name or email").Value(&v.query)
	v.rev = peopleRev{v: v}
	bind := []any{&v.rev}
	if v.Kind == jira.FieldUser {
		return []huh.Field{search, huh.NewSelect[string]().
			OptionsFunc(v.peopleOptions, bind).
			DescriptionFunc(v.searchStatus, bind).
			Height(userListHeight).
			Value(&v.single)}
	}
	return []huh.Field{search, huh.NewMultiSelect[string]().
		OptionsFunc(v.peopleOptions, bind).
		DescriptionFunc(v.searchStatus, bind).
		Height(userListHeight).
		Value(&v.multi)}
}

const userListHeight = 8

// searchDelay is how long a query waits for another keystroke before it is sent.
var searchDelay = 300 * time.Millisecond
