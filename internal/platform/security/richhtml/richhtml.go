// Package richhtml provides allowlist HTML sanitization for admin rich-text fields.
package richhtml

import (
	"bytes"
	"io"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

var allowedElements = map[string]struct{}{
	"p": {}, "br": {}, "strong": {}, "b": {}, "em": {}, "i": {},
	"ul": {}, "ol": {}, "li": {}, "a": {}, "blockquote": {},
	"h2": {}, "h3": {}, "h4": {},
}

// Sanitize returns allowlisted HTML. Plain text without tags is returned trimmed.
// Unsafe tags/attributes/URLs are removed. Empty input yields "".
func Sanitize(input string) string {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return ""
	}
	if !strings.ContainsAny(raw, "<>") {
		return raw
	}

	doc, err := html.Parse(strings.NewReader("<div>" + raw + "</div>"))
	if err != nil {
		return stripTagsFallback(raw)
	}
	body := findBody(doc)
	if body == nil {
		return stripTagsFallback(raw)
	}
	var buf bytes.Buffer
	for c := body.FirstChild; c != nil; c = c.NextSibling {
		// Parser wraps content in <html><head/><body><div>...</div></body>
		if c.Type == html.ElementNode && strings.EqualFold(c.Data, "div") {
			for gc := c.FirstChild; gc != nil; gc = gc.NextSibling {
				writeSanitized(&buf, gc)
			}
			continue
		}
		writeSanitized(&buf, c)
	}
	return strings.TrimSpace(buf.String())
}

func findBody(n *html.Node) *html.Node {
	if n == nil {
		return nil
	}
	if n.Type == html.ElementNode && strings.EqualFold(n.Data, "body") {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findBody(c); found != nil {
			return found
		}
	}
	return nil
}

// SanitizeOptional sanitizes a nullable string pointer.
func SanitizeOptional(v *string) *string {
	if v == nil {
		return nil
	}
	s := Sanitize(*v)
	if s == "" {
		return nil
	}
	return &s
}

func writeSanitized(w io.Writer, n *html.Node) {
	switch n.Type {
	case html.TextNode:
		_, _ = io.WriteString(w, html.EscapeString(n.Data))
	case html.ElementNode:
		tag := strings.ToLower(n.Data)
		if _, ok := allowedElements[tag]; !ok {
			// Drop element but keep children text (e.g. strip <script>).
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if tag == "script" || tag == "style" || tag == "iframe" || tag == "object" || tag == "embed" {
					continue
				}
				writeSanitized(w, c)
			}
			return
		}
		if tag == "br" {
			_, _ = io.WriteString(w, "<br>")
			return
		}
		if tag == "a" {
			href := anchorHref(n.Attr)
			if !safeHref(href) {
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					writeSanitized(w, c)
				}
				return
			}
			_, _ = io.WriteString(w, "<a")
			writeAnchorAttrs(w, n.Attr)
			_, _ = io.WriteString(w, ">")
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				writeSanitized(w, c)
			}
			_, _ = io.WriteString(w, "</a>")
			return
		}
		_, _ = io.WriteString(w, "<"+tag)
		_, _ = io.WriteString(w, ">")
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			writeSanitized(w, c)
		}
		_, _ = io.WriteString(w, "</"+tag+">")
	case html.DocumentNode:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			writeSanitized(w, c)
		}
	default:
		// skip comments, doctype, etc.
	}
}

func writeAnchorAttrs(w io.Writer, attrs []html.Attribute) {
	var href string
	var title string
	var targetBlank bool
	for _, a := range attrs {
		key := strings.ToLower(a.Key)
		switch key {
		case "href":
			href = strings.TrimSpace(a.Val)
		case "title":
			title = a.Val
		case "target":
			if strings.EqualFold(a.Val, "_blank") {
				targetBlank = true
			}
		}
	}
	_, _ = io.WriteString(w, ` href="`+html.EscapeString(href)+`"`)
	if title != "" {
		_, _ = io.WriteString(w, ` title="`+html.EscapeString(title)+`"`)
	}
	if targetBlank {
		_, _ = io.WriteString(w, ` target="_blank" rel="noopener noreferrer"`)
	}
}

func anchorHref(attrs []html.Attribute) string {
	for _, a := range attrs {
		if strings.EqualFold(a.Key, "href") {
			return strings.TrimSpace(a.Val)
		}
	}
	return ""
}

func safeHref(href string) bool {
	if href == "" {
		return false
	}
	lower := strings.ToLower(href)
	if strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "vbscript:") {
		return false
	}
	if strings.HasPrefix(href, "#") || strings.HasPrefix(href, "/") {
		return true
	}
	u, err := url.Parse(href)
	if err != nil || u.Scheme == "" {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "mailto":
		return true
	default:
		return false
	}
}

func stripTagsFallback(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}
