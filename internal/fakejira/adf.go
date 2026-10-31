package fakejira

import (
	"regexp"
	"strings"
)

var (
	headingRe = regexp.MustCompile(`^h([1-6])\.\s+(.*)$`)
	codeRe    = regexp.MustCompile(`^\{code(:[^}]*)?\}$`)
)

type node = map[string]any

// toADF converts the Jira wiki markup the fake stores to an Atlassian Document
// Format document, as the v3 API returns rich text. It handles headings,
// bullet and numbered lists, code blocks and paragraphs, and leaves inline
// markup as plain text.
func toADF(wiki string) node {
	var content []any
	var para []string
	var list node
	var listType string

	flushPara := func() {
		if len(para) == 0 {
			return
		}
		var inline []any
		for i, l := range para {
			if i > 0 {
				inline = append(inline, node{"type": "hardBreak"})
			}
			inline = append(inline, text(l))
		}
		content = append(content, node{"type": "paragraph", "content": inline})
		para = nil
	}
	flushList := func() {
		if list != nil {
			content = append(content, list)
			list, listType = nil, ""
		}
	}

	lines := strings.Split(strings.ReplaceAll(wiki, "\r\n", "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if m := codeRe.FindStringSubmatch(trimmed); m != nil {
			flushPara()
			flushList()
			var code []string
			for i++; i < len(lines) && strings.TrimSpace(lines[i]) != "{code}"; i++ {
				code = append(code, lines[i])
			}
			block := node{"type": "codeBlock", "content": []any{text(strings.Join(code, "\n"))}}
			if lang := strings.TrimPrefix(m[1], ":"); lang != "" {
				block["attrs"] = node{"language": lang}
			}
			content = append(content, block)
			continue
		}

		if m := headingRe.FindStringSubmatch(trimmed); m != nil {
			flushPara()
			flushList()
			content = append(content, node{
				"type":    "heading",
				"attrs":   node{"level": int(m[1][0] - '0')},
				"content": []any{text(m[2])},
			})
			continue
		}

		if typ, item, ok := listItem(trimmed); ok {
			flushPara()
			if listType != typ {
				flushList()
				list, listType = node{"type": typ, "content": []any{}}, typ
			}
			list["content"] = append(list["content"].([]any), node{
				"type":    "listItem",
				"content": []any{node{"type": "paragraph", "content": []any{text(item)}}},
			})
			continue
		}

		flushList()
		if trimmed == "" {
			flushPara()
			continue
		}
		para = append(para, line)
	}
	flushPara()
	flushList()

	if content == nil {
		content = []any{}
	}
	return node{"version": 1, "type": "doc", "content": content}
}

func listItem(line string) (typ, item string, ok bool) {
	if item, ok := strings.CutPrefix(line, "* "); ok {
		return "bulletList", item, true
	}
	if item, ok := strings.CutPrefix(line, "- "); ok {
		return "bulletList", item, true
	}
	if item, ok := strings.CutPrefix(line, "# "); ok {
		return "orderedList", item, true
	}
	return "", "", false
}

func text(s string) node {
	return node{"type": "text", "text": s}
}
