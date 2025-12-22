package output

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// CleanHTML removes unwanted elements and attributes to produce a safe HTML excerpt
func CleanHTML(htmlContent string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return "", err
	}

	// Remove unwanted tags
	doc.Find("script, style, link, meta, noscript, iframe, svg, form, input, button, select, textarea, canvas").Remove()

	// Clean attributes
	doc.Find("*").Each(func(i int, s *goquery.Selection) {
		if len(s.Nodes) == 0 {
			return
		}
		node := s.Nodes[0]
		var newAttrs []html.Attribute
		for _, attr := range node.Attr {
			keep := false
			switch node.Data {
			case "a":
				if attr.Key == "href" || attr.Key == "title" {
					keep = true
				}
			case "img":
				if attr.Key == "src" || attr.Key == "alt" || attr.Key == "title" {
					keep = true
				}
			default:
				// keep no attributes by default
			}
			if keep {
				newAttrs = append(newAttrs, attr)
			}
		}
		node.Attr = newAttrs
	})

	// Return sanitized HTML (preserve tags for downstream converters)
	htmlStr, err := doc.Html()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(htmlStr), nil
}

// PrettyPrint returns an indented human-readable representation of an HTML node tree.
func PrettyPrint(n *html.Node) string {
	var sb strings.Builder
	var f func(*html.Node, int)
	f = func(n *html.Node, depth int) {
		indent := strings.Repeat("  ", depth)
		switch n.Type {
		case html.DocumentNode:
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				f(c, depth)
			}
		case html.ElementNode:
			sb.WriteString(fmt.Sprintf("%s<%s", indent, n.Data))
			for _, a := range n.Attr {
				sb.WriteString(fmt.Sprintf(" %s=\"%s\"", a.Key, a.Val))
			}
			if isVoidElement(n.Data) {
				sb.WriteString(">\n")
				return
			}
			sb.WriteString(">\n")

			for c := n.FirstChild; c != nil; c = c.NextSibling {
				f(c, depth+1)
			}
			sb.WriteString(fmt.Sprintf("%s</%s>\n", indent, n.Data))
		case html.TextNode:
			text := strings.TrimSpace(n.Data)
			if text != "" {
				sb.WriteString(fmt.Sprintf("%s%s\n", indent, text))
			}
		case html.DoctypeNode:
			sb.WriteString(fmt.Sprintf("<!DOCTYPE %s>\n", n.Data))
		}
	}
	f(n, 0)
	return sb.String()
}

func isVoidElement(tag string) bool {
	switch tag {
	case "area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr":
		return true
	}
	return false
}
