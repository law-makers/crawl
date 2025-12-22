// internal/engine/metadata/utils.go
package metadata

import (
	"strings"
)

// NormalizeURL normalizes a URL for consistency
func NormalizeURL(url string) string {
	return strings.TrimSpace(url)
}

// IsAbsoluteURL checks if a URL is absolute
func IsAbsoluteURL(url string) bool {
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

// ExtractDomain extracts the domain from a URL
func ExtractDomain(url string) string {
	// Simple domain extraction
	if strings.HasPrefix(url, "http://") {
		url = url[7:]
	} else if strings.HasPrefix(url, "https://") {
		url = url[8:]
	}

	if idx := strings.Index(url, "/"); idx > 0 {
		url = url[:idx]
	}

	return url
}

// FilterUniqueLinks removes duplicate links
func FilterUniqueLinks(links []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(links))

	for _, link := range links {
		if !seen[link] {
			seen[link] = true
			result = append(result, link)
		}
	}

	return result
}
