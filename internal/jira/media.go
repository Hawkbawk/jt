package jira

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

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

// attachment is the part of an issue's attachment list that media matching
// needs.
type attachment struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	MimeType string `json:"mimeType"`
	Size     int64  `json:"size"`
}

// mediaIndex finds the image attachment a media node shows.
type mediaIndex struct {
	// images holds the IDs of the attachments jt can show.
	images map[string]bool
	// byUUID maps the media id on an ADF node to an attachment ID. It comes
	// from the images in Jira's rendered HTML, which carry both.
	byUUID map[string]string
	// byName maps a file name to an attachment ID, for nodes whose alt text
	// is still the file name.
	byName map[string]string
}

// newMediaIndex indexes the images among atts, skipping any too big to
// download. rendered is Jira's HTML for the text the index will be used on,
// which pairs media ids with attachments exactly.
func newMediaIndex(atts []attachment, rendered ...string) *mediaIndex {
	idx := &mediaIndex{images: map[string]bool{}, byUUID: map[string]string{}, byName: map[string]string{}}
	for _, a := range atts {
		if imageTypes[a.MimeType] && a.Size <= maxAttachmentBytes {
			idx.images[a.ID] = true
			idx.byName[a.Filename] = a.ID
		}
	}
	for _, doc := range rendered {
		idx.pair(doc)
	}
	return idx
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

// pair records the media id of each image in doc against the ID of the
// attachment it shows, for the attachments the index holds.
func (x *mediaIndex) pair(doc string) {
	root, err := html.Parse(strings.NewReader(doc))
	if err != nil {
		return
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
		if m := attachmentSrc.FindStringSubmatch(src); uuid != "" && m != nil && x.images[m[1]] {
			x.byUUID[uuid] = m[1]
		}
	}
}

// attachmentsTTL is how long an issue's attachment list is reused. The details
// and comments of an issue are usually opened together, and attachments rarely
// change in between.
const attachmentsTTL = time.Minute

type cachedAttachments struct {
	list []attachment
	at   time.Time
}

func (c *Client) rememberAttachments(key string, list []attachment) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.attachments == nil {
		c.attachments = map[string]cachedAttachments{}
	}
	c.attachments[key] = cachedAttachments{list: list, at: time.Now()}
}

// issueAttachments lists an issue's attachments, from the last time they were
// seen when that was recent. It returns nil when the lookup fails, which
// leaves media as text.
func (c *Client) issueAttachments(key string) []attachment {
	c.mu.Lock()
	cached, ok := c.attachments[key]
	c.mu.Unlock()
	if ok && time.Since(cached.at) < attachmentsTTL {
		return cached.list
	}
	var res struct {
		Fields struct {
			Attachment []attachment `json:"attachment"`
		} `json:"fields"`
	}
	if err := c.getJSON(c.jc.Get, "/issue/"+url.PathEscape(key)+"?fields=attachment", &res); err != nil {
		return nil
	}
	c.rememberAttachments(key, res.Fields.Attachment)
	return res.Fields.Attachment
}

func anyMedia(docs []any) bool {
	return slices.ContainsFunc(docs, func(d any) bool {
		doc, ok := d.(*adf.ADF)
		return ok && doc != nil && hasMedia(doc.Content)
	})
}

func hasMedia(nodes []*adf.Node) bool {
	return slices.ContainsFunc(nodes, func(n *adf.Node) bool {
		return n.NodeType == adf.NodeMedia || hasMedia(n.Content)
	})
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
