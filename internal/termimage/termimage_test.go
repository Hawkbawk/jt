package termimage

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDetect(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	for name, tc := range map[string]struct {
		env  map[string]string
		want bool
	}{
		"ghostty":      {map[string]string{"TERM": "xterm-ghostty"}, true},
		"ghostty prog": {map[string]string{"TERM_PROGRAM": "ghostty"}, true},
		"kitty":        {map[string]string{"KITTY_WINDOW_ID": "3"}, true},
		"tmux":         {map[string]string{"TERM": "xterm-ghostty", "TMUX": "/tmp/x"}, false},
		"xterm":        {map[string]string{"TERM": "xterm-256color"}, false},
	} {
		if got := detect(env(tc.env)); got != tc.want {
			t.Errorf("%s: detect = %v, want %v", name, got, tc.want)
		}
	}
}

func TestPlaceFitsAndNeverEnlarges(t *testing.T) {
	im, err := Decode(pngBytes(t, 80, 160))
	if err != nil {
		t.Fatal(err)
	}
	lines, seq := im.Place(50, 50)
	if len(lines) != 10 || ansi.StringWidth(lines[0]) != 10 {
		t.Fatalf("native size: got %d rows, width %d", len(lines), ansi.StringWidth(lines[0]))
	}
	if !strings.Contains(seq, "U=1") || !strings.Contains(seq, "c=10") || !strings.Contains(seq, "r=10") {
		t.Errorf("placement = %q", seq)
	}
	if _, seq := im.Place(50, 50); seq != "" {
		t.Errorf("unchanged size placed again: %q", seq)
	}
	lines, seq = im.Place(4, 50)
	if len(lines) != 4 || ansi.StringWidth(lines[0]) != 4 || seq == "" {
		t.Errorf("narrow: %d rows, width %d, seq %q", len(lines), ansi.StringWidth(lines[0]), seq)
	}
}

func TestUploadChunksLargeImages(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 300, 300))
	for i := range img.Pix {
		img.Pix[i] = byte(i * 7)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	im, err := Decode(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	up := im.Upload()
	if !strings.Contains(up, "m=1") || strings.Count(up, "m=0") != 1 || !strings.HasPrefix(up, "\x1b_Ga=t,f=100,") {
		t.Errorf("upload not chunked: %.80q", up)
	}
}

func TestGIFAnimates(t *testing.T) {
	pal := color.Palette{color.Black, color.White}
	g := &gif.GIF{Config: image.Config{Width: 4, Height: 4}}
	for range 3 {
		g.Image = append(g.Image, image.NewPaletted(image.Rect(0, 0, 4, 4), pal))
		g.Delay = append(g.Delay, 5)
		g.Disposal = append(g.Disposal, gif.DisposalNone)
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		t.Fatal(err)
	}
	im, err := Decode(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	up := im.Upload()
	if strings.Count(up, "a=f,") != 2 || !strings.Contains(up, "s=3,v=1") {
		t.Errorf("gif upload = %q", up)
	}
}

func TestDecodeRejectsGarbage(t *testing.T) {
	if _, err := Decode([]byte("not an image")); err == nil {
		t.Error("expected an error")
	}
}
