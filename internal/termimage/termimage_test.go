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

func TestFitFillsTheSpaceKeepingAspect(t *testing.T) {
	im, err := Decode(pngBytes(t, 80, 160))
	if err != nil {
		t.Fatal(err)
	}
	p := im.Fit(0, 50, 50)
	lines := im.Lines(p)
	if len(lines) != 50 || ansi.StringWidth(lines[0]) != 50 {
		t.Fatalf("enlarged: got %d rows, width %d", len(lines), ansi.StringWidth(lines[0]))
	}
	seq := im.Sequence(p)
	for _, want := range []string{"U=1", "c=50", "r=50", "p=1", "a=p"} {
		if !strings.Contains(seq, want) {
			t.Errorf("placement %q lacks %q", seq, want)
		}
	}
	if !strings.Contains(lines[0], "\x1b[58;2;0;0;1m") {
		t.Errorf("placeholders don't name the placement: %q", lines[0])
	}
	if p := im.Fit(0, 4, 50); p.Cols != 4 || p.Rows != 4 {
		t.Errorf("narrow: %+v", p)
	}
	if p := im.Fit(0, 100, 10); p.Rows != 10 || p.Cols != 10 {
		t.Errorf("short: %+v", p)
	}
}

func TestPlacementsOfOneImageKeepSeparateIDs(t *testing.T) {
	im, err := Decode(pngBytes(t, 80, 160))
	if err != nil {
		t.Fatal(err)
	}
	a, b := im.Fit(0, 50, 50), im.Fit(1, 20, 20)
	if a == b || !strings.Contains(im.Sequence(b), "p=2") || !strings.Contains(im.Lines(b)[0], "\x1b[58;2;0;0;2m") {
		t.Errorf("placements share an ID: %+v %+v", a, b)
	}
}

func TestFitKeepsPlacementIDsInRange(t *testing.T) {
	im, err := Decode(pngBytes(t, 8, 8))
	if err != nil {
		t.Fatal(err)
	}
	for _, owner := range []uint64{0, 1, maxPlacementID - 1, maxPlacementID, maxPlacementID + 1, 1 << 40, ^uint64(0)} {
		if id := im.Fit(owner, 10, 10).ID; id < 1 || id > maxPlacementID {
			t.Errorf("owner %d got placement ID %d", owner, id)
		}
	}
}

func TestUploadIsTakenOnce(t *testing.T) {
	im, err := Decode(pngBytes(t, 8, 8))
	if err != nil {
		t.Fatal(err)
	}
	if up := im.TakeUpload(); up == "" {
		t.Fatal("first take is empty")
	}
	if up := im.TakeUpload(); up != "" {
		t.Errorf("second take = %q", up)
	}
}

func TestDeleteAllFreesImageData(t *testing.T) {
	if got := DeleteAll(); !strings.Contains(got, "d=A") || !strings.Contains(got, "a=d") {
		t.Errorf("DeleteAll = %q", got)
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
	up := im.TakeUpload()
	if !strings.Contains(up, "m=1") || strings.Count(up, "m=0") != 1 || !strings.HasPrefix(up, "\x1b_Gf=100,") {
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
	up := im.TakeUpload()
	if strings.Count(up, "a=f") != 2 || !strings.Contains(up, "s=3,v=1") {
		t.Errorf("gif upload = %q", up)
	}
}

func TestDecodeRejectsGarbage(t *testing.T) {
	if _, err := Decode([]byte("not an image")); err == nil {
		t.Error("expected an error")
	}
}
