package issueview

import (
	"context"
	"sync"

	"golang.org/x/sync/semaphore"

	"github.com/hawkbawk/jt/internal/termimage"
)

// imageCache keeps decoded images by attachment ID, since an attachment's
// content doesn't change. However many modals ask for an image at once, it's
// loaded once, and only a few loads run at a time. An image holds its upload
// sequence only until it's sent to the terminal, so entries stay small and
// nothing is evicted.
type imageCache struct {
	flights sync.Map // attachment ID to *flight
	slots   *semaphore.Weighted
}

const parallelLoads = 4

// flight is one load of an image, kept after it finishes as the cached result.
type flight struct {
	done chan struct{}
	img  *termimage.Image
	err  error
}

// imageStore is the cache every modal shares.
var imageStore = newImageCache(parallelLoads)

func newImageCache(parallel int64) *imageCache {
	return &imageCache{slots: semaphore.NewWeighted(parallel)}
}

// get returns the image for key, running load if no one has yet. Failures
// aren't kept, so the next ask tries again.
func (c *imageCache) get(key string, load func() (*termimage.Image, error)) (*termimage.Image, error) {
	f := &flight{done: make(chan struct{})}
	if v, loaded := c.flights.LoadOrStore(key, f); loaded {
		f = v.(*flight)
		<-f.done
		return f.img, f.err
	}
	if err := c.slots.Acquire(context.Background(), 1); err != nil {
		c.flights.Delete(key)
		close(f.done)
		return nil, err
	}
	f.img, f.err = load()
	c.slots.Release(1)
	if f.err != nil {
		c.flights.Delete(key)
	}
	close(f.done)
	return f.img, f.err
}
