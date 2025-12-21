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

	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	var urls []string

	// Extract images
	if mediaType == MediaTypeImage || mediaType == MediaTypeAll {
		doc.Find("img").Each(func(i int, s *goquery.Selection) {
			if src, exists := s.Attr("src"); exists {
				if resolved := resolveURL(base, src); resolved != "" {
					urls = append(urls, resolved)
				}
			}
			// Also check srcset for high-res images
			if srcset, exists := s.Attr("srcset"); exists {
				srcsetURLs := parseSrcset(srcset, base)
				urls = append(urls, srcsetURLs...)
			}
		})

		// Also check Open Graph images
		doc.Find("meta[property='og:image']").Each(func(i int, s *goquery.Selection) {
			if content, exists := s.Attr("content"); exists {
				if resolved := resolveURL(base, content); resolved != "" {
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
				if resolved := resolveURL(base, src); resolved != "" {
					urls = append(urls, resolved)
				}
			}
		})

		// Direct <video> src
		doc.Find("video").Each(func(i int, s *goquery.Selection) {
			if src, exists := s.Attr("src"); exists {
				if resolved := resolveURL(base, src); resolved != "" {
					urls = append(urls, resolved)
				}
			}
		})

		// Open Graph videos
		doc.Find("meta[property='og:video'], meta[property='og:video:url']").Each(func(i int, s *goquery.Selection) {
			if content, exists := s.Attr("content"); exists {
				if resolved := resolveURL(base, content); resolved != "" {
					urls = append(urls, resolved)
				}
			}
		})

		// Try to extract from JSON blobs
		urls = append(urls, extractVideosFromJSON(html, baseURL)...)
	}

	// Extract audio
	if mediaType == MediaTypeAudio || mediaType == MediaTypeAll {
		doc.Find("audio source").Each(func(i int, s *goquery.Selection) {
			if src, exists := s.Attr("src"); exists {
				if resolved := resolveURL(base, src); resolved != "" {
					urls = append(urls, resolved)
				}
			}
		})

		doc.Find("audio").Each(func(i int, s *goquery.Selection) {
			if src, exists := s.Attr("src"); exists {
				if resolved := resolveURL(base, src); resolved != "" {
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
func resolveURL(base *url.URL, href string) string {
	// Skip data URLs and empty strings
	if href == "" || strings.HasPrefix(href, "data:") {
		return ""
	}

	// If already absolute, return as-is
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
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
func parseSrcset(srcset string, base *url.URL) []string {
	var urls []string
	parts := strings.Split(srcset, ",")
	for _, part := range parts {
		// Each part is like "image.jpg 2x" or "image.jpg 1024w"
		tokens := strings.Fields(strings.TrimSpace(part))
		if len(tokens) > 0 {
			if resolved := resolveURL(base, tokens[0]); resolved != "" {
				urls = append(urls, resolved)
			}
		}
	}
	return urls
}

var (
	jsonPatterns = []*regexp.Regexp{
		regexp.MustCompile(`<script[^>]*id=["']__NEXT_DATA__["'][^>]*>(.*?)</script>`),
		regexp.MustCompile(`<script[^>]*type=["']application/ld\+json["'][^>]*>(.*?)</script>`),
		regexp.MustCompile(`window\.__INITIAL_STATE__\s*=\s*({.*?});`),
	}
	videoURLPattern = regexp.MustCompile(`https?://[^\s"'<>]+\.(?:mp4|m3u8|webm|mov)(?:\?[^\s"'<>]*)?`)
)

// extractVideosFromJSON looks for video URLs in JSON blobs.
func extractVideosFromJSON(html string, _ string) []string {
	var urls []string

	for _, re := range jsonPatterns {
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
	matches := videoURLPattern.FindAllString(jsonData, -1)
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

// detectMediaType determines the media type from URL and optional Content-Type
func detectMediaType(urlStr string, contentType string) MediaType {
	// Check Content-Type header first (more reliable)
	if contentType != "" {
		if strings.HasPrefix(contentType, "image/") {
			return MediaTypeImage
		}
		if strings.HasPrefix(contentType, "video/") {
			return MediaTypeVideo
		}
		if strings.HasPrefix(contentType, "audio/") {
			return MediaTypeAudio
		}
	}

	// Fallback to extension
	u, err := url.Parse(urlStr)
	if err != nil {
		return MediaTypeAll // Unknown
	}

	path := strings.ToLower(u.Path)
	if hasExtension(path, imageExtensions) {
		return MediaTypeImage
	}
	if hasExtension(path, videoExtensions) {
		return MediaTypeVideo
	}
	if hasExtension(path, audioExtensions) {
		return MediaTypeAudio
	}

	return MediaTypeAll
}

var (
	imageExtensions = []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".bmp", ".ico", ".tiff", ".tif"}
	videoExtensions = []string{".mp4", ".webm", ".mov", ".avi", ".mkv", ".flv", ".m3u8", ".ts", ".wmv"}
	audioExtensions = []string{".mp3", ".wav", ".ogg", ".aac", ".flac", ".m4a", ".wma"}
)

func hasExtension(path string, extensions []string) bool {
	for _, ext := range extensions {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

// isValidMediaURL checks if a URL looks like a valid media URL
func isValidMediaURL(urlStr string) bool {
	// Must be HTTP/HTTPS
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		return false
	}

	// Check if detectMediaType finds a specific type
	if detectMediaType(urlStr, "") != MediaTypeAll {
		return true
	}

	// Parse URL for further checks
	u, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	// Check query parameters for format indicators
	if u.RawQuery != "" {
		query := strings.ToLower(u.RawQuery)
		if strings.Contains(query, "format=jpg") || strings.Contains(query, "format=png") ||
			strings.Contains(query, "type=image") || strings.Contains(query, "type=video") {
			return true
		}
	}

	// Also allow URLs with video/image in the path
	if strings.Contains(strings.ToLower(u.String()), "video") || strings.Contains(strings.ToLower(u.String()), "image") {
		return true
	}

	return false
}
