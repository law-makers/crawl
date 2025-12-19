// internal/downloader/extractor.go
package downloader

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// MediaType represents the type of media to extract
type MediaType string

const (
	MediaTypeImage MediaType = "image"
	MediaTypeVideo MediaType = "video"
	MediaTypeAudio MediaType = "audio"
	MediaTypeAll   MediaType = "all"
)

// ExtractMedia extracts media URLs from HTML based on the specified type
func ExtractMedia(html string, baseURL string, mediaType MediaType) ([]string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var urls []string

	// Extract images
	if mediaType == MediaTypeImage || mediaType == MediaTypeAll {
		doc.Find("img").Each(func(i int, s *goquery.Selection) {
			if src, exists := s.Attr("src"); exists {
				if resolved := resolveURL(baseURL, src); resolved != "" {
					urls = append(urls, resolved)
				}
			}
			// Also check srcset for high-res images
			if srcset, exists := s.Attr("srcset"); exists {
				srcsetURLs := parseSrcset(srcset, baseURL)
				urls = append(urls, srcsetURLs...)
			}
		})

		// Also check Open Graph images
		doc.Find("meta[property='og:image']").Each(func(i int, s *goquery.Selection) {
			if content, exists := s.Attr("content"); exists {
				if resolved := resolveURL(baseURL, content); resolved != "" {
					urls = append(urls, resolved)
				}
			}
		})
	}

	// Extract videos
	if mediaType == MediaTypeVideo || mediaType == MediaTypeAll {
		// Standard <video> tags
		doc.Find("video source").Each(func(i int, s *goquery.Selection) {
			if src, exists := s.Attr("src"); exists {
				if resolved := resolveURL(baseURL, src); resolved != "" {
					urls = append(urls, resolved)
				}
			}
		})

		// Direct <video> src
		doc.Find("video").Each(func(i int, s *goquery.Selection) {
			if src, exists := s.Attr("src"); exists {
				if resolved := resolveURL(baseURL, src); resolved != "" {
					urls = append(urls, resolved)
				}
			}
		})

		// Open Graph videos
		doc.Find("meta[property='og:video'], meta[property='og:video:url']").Each(func(i int, s *goquery.Selection) {
			if content, exists := s.Attr("content"); exists {
				if resolved := resolveURL(baseURL, content); resolved != "" {
					urls = append(urls, resolved)
				}
			}
		})

		// Try to extract from JSON blobs (TikTok, Instagram, etc.)
		urls = append(urls, extractVideosFromJSON(html, baseURL)...)
	}

	// Extract audio
	if mediaType == MediaTypeAudio || mediaType == MediaTypeAll {
		doc.Find("audio source").Each(func(i int, s *goquery.Selection) {
			if src, exists := s.Attr("src"); exists {
				if resolved := resolveURL(baseURL, src); resolved != "" {
					urls = append(urls, resolved)
				}
			}
		})

		doc.Find("audio").Each(func(i int, s *goquery.Selection) {
			if src, exists := s.Attr("src"); exists {
				if resolved := resolveURL(baseURL, src); resolved != "" {
					urls = append(urls, resolved)
				}
			}
		})
	}

	// Deduplicate URLs
	seen := make(map[string]bool)
	uniqueURLs := []string{}
	for _, u := range urls {
		if !seen[u] && isValidMediaURL(u) {
			seen[u] = true
			uniqueURLs = append(uniqueURLs, u)
		}
	}

	return uniqueURLs, nil
}

// resolveURL resolves relative URLs against the base URL
func resolveURL(baseURL, href string) string {
	// Skip data URLs and empty strings
	if href == "" || strings.HasPrefix(href, "data:") {
		return ""
	}

	// If already absolute, return as-is
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}

	// Parse base URL
	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}

	// Parse relative URL
	rel, err := url.Parse(href)
	if err != nil {
		return ""
	}

	// Resolve and return
	return base.ResolveReference(rel).String()
}

// parseSrcset parses the srcset attribute and extracts URLs
func parseSrcset(srcset string, baseURL string) []string {
	var urls []string
	parts := strings.Split(srcset, ",")
	for _, part := range parts {
		// Each part is like "image.jpg 2x" or "image.jpg 1024w"
		tokens := strings.Fields(strings.TrimSpace(part))
		if len(tokens) > 0 {
			if resolved := resolveURL(baseURL, tokens[0]); resolved != "" {
				urls = append(urls, resolved)
			}
		}
	}
	return urls
}

// extractVideosFromJSON looks for video URLs in JSON blobs (TikTok, Instagram, etc.)
func extractVideosFromJSON(html string, _ string) []string {
	var urls []string

	// Look for common JSON patterns
	patterns := []string{
		`<script[^>]*id=["']__NEXT_DATA__["'][^>]*>(.*?)</script>`,
		`<script[^>]*type=["']application/ld\+json["'][^>]*>(.*?)</script>`,
		`window\.__INITIAL_STATE__\s*=\s*({.*?});`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(html, -1)
		for _, match := range matches {
			if len(match) > 1 {
				jsonData := match[1]
				// Try to find video URLs in the JSON
				videoURLs := extractURLsFromJSON(jsonData)
				urls = append(urls, videoURLs...)
			}
		}
	}

	return urls
}

// extractURLsFromJSON extracts video URLs from JSON data
func extractURLsFromJSON(jsonData string) []string {
	var urls []string

	// Look for common video URL patterns
	urlPattern := regexp.MustCompile(`https?://[^\s"'<>]+\.(?:mp4|m3u8|webm|mov)(?:\?[^\s"'<>]*)?`)
	matches := urlPattern.FindAllString(jsonData, -1)
	urls = append(urls, matches...)

	// Try to parse as JSON and look for video-related fields
	var data interface{}
	if err := json.Unmarshal([]byte(jsonData), &data); err == nil {
		findVideoURLsInJSON(data, &urls)
	}

	return urls
}

// findVideoURLsInJSON recursively searches for video URLs in parsed JSON
func findVideoURLsInJSON(data interface{}, urls *[]string) {
	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			// Look for keys that typically contain video URLs
			lowerKey := strings.ToLower(key)
			if strings.Contains(lowerKey, "video") || strings.Contains(lowerKey, "playback") || strings.Contains(lowerKey, "download") {
				if strValue, ok := value.(string); ok && isValidMediaURL(strValue) {
					*urls = append(*urls, strValue)
				}
			}
			findVideoURLsInJSON(value, urls)
		}
	case []interface{}:
		for _, item := range v {
			findVideoURLsInJSON(item, urls)
		}
	}
}

// isValidMediaURL checks if a URL looks like a valid media URL
func isValidMediaURL(urlStr string) bool {
	// Must be HTTP/HTTPS
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		return false
	}

	// Parse URL
	u, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	// Check for common media file extensions
	path := strings.ToLower(u.Path)
	mediaExtensions := []string{
		".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".bmp",
		".mp4", ".webm", ".mov", ".avi", ".mkv", ".flv", ".m3u8",
		".mp3", ".wav", ".ogg", ".aac", ".flac",
	}

	for _, ext := range mediaExtensions {
		if strings.Contains(path, ext) {
			return true
		}
	}

	// Also allow URLs with video/image in the path
	if strings.Contains(strings.ToLower(u.String()), "video") || strings.Contains(strings.ToLower(u.String()), "image") {
		return true
	}

	return false
}
