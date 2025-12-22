// internal/engine/hybrid/detector.go
package hybrid

import (
	"strings"
)

// DetectJavaScriptFramework detects common JS frameworks in HTML
func DetectJavaScriptFramework(html string) string {
	html = strings.ToLower(html)

	if strings.Contains(html, "react") || strings.Contains(html, "_react") {
		return "React"
	}
	if strings.Contains(html, "vue") || strings.Contains(html, "vuejs") {
		return "Vue"
	}
	if strings.Contains(html, "angular") || strings.Contains(html, "ng-app") {
		return "Angular"
	}
	if strings.Contains(html, "ember") {
		return "Ember"
	}
	if strings.Contains(html, "svelte") {
		return "Svelte"
	}

	return "Unknown"
}

// NeedsJavaScript determines if a page likely needs JS rendering
func NeedsJavaScript(html string, scriptCount int) bool {
	// If there are many scripts, likely needs JS
	if scriptCount > 5 {
		return true
	}

	// Check for SPA indicators
	framework := DetectJavaScriptFramework(html)
	if framework != "Unknown" {
		return true
	}

	// Check for minimal HTML content (typical of SPAs)
	if strings.Count(html, "<div") < 3 && scriptCount > 0 {
		return true
	}

	return false
}
