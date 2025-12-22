// internal/engine/dynamic/extractor.go
package dynamic

import (
	"context"
	"strings"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	"github.com/law-makers/crawl/pkg/models"
	"github.com/rs/zerolog/log"
)

// extractDataFromHTML extracts links, images, scripts, and content from the page
func extractDataFromHTML(ctx context.Context, opts models.RequestOptions, pageData *models.PageData) error {
	// Extract content based on selector
	selector := opts.Selector
	if selector != "" && selector != "body" {
		var content string
		var html string
		err := chromedp.Run(ctx,
			chromedp.Text(selector, &content, chromedp.ByQuery),
			chromedp.OuterHTML(selector, &html, chromedp.ByQuery),
		)
		if err == nil {
			pageData.Content = strings.TrimSpace(content)
		} else {
			log.Warn().Str("selector", selector).Msg("Selector not found")
		}
	} else {
		// Extract body text
		var bodyText string
		err := chromedp.Run(ctx, chromedp.Text("body", &bodyText, chromedp.ByQuery))
		if err == nil {
			pageData.Content = strings.TrimSpace(bodyText)
		}
	}

	// Extract links
	var links []*cdp.Node
	err := chromedp.Run(ctx, chromedp.Nodes("a[href]", &links, chromedp.ByQueryAll))
	if err == nil {
		for _, node := range links {
			if href, ok := node.Attribute("href"); ok && href != "" {
				pageData.Links = append(pageData.Links, href)
			}
		}
	}

	// Extract images
	var images []*cdp.Node
	err = chromedp.Run(ctx, chromedp.Nodes("img[src]", &images, chromedp.ByQueryAll))
	if err == nil {
		for _, node := range images {
			if src, ok := node.Attribute("src"); ok && src != "" {
				pageData.Images = append(pageData.Images, src)
			}
		}
	}

	// Extract scripts
	var scripts []*cdp.Node
	err = chromedp.Run(ctx, chromedp.Nodes("script[src]", &scripts, chromedp.ByQueryAll))
	if err == nil {
		for _, node := range scripts {
			if src, ok := node.Attribute("src"); ok && src != "" {
				pageData.Scripts = append(pageData.Scripts, src)
			}
		}
	}

	// Extract metadata
	var metaTags []*cdp.Node
	err = chromedp.Run(ctx, chromedp.Nodes("meta", &metaTags, chromedp.ByQueryAll))
	if err == nil {
		for _, node := range metaTags {
			if name, ok := node.Attribute("name"); ok {
				if content, ok := node.Attribute("content"); ok {
					pageData.Metadata[name] = content
				}
			}
			if property, ok := node.Attribute("property"); ok {
				if content, ok := node.Attribute("content"); ok {
					pageData.Metadata[property] = content
				}
			}
		}
	}

	return nil
}
