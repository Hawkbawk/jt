// Package usersearch is the search box the people pickers sit under. It
// looks people up as the query changes and remembers the names it finds.
package usersearch

import (
	"slices"
	"sync"
	"time"

	"charm.land/huh/v2"

	"github.com/hawkbawk/jt/internal/jira"
)

// Func finds the people that match query.
type Func func(query string) ([]jira.Option, error)

// Delay is how long a query waits for another keystroke before it is sent.
var Delay = 300 * time.Millisecond

// ListHeight is how many rows a list of people shows.
const ListHeight = 8

// Box is a search box for people. Input is debounced and results cached.
type Box struct {
	title  string
	search Func
	delay  time.Duration
	query  string
	// known has the names of people the caller already knows about, by id.
	known map[string]string

	mu     sync.Mutex
	latest string
	found  map[string]result
	names  map[string]jira.Option
}

type result struct {
	people []jira.Option
	err    error
}

// New makes a box titled title that looks people up with search.
func New(title string, search Func) *Box {
	return &Box{
		title:  title,
		search: search,
		delay:  Delay,
		known:  map[string]string{},
		found:  map[string]result{},
		names:  map[string]jira.Option{},
	}
}

// Know adds the names of people the caller already knows about.
func (b *Box) Know(people ...jira.Option) {
	for _, o := range people {
		b.known[o.ID] = o.Name
	}
}

// Field starts a new, empty search box.
func (b *Box) Field() *huh.Input {
	b.query = ""
	return huh.NewInput().Title(b.title).Placeholder("Search by name or email").Value(&b.query)
}

// Query is what has been typed in the box.
func (b *Box) Query() string { return b.query }

// Results is who matches the query, or nobody before anything is typed.
func (b *Box) Results() []jira.Option {
	if b.query == "" {
		return nil
	}
	return b.lookup(b.query).people
}

// Status says why the list of people is empty.
func (b *Box) Status() string {
	if b.query == "" {
		return ""
	}
	r := b.lookup(b.query)
	switch {
	case r.err != nil:
		return "Error: " + r.err.Error()
	case len(r.people) == 0:
		return "No matching people."
	}
	return ""
}

// Person finds the name of a person the caller knows or a search found.
func (b *Box) Person(id string) jira.Option {
	if name, ok := b.known[id]; ok {
		return jira.Option{ID: id, Name: name}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if o, ok := b.names[id]; ok {
		return o
	}
	return jira.Option{ID: id, Name: id}
}

func (b *Box) lookup(query string) result {
	b.mu.Lock()
	if r, ok := b.found[query]; ok {
		b.mu.Unlock()
		return r
	}
	b.latest = query
	b.mu.Unlock()

	time.Sleep(b.delay)

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.latest != query {
		// Superseded while waiting. huh drops the answer for an old query.
		return result{}
	}
	if r, ok := b.found[query]; ok {
		return r
	}
	people, err := b.search(query)
	for _, o := range people {
		b.names[o.ID] = o
	}
	r := result{people: people, err: err}
	b.found[query] = r
	return r
}

// Watch gives what a list of people binds to, so it updates when the query or
// picks change. Call it again with each new Field.
//
// huh caches lists by a hash of their bindings and toggles the cached options
// in place, so binding the query and picks themselves would bring back stale
// toggles once they return to an earlier state. The hash gives every change
// of query or picks a number that is never reused, so nothing cached is ever
// picked up again.
func (b *Box) Watch(picks func() []string) any {
	return &rev{box: b, picks: picks}
}

type rev struct {
	box   *Box
	picks func() []string
	n     uint64
	query string
	last  []string
}

func (r *rev) Hash() (uint64, error) {
	picks := r.picks()
	if r.box.query != r.query || !slices.Equal(picks, r.last) {
		r.n++
		r.query, r.last = r.box.query, slices.Clone(picks)
	}
	return r.n, nil
}

// Options lists people for a select, leaving out repeats.
func Options(people []jira.Option) []huh.Option[string] {
	var opts []huh.Option[string]
	seen := map[string]bool{}
	for _, o := range people {
		if !seen[o.ID] {
			seen[o.ID] = true
			opts = append(opts, huh.NewOption(o.Name, o.ID))
		}
	}
	return opts
}
