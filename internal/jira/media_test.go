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
	got := toMarkdownWithImages(doc, &mediaIndex{byName: map[string]string{"shot.png": "10001"}})
	if !strings.Contains(got, "![shot.png]("+MediaLinkPrefix+"10001)") {
		t.Errorf("matched image not linked:\n%s", got)
	}
	if !strings.Contains(got, "[attachment]") {
		t.Errorf("unmatched media should stay as text:\n%s", got)
	}
	if plain := toMarkdown(doc); strings.Contains(plain, MediaLinkPrefix) {
		t.Errorf("no images known, but got links:\n%s", plain)
	}
}

func TestMediaIDMatchesWhenAltTextWasEdited(t *testing.T) {
	doc := parseDoc(t, `{"version":1,"type":"doc","content":[{"type":"mediaSingle","content":[
	 {"type":"media","attrs":{"id":"u1","type":"file","alt":"A dashboard with a red error banner"}}]}]}`)
	idx := &mediaIndex{byUUID: map[string]string{"u1": "10002"}, byName: map[string]string{}}
	if got := toMarkdownWithImages(doc, idx); !strings.Contains(got, MediaLinkPrefix+"10002") {
		t.Errorf("media id match missing:\n%s", got)
	}
}

func TestPairRendered(t *testing.T) {
	html := `<p><span class="image-wrap"><img src="https://x.atlassian.net/rest/api/3/attachment/content/10001" alt="a.png" data-media-services-id="u1" data-attachment-type="file"></span>
	<img data-media-services-id="u2" src="https://x.atlassian.net/secure/attachment/10003/b.mp4">
	<img src="https://x.atlassian.net/secure/attachment/10009/c.png">
	<img alt="a > b" data-media-services-id='u4' src="https://x.atlassian.net/secure/attachment/10010/d.png?a=1&amp;b=2" /></p>`
	got := pairRendered(html, map[string]bool{"10001": true, "10009": true, "10010": true})
	if len(got) != 2 || got["u1"] != "10001" || got["u4"] != "10010" {
		t.Errorf("pairs = %v, want u1 -> 10001 and u4 -> 10010", got)
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
