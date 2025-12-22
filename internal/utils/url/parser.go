package urlutil

import (
	"fmt"
	"net/url"

	"github.com/law-makers/crawl/pkg/models"
)

// ValidateURL performs comprehensive URL validation
func ValidateURL(urlStr string) error {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("invalid URL scheme: must be http or https, got %s", parsed.Scheme)
	}

	if parsed.Host == "" {
		return fmt.Errorf("invalid URL: missing host")
	}

	return nil
}

// ResolveURL resolves a possibly-relative href against a base URL and returns a string
func ResolveURL(base, href string) string {
	u, err := url.Parse(href)
	if err != nil {
		return href
	}
	if u.IsAbs() {
		return href
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return href
	}
	return baseURL.ResolveReference(u).String()
}

// ResolveRelativeLinks updates all link-like fields in PageData to absolute URLs
func ResolveRelativeLinks(data *models.PageData) {
	// Resolve Links
	resolvedLinks := make([]string, len(data.Links))
	for i, link := range data.Links {
		resolvedLinks[i] = ResolveURL(data.URL, link)
	}
	data.Links = resolvedLinks

	// Resolve Images
	resolvedImages := make([]string, len(data.Images))
	for i, img := range data.Images {
		resolvedImages[i] = ResolveURL(data.URL, img)
	}
	data.Images = resolvedImages

	// Resolve Scripts
	resolvedScripts := make([]string, len(data.Scripts))
	for i, script := range data.Scripts {
		resolvedScripts[i] = ResolveURL(data.URL, script)
	}
	data.Scripts = resolvedScripts
}
