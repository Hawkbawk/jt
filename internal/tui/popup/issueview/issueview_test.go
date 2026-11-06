package issueview

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"slices"
	"strings"
	"testing"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/termimage"
)

type fakeBackend struct {
	detail   *jira.IssueDetail
	comments *jira.CommentList
	err      error
	files    map[string][]byte
}

func (f fakeBackend) Attachment(id string) ([]byte, error) {
	if data, ok := f.files[id]; ok {
		return data, nil
	}
	return nil, errors.New("no such attachment")
}

func (f fakeBackend) Issue(string) (*jira.IssueDetail, error) { return f.detail, f.err }

func (f fakeBackend) Comments(string) (*jira.CommentList, error) { return f.comments, f.err }

func plain(md string, _ int) string { return md }

func detail() *jira.IssueDetail {
	return &jira.IssueDetail{
		Key: "T-1", Summary: "one", Status: "Open",
		Description: strings.Repeat("line\n", 100),
	}
}

func comments() *jira.CommentList {
	return &jira.CommentList{Total: 2, Items: []jira.Comment{
		{Author: "Ada", Created: "2026-10-01 09:00", Body: "first comment"},
		{Author: "Grace", Created: "2026-10-02 10:00", Body: "second comment"},
	}}
}

func sized(m *Model, w, h int) *Model {
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m
}

// loaded runs the model's load and hands the response back to it.
func loaded(m *Model) *Model {
	m.Update(m.Reload()())
	return m
}

func press(m *Model, k string) {
	msg := tea.KeyPressMsg{Code: rune(k[0]), Text: k}
	m.Update(msg)
}

func view(m *Model) string { return ansi.Strip(m.View().Content) }

func TestStartsLoading(t *testing.T) {
	m := sized(Details(fakeBackend{detail: detail()}, "T-1", plain), 40, 10)
	if !m.Loading() || m.Ready() || !strings.Contains(view(m), "Loading") {
		t.Fatal("a new model should be loading")
	}
}

func TestLoadShowsDetailsAndRerendersOnResize(t *testing.T) {
	var widths []int
	md := func(s string, w int) string { widths = append(widths, w); return s }
	m := sized(Details(fakeBackend{detail: detail()}, "T-1", md), 40, 10)
	loaded(m)
	sized(m, 30, 10)
	if m.Loading() || !m.Ready() || len(widths) != 2 || widths[0] != 40 || widths[1] != 30 {
		t.Fatalf("text should render at each width, got %v", widths)
	}
	if v := view(m); !strings.Contains(v, "Details · T-1") || !strings.Contains(v, "one") {
		t.Fatalf("missing title or summary:\n%s", v)
	}
}

func TestCommentsShowEveryComment(t *testing.T) {
	m := sized(Comments(fakeBackend{comments: comments()}, "T-1", plain), 60, 20)
	loaded(m)
	v := view(m)
	for _, want := range []string{"Comments · T-1", "Ada", "first comment", "Grace", "second comment"} {
		if !strings.Contains(v, want) {
			t.Errorf("comments missing %q:\n%s", want, v)
		}
	}
}

func TestNoComments(t *testing.T) {
	if got := renderComments(&jira.CommentList{}, 40, plain); !strings.Contains(got, "No comments") {
		t.Fatalf("got %q", got)
	}
}

func TestLoadErrorIsShown(t *testing.T) {
	m := sized(Details(fakeBackend{err: errors.New("boom")}, "T-1", plain), 40, 10)
	loaded(m)
	if v := view(m); m.Ready() || !strings.Contains(v, "Error: boom") {
		t.Fatalf("unexpected error view:\n%s", v)
	}
}

func TestLoadForAnotherModelIsIgnored(t *testing.T) {
	m := sized(Details(fakeBackend{detail: detail()}, "T-1", plain), 40, 10)
	m.Update(loadedMsg{id: m.id - 1, issue: &jira.IssueDetail{}})
	m.Update(loadedMsg{id: m.id - 1, err: errors.New("boom")})
	if !m.Loading() || m.err != nil {
		t.Fatal("model accepted an issue it did not request")
	}
}

func TestScrollKeys(t *testing.T) {
	m := sized(Details(fakeBackend{detail: detail()}, "T-1", plain), 40, 10)
	loaded(m)
	press(m, "G")
	if m.ScrollPercent() != 1 {
		t.Fatalf("G should scroll to the end, at %v", m.ScrollPercent())
	}
	press(m, "g")
	if m.ScrollPercent() != 0 {
		t.Fatalf("g should scroll to the top, at %v", m.ScrollPercent())
	}
}

func TestViewFillsTheAreaItIsSizedFor(t *testing.T) {
	m := sized(Details(fakeBackend{detail: detail()}, "T-1", plain), 40, 20)
	loaded(m)
	if h := strings.Count(m.View().Content, "\n") + 1; h != 20 {
		t.Fatalf("view should fill the area, got %d lines", h)
	}
}

func TestSpinnerStopsWhenIdleAndRestarts(t *testing.T) {
	m := loaded(Details(fakeBackend{detail: detail()}, "T-1", plain))
	if cmd := m.SetBusy(true); cmd == nil {
		t.Fatal("becoming busy should restart the spinner")
	}
	if cmd := m.SetBusy(true); cmd != nil {
		t.Fatal("already busy, so nothing to restart")
	}
	m.SetBusy(false)
	if _, cmd := m.Update(spinner.TickMsg{}); cmd != nil {
		t.Fatal("the tick chain should end when nothing is busy")
	}
}

func TestEditedCommentIsMarked(t *testing.T) {
	l := &jira.CommentList{Items: []jira.Comment{
		{Author: "Ada", Created: "2026-10-01 09:00", Edited: true, Body: "a"},
		{Author: "Grace", Created: "2026-10-02 10:00", Body: "b"},
	}}
	if got := strings.Count(renderComments(l, 40, plain), "edited"); got != 1 {
		t.Fatalf("got %d edited markers, want 1", got)
	}
}

func supported(t *testing.T, on bool) {
	t.Helper()
	old := termimage.Supported
	termimage.Supported = func() bool { return on }
	t.Cleanup(func() { termimage.Supported = old })
}

func withImage(files map[string][]byte) fakeBackend {
	d := detail()
	d.Description = "before\n\n![shot.png](" + jira.MediaLinkPrefix + "img-1)\n\nafter"
	return fakeBackend{detail: d, files: files}
}

func pngFile(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 80, 32))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestSplitOnlyCutsAtImageParagraphs(t *testing.T) {
	link := "![a.png](" + jira.MediaLinkPrefix + "7)"
	got := split("before\n\n" + link + "\n\nafter")
	if len(got) != 3 || got[1].media != "7" ||
		strings.TrimSpace(got[0].text) != "before" || strings.TrimSpace(got[2].text) != "after" {
		t.Errorf("split = %+v", got)
	}
	for name, src := range map[string]string{
		"code block": "```\n" + link + "\n```",
		"in a line":  "see " + link + " here",
		"other link": "![x](https://example.com/x.png)",
	} {
		if got := split(src); len(got) != 1 || got[0].text != src {
			t.Errorf("%s: split = %+v", name, got)
		}
	}
}

func TestExtractMarksStripsThemAndFindsTheirLines(t *testing.T) {
	src := "one\n" + markOpen + "a" + markClose + "Loading\nthree\n" + markOpen + "b" + markClose + "Loading"
	got, rows := extractMarks(src)
	if got != "one\nLoading\nthree\nLoading" {
		t.Errorf("text = %q", got)
	}
	if want := []imageRow{{"a", 1}, {"b", 3}}; !slices.Equal(rows, want) {
		t.Errorf("rows = %v, want %v", rows, want)
	}
	if got, rows := extractMarks("plain"); got != "plain" || rows != nil {
		t.Errorf("plain text changed: %q %v", got, rows)
	}
}

func TestImageLinkFallsBackToTextWithoutSupport(t *testing.T) {
	supported(t, false)
	m := loaded(sized(Details(withImage(nil), "T-1", plain), 40, 20))
	if got := view(m); !strings.Contains(got, "[attachment]") || !strings.Contains(got, "before") || !strings.Contains(got, "after") {
		t.Errorf("view = %q", got)
	}
	if len(m.images) != 0 {
		t.Errorf("loaded images without support: %v", m.images)
	}
}

func TestImageReplacesLinkOnceLoaded(t *testing.T) {
	supported(t, true)
	t.Cleanup(func() { imageStore.reset() })
	m := loaded(sized(Details(withImage(map[string][]byte{"img-1": pngFile(t)}), "T-1", plain), 40, 20))
	if got := view(m); !strings.Contains(got, "Loading image") {
		t.Fatalf("expected a loading hint, got %q", got)
	}

	_, cmd := m.Update(m.loadImage("img-1")())
	if cmd == nil {
		t.Fatal("expected a command that sends the image to the terminal")
	}
	got := view(m)
	if !strings.ContainsRune(got, '\U0010EEEE') || strings.Contains(got, "[attachment]") || strings.Contains(got, "Loading image") {
		t.Errorf("view = %q", got)
	}
	if len(m.pending) != 0 {
		t.Errorf("pending not flushed: %d", len(m.pending))
	}
}

func TestImageThatFailsToLoadStaysText(t *testing.T) {
	supported(t, true)
	m := loaded(sized(Details(withImage(nil), "T-1", plain), 40, 20))
	m.Update(m.loadImage("img-1")())
	if got := view(m); !strings.Contains(got, "[attachment]") {
		t.Errorf("view = %q", got)
	}
}

func farImage(files map[string][]byte) fakeBackend {
	b := withImage(files)
	b.detail.Description = strings.Repeat("line\n\n", 100) + "![shot.png](" + jira.MediaLinkPrefix + "img-1)\n\nafter"
	return b
}

func TestImagesLoadWhenScrolledNear(t *testing.T) {
	supported(t, true)
	t.Cleanup(func() { imageStore.reset() })
	m := loaded(sized(Details(farImage(map[string][]byte{"img-1": pngFile(t)}), "T-1", plain), 40, 20))
	if len(m.images) != 0 {
		t.Fatalf("asked for an image far off screen: %v", m.images)
	}
	press(m, "G")
	if len(m.images) != 1 {
		t.Errorf("image near the visible text wasn't asked for: %v", m.images)
	}
}

func TestPopupsPlaceTheSameImageUnderTheirOwnIDs(t *testing.T) {
	supported(t, true)
	t.Cleanup(func() { imageStore.reset() })
	files := map[string][]byte{"img-1": pngFile(t)}
	a := loaded(sized(Details(withImage(files), "T-1", plain), 40, 20))
	b := loaded(sized(Details(withImage(files), "T-1", plain), 80, 40))
	a.Update(a.loadImage("img-1")())
	b.Update(b.loadImage("img-1")())
	pa, pb := a.placed["img-1"], b.placed["img-1"]
	if pa.ID == 0 || pa.ID == pb.ID || pa == pb {
		t.Errorf("placements = %+v and %+v", pa, pb)
	}
	if h := strings.Count(a.View().Content, "\n") + 1; h != 20 {
		t.Errorf("view with an image is %d lines, want 20", h)
	}
}

func TestImageLoadedForAClosedPopupIsSentByTheNext(t *testing.T) {
	supported(t, true)
	t.Cleanup(func() { imageStore.reset() })
	files := map[string][]byte{"img-1": pngFile(t)}
	closed := loaded(sized(Details(withImage(files), "T-1", plain), 40, 20))
	closed.loadImage("img-1")()

	next := loaded(sized(Details(withImage(files), "T-1", plain), 40, 20))
	next.pending = nil
	next.images["img-1"] = &imageState{}
	next.imageLoaded(next.loadImage("img-1")().(imageMsg))
	if len(next.pending) != 2 || !strings.HasPrefix(next.pending[0], "\x1b_G") {
		t.Errorf("image wasn't uploaded by the popup that showed it: %q", next.pending)
	}
}
