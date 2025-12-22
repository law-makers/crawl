// internal/engine/metadata/extractor.go
package metadata

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/law-makers/crawl/pkg/models"
)

// Extract extracts metadata, links, images, and scripts from a goquery document
func Extract(doc *goquery.Document, pageData *models.PageData) {
	if doc == nil || pageData == nil {
		return
	}

	// Extract title
	pageData.Title = doc.Find("title").First().Text()

	// Extract metadata tags
	doc.Find("meta").Each(func(i int, sel *goquery.Selection) {
		if name, exists := sel.Attr("name"); exists {
			content, _ := sel.Attr("content")
			pageData.Metadata[name] = content
		}
		if property, exists := sel.Attr("property"); exists {
			content, _ := sel.Attr("content")
			pageData.Metadata[property] = content
		}
	})

	// Extract links
	doc.Find("a[href]").Each(func(i int, sel *goquery.Selection) {
		if href, exists := sel.Attr("href"); exists && href != "" {
			pageData.Links = append(pageData.Links, href)
		}
	})

	// Extract images
	doc.Find("img[src]").Each(func(i int, sel *goquery.Selection) {
		if src, exists := sel.Attr("src"); exists && src != "" {
			pageData.Images = append(pageData.Images, src)
		}
	})

	// Extract scripts
	doc.Find("script[src]").Each(func(i int, sel *goquery.Selection) {
		if src, exists := sel.Attr("src"); exists && src != "" {
			pageData.Scripts = append(pageData.Scripts, src)
		}
	})
}

// ExtractContent extracts content based on selector or defaults to body
func ExtractContent(doc *goquery.Document, selector string) (content string, html string) {
	if doc == nil {
		return "", ""
	}

	if selector != "" && selector != "body" {
		// Extract specific selector
		selection := doc.Find(selector)
		if selection.Length() > 0 {
			content = strings.TrimSpace(selection.Text())
			html, _ = selection.Html()
			return content, html
		}
	}

	// Default: extract body content
	content = strings.TrimSpace(doc.Find("body").Text())
	html, _ = doc.Find("html").Html()
	return content, html
}
