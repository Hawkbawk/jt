package jira

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ankitpokhrel/jira-cli/pkg/adf"
)

const mediaDoc = `{"version":1,"type":"doc","content":[
 {"type":"paragraph","content":[{"type":"text","text":"before"}]},
 {"type":"mediaSingle","content":[{"type":"media","attrs":{"id":"u1","type":"file","alt":"shot.png"}}]},
 {"type":"mediaSingle","content":[{"type":"media","attrs":{"id":"u2","type":"file","alt":"clip.mp4"}}]},
 {"type":"paragraph","content":[{"type":"text","text":"after"}]}]}`

func parseDoc(t *testing.T, s string) *adf.ADF {
	t.Helper()
	var doc adf.ADF
	if err := json.Unmarshal([]byte(s), &doc); err != nil {
		t.Fatal(err)
	}
	return &doc
}

func TestMediaBecomesImageLinkOnlyForMatchedImages(t *testing.T) {
	doc := parseDoc(t, mediaDoc)
	got := toMarkdown(doc, &mediaIndex{byName: map[string]string{"shot.png": "10001"}})
	if !strings.Contains(got, "![shot.png]("+MediaLinkPrefix+"10001)") {
		t.Errorf("matched image not linked:\n%s", got)
	}
	if !strings.Contains(got, "[attachment]") {
		t.Errorf("unmatched media should stay as text:\n%s", got)
	}
	if plain := toMarkdown(doc, nil); strings.Contains(plain, MediaLinkPrefix) {
		t.Errorf("no images known, but got links:\n%s", plain)
	}
}

func TestMediaIDMatchesWhenAltTextWasEdited(t *testing.T) {
	doc := parseDoc(t, `{"version":1,"type":"doc","content":[{"type":"mediaSingle","content":[
	 {"type":"media","attrs":{"id":"u1","type":"file","alt":"A dashboard with a red error banner"}}]}]}`)
	idx := &mediaIndex{byUUID: map[string]string{"u1": "10002"}, byName: map[string]string{}}
	if got := toMarkdown(doc, idx); !strings.Contains(got, MediaLinkPrefix+"10002") {
		t.Errorf("media id match missing:\n%s", got)
	}
}

func TestNewMediaIndexPairsRenderedImages(t *testing.T) {
	html := `<p><span class="image-wrap"><img src="https://x.atlassian.net/rest/api/3/attachment/content/10001" alt="a.png" data-media-services-id="u1" data-attachment-type="file"></span>
	<img data-media-services-id="u2" src="https://x.atlassian.net/secure/attachment/10003/b.mp4">
	<img src="https://x.atlassian.net/secure/attachment/10009/c.png">
	<img alt="a > b" data-media-services-id='u4' src="https://x.atlassian.net/secure/attachment/10010/d.png?a=1&amp;b=2" /></p>`
	idx := newMediaIndex([]attachment{
		{ID: "10001", Filename: "a.png", MimeType: "image/png"},
		{ID: "10003", Filename: "b.mp4", MimeType: "video/mp4"},
		{ID: "10009", Filename: "c.png", MimeType: "image/png"},
		{ID: "10010", Filename: "d.png", MimeType: "image/png"},
	}, html)
	if len(idx.byUUID) != 2 || idx.byUUID["u1"] != "10001" || idx.byUUID["u4"] != "10010" {
		t.Errorf("pairs = %v, want u1 -> 10001 and u4 -> 10010", idx.byUUID)
	}
}

func TestNewMediaIndexSkipsOversizedImages(t *testing.T) {
	idx := newMediaIndex([]attachment{
		{ID: "1", Filename: "ok.png", MimeType: "image/png", Size: maxAttachmentBytes},
		{ID: "2", Filename: "big.png", MimeType: "image/png", Size: maxAttachmentBytes + 1},
	})
	if _, ok := idx.lookup("", "ok.png"); !ok {
		t.Error("image at the limit left out")
	}
	if _, ok := idx.lookup("", "big.png"); ok {
		t.Error("image over the limit indexed")
	}
}

func TestAnyMedia(t *testing.T) {
	if !anyMedia([]any{parseDoc(t, mediaDoc)}) {
		t.Error("document with media not detected")
	}
	text := parseDoc(t, `{"version":1,"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"hi"}]}]}`)
	if anyMedia([]any{text, "wiki", (*adf.ADF)(nil)}) {
		t.Error("document without media detected as having some")
	}
}
