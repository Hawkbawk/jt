package issueview

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/hawkbawk/jt/internal/termimage"
)

// reset empties the cache, so tests sharing imageStore stay apart.
func (c *imageCache) reset() { c.flights.Clear() }

func TestCacheLoadsOnceForConcurrentAsks(t *testing.T) {
	var c imageCache
	var loads atomic.Int32
	release := make(chan struct{})
	img := &termimage.Image{}
	var loadErr error
	load := func() (*termimage.Image, error) { loads.Add(1); <-release; return img, loadErr }

	var wg sync.WaitGroup
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got, err := c.get("a", load); err != nil || got != img {
				t.Errorf("get = %v, %v", got, err)
			}
		}()
	}
	close(release)
	wg.Wait()
	if _, err := c.get("a", load); err != nil || loads.Load() != 1 {
		t.Errorf("loaded %d times", loads.Load())
	}
}

func TestCacheDoesNotKeepFailures(t *testing.T) {
	var c imageCache
	var loads int
	fail := func() (*termimage.Image, error) { loads++; return nil, errors.New("boom") }
	for range 2 {
		if _, err := c.get("x", fail); err == nil {
			t.Fatal("expected the failure")
		}
	}
	if loads != 2 {
		t.Errorf("a failure was cached, loads = %d", loads)
	}
}
