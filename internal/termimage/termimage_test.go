package termimage

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"strings"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/ansi/kitty"
)

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDetectTmux(t *testing.T) {
	for name, tc := range map[string]struct {
		out  string
		want mode
	}{
		"kitty":        {"on xterm-kitty", viaTmux},
		"ghostty all":  {"all xterm-ghostty", viaTmux},
		"no passthru":  {"off xterm-kitty", unsupported},
		"plain client": {"on xterm-256color", unsupported},
	} {
		got := detectTmux(func(string) (string, error) { return tc.out, nil })
		if got != tc.want {
			t.Errorf("%s: detectTmux = %v, want %v", name, got, tc.want)
		}
	}
	if got := detectTmux(func(string) (string, error) { return "", errors.New("no tmux") }); got != unsupported {
		t.Errorf("tmux failing: detectTmux = %v, want unsupported", got)
	}
}

func TestObserveEnablesGraphicsAfterTheReplies(t *testing.T) {
	reset := func() { current.Store(0); graphicsSeen.Store(false) }
	reset()
	t.Cleanup(reset)

	ok := uv.KittyGraphicsEvent{Options: kitty.Options{ID: probeID}, Payload: []byte("OK")}
	da1 := uv.PrimaryDeviceAttributesEvent{62, 4}

	Observe(da1)
	if Supported() {
		t.Fatal("supported with only the device attributes reply")
	}
	Observe(uv.KittyGraphicsEvent{Options: kitty.Options{ID: probeID}, Payload: []byte("ENOTSUPPORTED")})
	Observe(da1)
	if Supported() {
		t.Fatal("supported after a graphics error")
	}
	Observe(ok)
	if Supported() {
		t.Fatal("supported before the device attributes reply")
	}
	Observe(da1)
	if !Supported() {
		t.Fatal("not supported after both replies")
	}
}

func TestWrapSendsEachCommandThroughTmux(t *testing.T) {
	t.Cleanup(func() { current.Store(0) })
	two := "\x1b_Ga=q;AAAA\x1b\\\x1b_Ga=d\x1b\\"

	current.Store(int32(direct))
	if got := wrap(two); got != two {
		t.Errorf("direct: wrap = %q, want it unchanged", got)
	}
	current.Store(int32(viaTmux))
	want := ansi.TmuxPassthrough("\x1b_Ga=q;AAAA\x1b\\") + ansi.TmuxPassthrough("\x1b_Ga=d\x1b\\")
	if got := wrap(two); got != want {
		t.Errorf("tmux: wrap = %q, want %q", got, want)
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
