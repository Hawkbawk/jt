package issueview

import (
	"sync"

	"github.com/hawkbawk/jt/internal/termimage"
)

// imageCache keeps decoded images by attachment ID, since an attachment's
// content doesn't change. However many popups ask for an image at once, it's
// loaded once. An image holds its upload sequence only until it's sent, so
// entries stay small and nothing is evicted.
type imageCache struct {
	flights sync.Map // attachment ID to *flight
}

// flight is one load of an image, kept after it finishes as the cached result.
type flight struct {
	done chan struct{}
	img  *termimage.Image
	err  error
}

// imageStore is the cache every popup shares.
var imageStore = &imageCache{}

// get returns the image for key, running load if no one has yet. Failures
// aren't kept, so the next ask tries again.
func (c *imageCache) get(key string, load func() (*termimage.Image, error)) (*termimage.Image, error) {
	f := &flight{done: make(chan struct{})}
	if v, loaded := c.flights.LoadOrStore(key, f); loaded {
		f = v.(*flight)
		<-f.done
		return f.img, f.err
	}
	f.img, f.err = load()
	if f.err != nil {
		c.flights.Delete(key)
	}
	close(f.done)
	return f.img, f.err
}
