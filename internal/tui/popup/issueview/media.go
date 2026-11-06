package issueview

import (
	"bytes"
	"strings"

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

// Image blocks that haven't been asked for yet carry their attachment ID
// between these marks, so the rendered text says where they are. They're
// stripped before the text reaches the viewport.
const (
	markOpen  = "\uE000"
	markClose = "\uE001"
)

// imageRow is a line of the rendered text holding an image that hasn't been
// asked for yet.
type imageRow struct {
	media string
	line  int
}

// extractMarks removes the marks from rendered text and lists the lines they
// were on.
func extractMarks(s string) (string, []imageRow) {
	if !strings.Contains(s, markOpen) {
		return s, nil
	}
	var rows []imageRow
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		for {
			start := strings.Index(l, markOpen)
			if start < 0 {
				break
			}
			end := strings.Index(l[start:], markClose)
			if end < 0 {
				break
			}
			rows = append(rows, imageRow{media: l[start+len(markOpen) : start+end], line: i})
			l = l[:start] + l[start+end+len(markClose):]
		}
		lines[i] = l
	}
	return strings.Join(lines, "\n"), rows
}

// imageState is what a model knows about an image it asked for. With neither
// field set, it's still loading.
type imageState struct {
	img    *termimage.Image
	failed bool
}

// imageMsg carries an attachment that was downloaded and decoded, or why it
// wasn't.
type imageMsg struct {
	id    int64
	media string
	img   *termimage.Image
	err   error
}

// loadVisible starts downloading the images within a screen of the visible
// text that haven't been asked for yet, so scrolling toward an image loads it
// without fetching every image in a long thread up front.
func (m *Model) loadVisible() tea.Cmd {
	top := m.vp.YOffset() - m.vp.Height()
	bottom := m.vp.YOffset() + 2*m.vp.Height()
	var cmds []tea.Cmd
	for _, r := range m.imageRows {
		if r.line < top || r.line > bottom || m.images[r.media] != nil {
			continue
		}
		m.images[r.media] = &imageState{}
		cmds = append(cmds, m.loadImage(r.media))
	}
	return tea.Batch(cmds...)
}

func (m *Model) loadImage(media string) tea.Cmd {
	b, id := m.backend, m.id
	return func() tea.Msg {
		img, err := imageStore.get(media, func() (*termimage.Image, error) {
			data, err := b.Attachment(media)
			if err != nil {
				return nil, err
			}
			return termimage.Decode(data)
		})
		return imageMsg{id: id, media: media, img: img, err: err}
	}
}

func (m *Model) imageLoaded(msg imageMsg) {
	st := m.images[msg.media]
	if msg.id != m.id || st == nil {
		return
	}
	if msg.err != nil {
		st.failed = true
	} else {
		st.img = msg.img
		// Whichever popup gets the image first sends it, even if the one that
		// asked for it has closed since.
		if up := msg.img.TakeUpload(); up != "" {
			m.pending = append(m.pending, up)
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
	case st == nil && termimage.Supported():
		return markOpen + media + markClose + styles.Dim().Render("Loading image…")
	case st == nil || st.failed:
		return strings.TrimRight(md("[attachment]", width), "\n")
	case st.img == nil:
		return styles.Dim().Render("Loading image…")
	}
	// Each model places images under an ID of its own, so another popup
	// showing the same image at another size doesn't change this one's.
	p := st.img.Fit(uint64(m.id), width*4/5, m.vp.Height()*4/5)
	if m.placed[media] != p {
		m.placed[media] = p
		m.pending = append(m.pending, st.img.Sequence(p))
	}
	lines := st.img.Lines(p)
	pad := strings.Repeat(" ", max(0, (width-p.Cols)/2))
	for i, l := range lines {
		lines[i] = pad + l
	}
	return strings.Join(lines, "\n")
}
