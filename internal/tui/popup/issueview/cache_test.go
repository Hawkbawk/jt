package issueview

import (
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hawkbawk/jt/internal/termimage"
)

// reset empties the cache, so tests sharing imageStore stay apart.
func (c *imageCache) reset() { c.flights.Clear() }

func TestCacheLoadsOnceForConcurrentAsks(t *testing.T) {
	c := newImageCache(2)
	var loads atomic.Int32
	release := make(chan struct{})
	img := &termimage.Image{}
	var loadErr error
	load := func() (*termimage.Image, error) { loads.Add(1); <-release; return img, loadErr }

	var wg sync.WaitGroup
	for range 5 {
		wg.Go(func() {
			if got, err := c.get("a", load); err != nil || got != img {
				t.Errorf("get = %v, %v", got, err)
			}
		})
	}
	close(release)
	wg.Wait()
	if _, err := c.get("a", load); err != nil || loads.Load() != 1 {
		t.Errorf("loaded %d times", loads.Load())
	}
}

func TestCacheRunsLimitedLoadsAtOnce(t *testing.T) {
	c := newImageCache(2)
	var mu sync.Mutex
	var running, peak int
	var loadErr error
	load := func() (*termimage.Image, error) {
		mu.Lock()
		running++
		peak = max(peak, running)
		mu.Unlock()
		time.Sleep(10 * time.Millisecond)
		mu.Lock()
		running--
		mu.Unlock()
		return &termimage.Image{}, loadErr
	}
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			if _, err := c.get(strconv.Itoa(i), load); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if peak != 2 {
		t.Errorf("peak parallel loads = %d, want 2", peak)
	}
}

func TestCacheDoesNotKeepFailures(t *testing.T) {
	c := newImageCache(2)
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
