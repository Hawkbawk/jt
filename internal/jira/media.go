package jira

import (
	"context"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/ankitpokhrel/jira-cli/pkg/adf"
	jcli "github.com/ankitpokhrel/jira-cli/pkg/jira"
	"golang.org/x/net/html"
)

// MediaLinkPrefix starts the destination of an image link in the markdown for
// descriptions and comments. What follows it is the attachment ID to pass to
// Attachment. Media jt can't resolve stays as the text [attachment].
const MediaLinkPrefix = "jt-media:"

const maxAttachmentBytes = 20 << 20

var imageTypes = map[string]bool{"image/png": true, "image/jpeg": true, "image/gif": true}

// Attachment downloads an attachment's content.
func (c *Client) Attachment(id string) ([]byte, error) {
	res, err := c.jc.Get(context.Background(), "/attachment/content/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, jcli.ErrEmptyResponse
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return nil, responseError(res, jcli.Errors{})
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxAttachmentBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxAttachmentBytes {
		return nil, fmt.Errorf("attachment is larger than %d MB", maxAttachmentBytes>>20)
	}
	return data, nil
}

// mediaIndex finds the image attachment a media node shows.
type mediaIndex struct {
	// byUUID maps the media id on an ADF node to an attachment ID. It comes
	// from the images in Jira's rendered HTML, which carry both.
	byUUID map[string]string
	// byName maps a file name to an attachment ID, for nodes whose alt text
	// is still the file name.
	byName map[string]string
}

// lookup matches by media id first, since alt text is whatever the author
// typed, and falls back to the file name.
func (x *mediaIndex) lookup(uuid, alt string) (string, bool) {
	if x == nil {
		return "", false
	}
	if id, ok := x.byUUID[uuid]; ok {
		return id, true
	}
	id, ok := x.byName[alt]
	return id, ok
}

// attachmentSrc picks the attachment ID out of an image URL.
var attachmentSrc = regexp.MustCompile(`/attachment/(?:content/)?(\d+)`)

// imageAttachments indexes an issue's image attachments so the media in docs
// can be matched to them. rendered is Jira's HTML for the same text, which
// pairs media ids with attachments exactly. It fetches nothing unless one of
// the documents has media, and returns nil when the lookup fails, which leaves
// the media as text.
func (c *Client) imageAttachments(key string, rendered []string, docs ...any) *mediaIndex {
	if !anyMedia(docs) {
		return nil
	}
	var res struct {
		Fields struct {
			Attachment []struct {
				ID       string `json:"id"`
				Filename string `json:"filename"`
				MimeType string `json:"mimeType"`
			} `json:"attachment"`
		} `json:"fields"`
		RenderedFields struct {
			Description string `json:"description"`
		} `json:"renderedFields"`
	}
	path := fmt.Sprintf("/issue/%s?fields=attachment,description&expand=renderedFields", url.PathEscape(key))
	if err := c.getJSON(c.jc.Get, path, &res); err != nil {
		return nil
	}
	idx := &mediaIndex{byUUID: map[string]string{}, byName: map[string]string{}}
	images := map[string]bool{}
	for _, a := range res.Fields.Attachment {
		if imageTypes[a.MimeType] {
			idx.byName[a.Filename] = a.ID
			images[a.ID] = true
		}
	}
	for _, html := range append([]string{res.RenderedFields.Description}, rendered...) {
		maps.Copy(idx.byUUID, pairRendered(html, images))
	}
	return idx
}

// pairRendered maps the media id of each image in doc to the ID of the
// attachment it shows, for the attachments in images.
func pairRendered(doc string, images map[string]bool) map[string]string {
	out := map[string]string{}
	root, err := html.Parse(strings.NewReader(doc))
	if err != nil {
		return out
	}
	for n := range root.Descendants() {
		if n.Type != html.ElementNode || n.Data != "img" {
			continue
		}
		var uuid, src string
		for _, a := range n.Attr {
			switch a.Key {
			case "data-media-services-id":
				uuid = a.Val
			case "src":
				src = a.Val
			}
		}
		if m := attachmentSrc.FindStringSubmatch(src); uuid != "" && m != nil && images[m[1]] {
			out[uuid] = m[1]
		}
	}
	return out
}

func anyMedia(docs []any) bool {
	for _, d := range docs {
		if doc, ok := d.(*adf.ADF); ok && doc != nil && hasMedia(doc.Content) {
			return true
		}
	}
	return false
}

func hasMedia(nodes []*adf.Node) bool {
	for _, n := range nodes {
		if n.NodeType == adf.NodeMedia || hasMedia(n.Content) {
			return true
		}
	}
	return false
}

// mediaHook writes a media node that matches an image attachment as an image
// link on its own paragraph, and any other as [attachment].
func mediaHook(idx *mediaIndex) func(adf.Connector) string {
	return func(n adf.Connector) string {
		attrs, _ := n.GetAttributes().(map[string]any)
		uuid, _ := attrs["id"].(string)
		alt, _ := attrs["alt"].(string)
		if id, ok := idx.lookup(uuid, alt); ok {
			label := strings.NewReplacer("[", "", "]", "", "\n", " ").Replace(alt)
			return "\n\n![" + label + "](" + MediaLinkPrefix + url.PathEscape(id) + ")\n\n"
		}
		return "\n[attachment]"
	}
}
