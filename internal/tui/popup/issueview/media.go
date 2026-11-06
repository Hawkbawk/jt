package issueview

import (
	"bytes"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/hawkbawk/jt/internal/jira"
	"github.com/hawkbawk/jt/internal/styles"
	"github.com/hawkbawk/jt/internal/termimage"
)

// segment is a run of markdown, or the attachment an image link points to.
type segment struct {
	text  string
	media string
}

// split cuts markdown at the image links jira writes as paragraphs of their
// own. Links anywhere else, like inside a code block or a sentence, stay in
// the text.
func split(src string) []segment {
	if !strings.Contains(src, jira.MediaLinkPrefix) {
		return []segment{{text: src}}
	}
	doc := goldmark.DefaultParser().Parse(text.NewReader([]byte(src)))
	var segs []segment
	last := 0
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		p, ok := n.(*ast.Paragraph)
		if !ok || p.ChildCount() != 1 || p.Lines().Len() == 0 {
			continue
		}
		img, ok := p.FirstChild().(*ast.Image)
		if !ok || !bytes.HasPrefix(img.Destination, []byte(jira.MediaLinkPrefix)) {
			continue
		}
		lines := p.Lines()
		start, stop := lines.At(0).Start, lines.At(lines.Len()-1).Stop
		segs = append(segs,
			segment{text: src[last:start]},
			segment{media: string(img.Destination[len(jira.MediaLinkPrefix):])})
		last = stop
	}
	return append(segs, segment{text: src[last:]})
}

// imageCache keeps decoded images by attachment ID. An attachment's content
// doesn't change, and the terminal already holds what was uploaded.
var imageCache sync.Map

type imageStatus int

const (
	imageLoading imageStatus = iota
	imageReady
	imageFailed
)

type imageState struct {
	status imageStatus
	img    *termimage.Image
}

// imageMsg carries an attachment that was downloaded and decoded, or why it
// wasn't. fresh is set when the image still has to be sent to the terminal.
type imageMsg struct {
	id    int64
	media string
	img   *termimage.Image
	fresh bool
	err   error
}

// mediaIDs lists the attachments the loaded text links to, once each.
func (m *Model) mediaIDs() []string {
	var texts []string
	if m.issue != nil {
		texts = append(texts, m.issue.Description)
	}
	if m.list != nil {
		for _, c := range m.list.Items {
			texts = append(texts, c.Body)
		}
	}
	var ids []string
	seen := map[string]bool{}
	for _, t := range texts {
		for _, s := range split(t) {
			if s.media != "" && !seen[s.media] {
				seen[s.media] = true
				ids = append(ids, s.media)
			}
		}
	}
	return ids
}

// loadImages starts downloading the images in the loaded text that haven't
// been asked for yet. It does nothing when the terminal can't show them.
func (m *Model) loadImages() tea.Cmd {
	if !termimage.Supported() {
		return nil
	}
	var cmds []tea.Cmd
	for _, id := range m.mediaIDs() {
		if m.images[id] != nil {
			continue
		}
		m.images[id] = &imageState{}
		cmds = append(cmds, m.loadImage(id))
	}
	return tea.Batch(cmds...)
}

func (m *Model) loadImage(media string) tea.Cmd {
	b, id := m.backend, m.id
	return func() tea.Msg {
		if v, ok := imageCache.Load(media); ok {
			return imageMsg{id: id, media: media, img: v.(*termimage.Image)}
		}
		data, err := b.Attachment(media)
		var img *termimage.Image
		if err == nil {
			img, err = termimage.Decode(data)
		}
		if err != nil {
			return imageMsg{id: id, media: media, err: err}
		}
		imageCache.Store(media, img)
		return imageMsg{id: id, media: media, img: img, fresh: true}
	}
}

func (m *Model) imageLoaded(msg imageMsg) {
	st := m.images[msg.media]
	if msg.id != m.id || st == nil {
		return
	}
	if msg.err != nil {
		st.status = imageFailed
	} else {
		st.status, st.img = imageReady, msg.img
		if msg.fresh {
			m.pending = append(m.pending, msg.img.Upload())
		}
	}
	m.refresh()
}

// flush writes what the terminal has to be told before it can draw the
// images in the text.
func (m *Model) flush() tea.Cmd {
	if len(m.pending) == 0 {
		return nil
	}
	seq := strings.Join(m.pending, "")
	m.pending = nil
	return tea.Raw(seq)
}

// withImages wraps md so a link to an image shows the image, or the text it
// showed before images were supported when there's none to draw.
func (m *Model) withImages(md Markdown) Markdown {
	return func(src string, width int) string {
		segs := split(src)
		if len(segs) == 1 {
			return md(src, width)
		}
		var parts []string
		for _, s := range segs {
			switch {
			case s.media != "":
				parts = append(parts, m.imageBlock(s.media, width, md))
			case strings.TrimSpace(s.text) != "":
				parts = append(parts, strings.TrimRight(md(s.text, width), "\n"))
			}
		}
		return strings.Join(parts, "\n")
	}
}

func (m *Model) imageBlock(media string, width int, md Markdown) string {
	st := m.images[media]
	switch {
	case st == nil || st.status == imageFailed:
		return strings.TrimRight(md("[attachment]", width), "\n")
	case st.status == imageLoading:
		return styles.Dim().Render("Loading image…")
	}
	lines, seq := st.img.Place(width, m.vp.Height())
	if seq != "" {
		m.pending = append(m.pending, seq)
	}
	return strings.Join(lines, "\n")
}
