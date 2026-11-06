// Package termimage draws images in the terminal with the kitty graphics
// protocol. Images are placed virtually and shown through Unicode
// placeholders, so they're ordinary text cells that scroll, clip and wrap
// with the text around them.
//
// Decode an image, write its TakeUpload sequence to the terminal, then put the
// lines for a Fit placement in the text, after writing its Sequence.
package termimage

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	_ "image/jpeg" // registers the jpeg decoder for image.Decode
	_ "image/png"  // registers the png decoder for image.Decode
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/kitty"
)

const (
	// cellW and cellH are the pixel size assumed for a terminal cell. Images
	// are stretched to their cell box, so a terminal with a different cell
	// shape shows them slightly squashed.
	cellW, cellH = 8, 16

	maxStillPixels    = 1600
	maxAnimatedPixels = 640
	maxFrames         = 100
	defaultFrameMS    = 100
	minFrameMS        = 20

	// maxPlacementID is the largest placement ID. The placeholder colors have
	// room for two bytes of it.
	maxPlacementID = 0xffff

	// quiet stops the terminal from answering the commands jt sends.
	quiet = 2
)

// Supported reports whether the terminal draws kitty graphics with Unicode
// placeholders. It looks at the environment, so it's a variable tests can
// replace. tmux is left out because it needs passthrough set up.
var Supported = sync.OnceValue(func() bool { return detect(os.Getenv) })

func detect(getenv func(string) string) bool {
	if getenv("TMUX") != "" {
		return false
	}
	return getenv("TERM") == "xterm-kitty" ||
		getenv("TERM") == "xterm-ghostty" ||
		getenv("TERM_PROGRAM") == "ghostty" ||
		getenv("KITTY_WINDOW_ID") != ""
}

var lastID atomic.Uint32

// Image is a decoded image, or the frames of a gif, ready to upload.
type Image struct {
	id     uint32
	width  int
	height int

	mu     sync.Mutex
	upload string
}

// Decode reads a png, jpeg or gif. A gif with several frames animates.
func Decode(data []byte) (*Image, error) {
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	var frames []image.Image
	delays := []int{0}
	if format == "gif" {
		frames, delays, err = decodeGIF(data)
	} else {
		var img image.Image
		img, _, err = image.Decode(bytes.NewReader(data))
		frames = []image.Image{img}
	}
	if err != nil {
		return nil, err
	}
	if len(frames) == 0 {
		return nil, errors.New("image has no frames")
	}

	limit := maxStillPixels
	if len(frames) > 1 {
		limit = maxAnimatedPixels
	}
	b := frames[0].Bounds()
	im := &Image{id: lastID.Add(1)}
	im.width, im.height = fit(b.Dx(), b.Dy(), limit)
	if im.width < 1 || im.height < 1 {
		return nil, errors.New("image is empty")
	}

	var up strings.Builder
	for i, f := range frames {
		opts := &kitty.Options{
			Format: kitty.PNG, Transmission: kitty.Direct, Chunk: true,
			ID: int(im.id), Quiet: quiet,
		}
		if i > 0 {
			opts.Action, opts.Z = kitty.Frame, frameMS(delays[i])
		}
		if err := kitty.EncodeGraphics(&up, scale(f, im.width, im.height), opts); err != nil {
			return nil, err
		}
	}
	if len(frames) > 1 {
		up.WriteString(command(&kitty.Options{Action: kitty.Animate, ID: int(im.id), Rows: 1, Z: frameMS(delays[0])}))
		// Run the animation, looping forever.
		up.WriteString(command(&kitty.Options{Action: kitty.Animate, ID: int(im.id), ImageWidth: 3, ImageHeight: 1}))
	}
	im.upload = up.String()
	return im, nil
}

// command is the sequence for options that carry no image data.
func command(o *kitty.Options) string {
	o.Quiet = quiet
	return ansi.KittyGraphics(nil, o.Options()...)
}

// TakeUpload returns the sequence that sends the image to the terminal the
// first time it's called, and nothing after. Write what it returns to the
// terminal before showing the image. The terminal keeps the image, so the
// sequence, which can be large, is dropped once taken.
func (im *Image) TakeUpload() string {
	im.mu.Lock()
	defer im.mu.Unlock()
	up := im.upload
	im.upload = ""
	return up
}

// Placement is an image shown at a size, under an ID of its own. Showing the
// same image at two sizes needs two IDs, since placing an ID again replaces the
// placement it had. Build one with Fit.
type Placement struct {
	ID         int
	Cols, Rows int
}

// Fit picks the largest size that keeps the image's shape within maxCols by
// maxRows cells, enlarging it if need be. owner is any number that's the same
// for everything that shows the image together, like a popup's ID. It's folded
// into the range placement IDs allow, so owners far enough apart can share an
// ID.
func (im *Image) Fit(owner uint64, maxCols, maxRows int) Placement {
	cols, rows := cells(im.width, im.height, max(1, maxCols), max(1, maxRows))
	return Placement{ID: int(owner%maxPlacementID) + 1, Cols: cols, Rows: rows}
}

// Sequence is what the terminal needs before it can draw the lines for p.
func (im *Image) Sequence(p Placement) string {
	return command(&kitty.Options{
		Action: kitty.Put, ID: int(im.id), PlacementID: p.ID,
		VirtualPlacement: true, Columns: p.Cols, Rows: p.Rows,
	})
}

// Lines is the rows of placeholder text that show the image placed as p.
func (im *Image) Lines(p Placement) []string {
	// The foreground color names the image and the underline color names the
	// placement, three bytes each.
	color := fmt.Sprintf("\x1b[38;2;%d;%d;%dm\x1b[58;2;0;%d;%dm",
		im.id>>16&0xff, im.id>>8&0xff, im.id&0xff, p.ID>>8&0xff, p.ID&0xff)
	lines := make([]string, p.Rows)
	for r := range lines {
		var b strings.Builder
		b.WriteString(color)
		// Only the first cell of a row says where it is. The terminal takes
		// a bare placeholder as the next column of the same row, which saves
		// the redraw work of measuring two combining marks on every cell.
		b.WriteRune(kitty.Placeholder)
		b.WriteRune(kitty.Diacritic(r))
		b.WriteRune(kitty.Diacritic(0))
		for range p.Cols - 1 {
			b.WriteRune(kitty.Placeholder)
		}
		b.WriteString("\x1b[39m\x1b[59m")
		lines[r] = b.String()
	}
	return lines
}

// DeleteAll is the sequence that frees every image the program sent.
func DeleteAll() string {
	return command(&kitty.Options{Action: kitty.Delete, Delete: kitty.DeleteAll, DeleteResources: true})
}

// cells is the size in cells of an image of w by h pixels, scaled up or down
// to the largest size that fits maxCols by maxRows.
func cells(w, h, maxCols, maxRows int) (cols, rows int) {
	scale := min(float64(maxCols*cellW)/float64(w), float64(maxRows*cellH)/float64(h))
	cols = max(1, min(maxCols, int(float64(w)*scale/cellW+0.5)))
	rows = max(1, min(maxRows, int(float64(h)*scale/cellH+0.5)))
	return cols, rows
}

// fit shrinks w by h so neither side passes limit.
func fit(w, h, limit int) (int, int) {
	if w <= limit && h <= limit {
		return w, h
	}
	s := float64(limit) / float64(max(w, h))
	return max(1, int(float64(w)*s)), max(1, int(float64(h)*s))
}

// scale shrinks img to w by h with nearest-neighbor sampling, or returns it
// as is when it already fits.
func scale(img image.Image, w, h int) image.Image {
	b := img.Bounds()
	if b.Dx() == w && b.Dy() == h {
		return img
	}
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			out.Set(x, y, img.At(b.Min.X+x*b.Dx()/w, b.Min.Y+y*b.Dy()/h))
		}
	}
	return out
}

func frameMS(delay int) int {
	if delay <= 1 {
		return defaultFrameMS
	}
	return max(delay*10, minFrameMS)
}

// decodeGIF returns each frame composited over the ones before it, with the
// delays in hundredths of a second.
func decodeGIF(data []byte) ([]image.Image, []int, error) {
	g, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		return nil, nil, err
	}
	canvas := image.NewRGBA(image.Rect(0, 0, g.Config.Width, g.Config.Height))
	var frames []image.Image
	var delays []int
	for i, f := range g.Image {
		if i >= maxFrames {
			break
		}
		var before *image.RGBA
		if i < len(g.Disposal) && g.Disposal[i] == gif.DisposalPrevious {
			before = cloneRGBA(canvas)
		}
		draw.Draw(canvas, f.Bounds(), f, f.Bounds().Min, draw.Over)
		frames = append(frames, cloneRGBA(canvas))
		delays = append(delays, g.Delay[i])

		switch {
		case before != nil:
			canvas = before
		case i < len(g.Disposal) && g.Disposal[i] == gif.DisposalBackground:
			draw.Draw(canvas, f.Bounds(), image.Transparent, image.Point{}, draw.Src)
		}
	}
	return frames, delays, nil
}

func cloneRGBA(src *image.RGBA) *image.RGBA {
	dst := image.NewRGBA(src.Bounds())
	copy(dst.Pix, src.Pix)
	return dst
}
