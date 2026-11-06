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

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/kitty"

	"github.com/hawkbawk/jt/internal/system"
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

// mode is how the program reaches the terminal's graphics.
type mode int32

const (
	unsupported mode = iota
	direct
	// viaTmux sends every graphics command through tmux's passthrough.
	viaTmux
)

var (
	// current is unsupported until Setup or Observe learn better.
	current atomic.Int32

	// graphicsSeen is set when the terminal has said it draws graphics. It
	// counts once the device attributes reply shows nothing else is coming.
	graphicsSeen atomic.Bool
)

func currentMode() mode { return mode(current.Load()) }

// Supported reports whether the terminal draws kitty graphics with Unicode
// placeholders. It's false until Setup and Observe have found out. It's a
// variable so tests can replace it.
var Supported = func() bool { return currentMode() != unsupported }

// Setup starts finding out whether the terminal supports graphics. Write what
// it returns to the terminal and pass the terminal's replies to Observe. It
// returns nothing when the answer is already known.
//
// tmux swallows the terminal's replies, so inside it Setup asks tmux for the
// outer terminal's name instead. The commands only reach that terminal if
// allow-passthrough is on.
func Setup() string {
	if os.Getenv("TMUX") == "" {
		return probeQuery
	}
	current.Store(int32(detectTmux(system.TmuxDisplay)))
	return ""
}

func detectTmux(tmux func(format string) (string, error)) mode {
	out, err := tmux("#{allow-passthrough} #{client_termname}")
	if err != nil {
		return unsupported
	}
	pass, term, _ := strings.Cut(out, " ")
	if (pass == "on" || pass == "all") && (term == "xterm-kitty" || term == "xterm-ghostty") {
		return viaTmux
	}
	return unsupported
}

// Observe takes a message from the terminal's input. It enables Supported when
// the reply to Setup's query shows graphics, and ignores any other message.
func Observe(msg any) {
	switch msg := msg.(type) {
	case uv.KittyGraphicsEvent:
		if msg.Options.ID == probeID && string(msg.Payload) == "OK" {
			graphicsSeen.Store(true)
		}
	case uv.PrimaryDeviceAttributesEvent:
		if graphicsSeen.Load() {
			current.CompareAndSwap(int32(unsupported), int32(direct))
		}
	}
}

const st = "\x1b\\"

// wrap makes seq reach the outer terminal when the program runs in tmux. Each
// command goes in a passthrough of its own, so an upload isn't one huge
// sequence for tmux to buffer.
func wrap(seq string) string {
	if currentMode() != viaTmux {
		return seq
	}
	var b strings.Builder
	for seq != "" {
		end := len(seq)
		if i := strings.Index(seq, st); i >= 0 {
			end = i + len(st)
		}
		b.WriteString(ansi.TmuxPassthrough(seq[:end]))
		seq = seq[end:]
	}
	return b.String()
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
	var (
		width, height, count int
		walk                 func(yield func(image.Image, int) error) error
	)
	if format == "gif" {
		g, err := gif.DecodeAll(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		width, height, count = g.Config.Width, g.Config.Height, min(len(g.Image), maxFrames)
		walk = func(yield func(image.Image, int) error) error { return eachGIFFrame(g, yield) }
	} else {
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		b := img.Bounds()
		width, height, count = b.Dx(), b.Dy(), 1
		walk = func(yield func(image.Image, int) error) error { return yield(img, 0) }
	}
	if count == 0 {
		return nil, errors.New("image has no frames")
	}

	limit := maxStillPixels
	if count > 1 {
		limit = maxAnimatedPixels
	}
	im := &Image{id: lastID.Add(1)}
	im.width, im.height = fit(width, height, limit)
	if im.width < 1 || im.height < 1 {
		return nil, errors.New("image is empty")
	}

	var up strings.Builder
	firstDelay := 0
	i := 0
	err = walk(func(f image.Image, delay int) error {
		opts := &kitty.Options{
			Format: kitty.PNG, Transmission: kitty.Direct, Chunk: true,
			ID: int(im.id), Quiet: quiet,
		}
		if i == 0 {
			firstDelay = delay
		} else {
			opts.Action, opts.Z = kitty.Frame, frameMS(delay)
		}
		i++
		return kitty.EncodeGraphics(&up, scale(f, im.width, im.height), opts)
	})
	if err != nil {
		return nil, err
	}
	if count > 1 {
		up.WriteString(command(&kitty.Options{Action: kitty.Animate, ID: int(im.id), Rows: 1, Z: frameMS(firstDelay)}))
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
	return wrap(up)
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
	return wrap(command(&kitty.Options{
		Action: kitty.Put, ID: int(im.id), PlacementID: p.ID,
		VirtualPlacement: true, Columns: p.Cols, Rows: p.Rows,
	}))
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
	return wrap(command(&kitty.Options{Action: kitty.Delete, Delete: kitty.DeleteAll, DeleteResources: true}))
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

// eachGIFFrame calls yield with each frame composited over the ones before it,
// and its delay in hundredths of a second. The image is the canvas itself, so
// yield must be done with it before it returns.
func eachGIFFrame(g *gif.GIF, yield func(image.Image, int) error) error {
	canvas := image.NewRGBA(image.Rect(0, 0, g.Config.Width, g.Config.Height))
	for i, f := range g.Image {
		if i >= maxFrames {
			break
		}
		var before *image.RGBA
		if i < len(g.Disposal) && g.Disposal[i] == gif.DisposalPrevious {
			before = cloneRGBA(canvas)
		}
		draw.Draw(canvas, f.Bounds(), f, f.Bounds().Min, draw.Over)
		if err := yield(canvas, g.Delay[i]); err != nil {
			return err
		}

		switch {
		case before != nil:
			canvas = before
		case i < len(g.Disposal) && g.Disposal[i] == gif.DisposalBackground:
			draw.Draw(canvas, f.Bounds(), image.Transparent, image.Point{}, draw.Src)
		}
	}
	return nil
}

func cloneRGBA(src *image.RGBA) *image.RGBA {
	dst := image.NewRGBA(src.Bounds())
	copy(dst.Pix, src.Pix)
	return dst
}
