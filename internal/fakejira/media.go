package fakejira

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/png"

	"github.com/hawkbawk/jt/internal/jira"
)

// Attachment IDs the demo descriptions link to.
const (
	pngAttachment = "fake-png"
	gifAttachment = "fake-gif"
)

// imageLink is how a description links to a generated image.
func imageLink(name, id string) string {
	return "![" + name + "](" + jira.MediaLinkPrefix + id + ")"
}

// Attachment returns a generated animated gif for the gif attachment and a
// still png for every other ID.
func (b *Backend) Attachment(id string) ([]byte, error) {
	var buf bytes.Buffer
	var err error
	if id == gifAttachment {
		err = gif.EncodeAll(&buf, bouncingSquare())
	} else {
		err = png.Encode(&buf, gradient(320, 180))
	}
	return buf.Bytes(), err
}

func gradient(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: uint8(x * 255 / w), G: uint8(y * 255 / h), B: 160, A: 255})
		}
	}
	return img
}

func bouncingSquare() *gif.GIF {
	const size, frames, box = 120, 8, 24
	pal := color.Palette{color.RGBA{A: 255}, color.RGBA{R: 240, G: 160, B: 40, A: 255}}
	g := &gif.GIF{LoopCount: 0}
	for i := range frames {
		img := image.NewPaletted(image.Rect(0, 0, size, size), pal)
		x := i * (size - box) / (frames - 1)
		for dy := range box {
			for dx := range box {
				img.SetColorIndex(x+dx, size/2-box/2+dy, 1)
			}
		}
		g.Image = append(g.Image, img)
		g.Delay = append(g.Delay, 8)
	}
	return g
}
