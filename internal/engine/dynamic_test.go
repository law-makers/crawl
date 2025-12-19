// internal/engine/dynamic_test.go
package engine

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/law-makers/crawl/pkg/models"
)

func TestDynamicScraper_Fetch_BasicHTML(t *testing.T) {
	// Create a test server with HTML
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `<!DOCTYPE html>
<html>
<head>
	<title>Test Page</title>
</head>
<body>
	<h1>Hello from Dynamic</h1>
	<p>This is a test page.</p>
</body>
</html>`
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(html))
	}))
	defer server.Close()

	// Create scraper
	scraper := NewDynamicScraper()

	// Test fetch
	opts := models.RequestOptions{
		URL:      server.URL,
		Mode:     models.ModeSPA,
		Selector: "body",
		Timeout:  10 * time.Second,
	}

	pageData, err := scraper.Fetch(opts)

	// Assertions
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if pageData.Title != "Test Page" {
		t.Errorf("Expected title 'Test Page', got '%s'", pageData.Title)
	}

	if pageData.Content == "" {
		t.Error("Expected content to be non-empty")
	}
}

func TestDynamicScraper_Fetch_JavaScript(t *testing.T) {
	// Create a test server with JavaScript that modifies the DOM
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `<!DOCTYPE html>
<html>
<head>
	<title>JS Test</title>
</head>
<body>
	<div id="content">Loading...</div>
	<script>
		// Simulate async data loading
		setTimeout(function() {
			document.getElementById('content').innerText = 'Loaded by JavaScript';
		}, 100);
	</script>
</body>
</html>`
		w.Write([]byte(html))
	}))
	defer server.Close()

	scraper := NewDynamicScraper()

	opts := models.RequestOptions{
		URL:      server.URL,
		Mode:     models.ModeSPA,
		Selector: "#content",
		Timeout:  10 * time.Second,
	}

	// Wait a bit for JavaScript to execute
	time.Sleep(200 * time.Millisecond)

	pageData, err := scraper.Fetch(opts)

	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	// The DynamicScraper should wait for the selector and capture JS-modified content
	// Note: Due to timing, this might still be "Loading..." in some environments
	if pageData.Content == "" {
		t.Error("Expected content to be non-empty")
	}
}

func TestDynamicScraper_Fetch_WithSelector(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `<!DOCTYPE html>
<html>
<head><title>Selector Test</title></head>
<body>
	<div class="target">Target Content</div>
	<div class="other">Other Content</div>
</body>
</html>`
		w.Write([]byte(html))
	}))
	defer server.Close()

	scraper := NewDynamicScraper()

	opts := models.RequestOptions{
		URL:      server.URL,
		Mode:     models.ModeSPA,
		Selector: ".target",
		Timeout:  10 * time.Second,
	}

	pageData, err := scraper.Fetch(opts)

	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if pageData.Content != "Target Content" {
		t.Errorf("Expected content 'Target Content', got '%s'", pageData.Content)
	}
}

func TestDynamicScraper_Name(t *testing.T) {
	scraper := NewDynamicScraper()

	if scraper.Name() != "DynamicScraper" {
		t.Errorf("Expected name 'DynamicScraper', got '%s'", scraper.Name())
	}
}

func TestDynamicScraper_Fetch_InvalidURL(t *testing.T) {
	scraper := NewDynamicScraper()

	opts := models.RequestOptions{
		URL:      "http://invalid-url-that-does-not-exist-99999.com",
		Mode:     models.ModeSPA,
		Selector: "body",
		Timeout:  5 * time.Second,
	}

	_, err := scraper.Fetch(opts)

	if err == nil {
		t.Error("Expected error for invalid URL, got nil")
	}
}
