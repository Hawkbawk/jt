// Package termimage draws images in the terminal with the kitty graphics
// protocol. Images are placed virtually and shown through Unicode
// placeholders, so they're ordinary text cells that scroll, clip and wrap
// with the text around them.
//
// Decode an image, write its Upload sequence to the terminal once, then put
// the lines Place returns in the text.
package termimage

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	_ "image/jpeg" // registers the jpeg decoder for image.Decode
	"image/png"
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
	chunkSize         = 4096
	defaultFrameMS    = 100
	minFrameMS        = 20

	// placementID is the one placement each image has. Placing it again at a
	// new size replaces it instead of adding another.
	placementID = 1
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
	id         uint32
	width      int
	height     int
	upload     string
	cols, rows int
}

// Decode reads a png, jpeg or gif. A gif with several frames animates.
func Decode(data []byte) (*Image, error) {
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	var frames []image.Image
	var delays []int
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
		f = scale(f, im.width, im.height)
		var buf bytes.Buffer
		enc := png.Encoder{CompressionLevel: png.BestSpeed}
		if err := enc.Encode(&buf, f); err != nil {
			return nil, err
		}
		opts := []string{"a=t", "f=100"}
		if i > 0 {
			opts = []string{"a=f", "f=100", fmt.Sprintf("z=%d", frameMS(delays[i]))}
		}
		opts = append(opts, fmt.Sprintf("i=%d", im.id), "q=2")
		up.WriteString(chunks(buf.Bytes(), opts))
	}
	if len(frames) > 1 {
		up.WriteString(ansi.KittyGraphics(nil, "a=a", fmt.Sprintf("i=%d", im.id), "r=1",
			fmt.Sprintf("z=%d", frameMS(delays[0])), "q=2"))
		up.WriteString(ansi.KittyGraphics(nil, "a=a", fmt.Sprintf("i=%d", im.id), "s=3", "v=1", "q=2"))
	}
	im.upload = up.String()
	return im, nil
}

// Upload is the sequence that sends the image to the terminal. Write it once.
func (im *Image) Upload() string { return im.upload }

// Place sizes the image to the largest size that fits maxCols by maxRows
// cells, enlarging it if need be, and returns the rows of placeholder text that show it. The sequence is what
// the terminal needs before it can draw them, and is empty when the image is
// already placed at that size.
func (im *Image) Place(maxCols, maxRows int) (lines []string, seq string) {
	cols, rows := cells(im.width, im.height, max(1, maxCols), max(1, maxRows))
	if cols != im.cols || rows != im.rows {
		im.cols, im.rows = cols, rows
		seq = ansi.KittyGraphics(nil, "a=p", fmt.Sprintf("i=%d", im.id), fmt.Sprintf("p=%d", placementID),
			"U=1", fmt.Sprintf("c=%d", cols), fmt.Sprintf("r=%d", rows), "q=2")
	}
	// The foreground color names the image and the underline color names the
	// placement, three bytes each.
	color := fmt.Sprintf("\x1b[38;2;%d;%d;%dm\x1b[58;2;0;%d;%dm",
		im.id>>16&0xff, im.id>>8&0xff, im.id&0xff, placementID>>8&0xff, placementID&0xff)
	lines = make([]string, rows)
	for r := range lines {
		var b strings.Builder
		b.WriteString(color)
		// Only the first cell of a row says where it is. The terminal takes
		// a bare placeholder as the next column of the same row, which saves
		// the redraw work of measuring two combining marks on every cell.
		b.WriteRune(kitty.Placeholder)
		b.WriteRune(kitty.Diacritic(r))
		b.WriteRune(kitty.Diacritic(0))
		for range cols - 1 {
			b.WriteRune(kitty.Placeholder)
		}
		b.WriteString("\x1b[39m\x1b[59m")
		lines[r] = b.String()
	}
	return lines, seq
}

// DeleteAll is the sequence that frees every image the program sent.
func DeleteAll() string {
	return ansi.KittyGraphics(nil, "a=d", "d=A", "q=2")
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

// chunks splits a payload into the sequences the protocol allows, putting
// opts on the first.
func chunks(data []byte, opts []string) string {
	enc := base64.StdEncoding.EncodeToString(data)
	var b strings.Builder
	for first := true; len(enc) > 0 || first; first = false {
		n := min(len(enc), chunkSize)
		part := enc[:n]
		enc = enc[n:]
		more := "m=0"
		if len(enc) > 0 {
			more = "m=1"
		}
		o := []string{more}
		if first {
			o = append(opts, more)
		}
		b.WriteString(ansi.KittyGraphics([]byte(part), o...))
	}
	return b.String()
}
