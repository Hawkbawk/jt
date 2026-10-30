package usersearch

import (
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/hawkbawk/jt/internal/jira"
)

func TestLookupSkipsSupersededQueries(t *testing.T) {
	var mu sync.Mutex
	var searches []string
	b := New("People", func(query string) ([]jira.Option, error) {
		mu.Lock()
		defer mu.Unlock()
		searches = append(searches, query)
		return []jira.Option{{ID: "u2", Name: "Bo"}}, nil
	})
	b.delay = 50 * time.Millisecond
	var wg sync.WaitGroup
	for _, q := range []string{"b", "bo", "bob"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.lookup(q)
		}()
		time.Sleep(5 * time.Millisecond)
	}
	wg.Wait()
	if !slices.Equal(searches, []string{"bob"}) {
		t.Errorf("searches = %v", searches)
	}
}
