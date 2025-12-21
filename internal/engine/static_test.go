// internal/engine/static_test.go
package engine

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/law-makers/crawl/pkg/models"
)

func TestStaticScraper_Fetch_BasicHTML(t *testing.T) {
	// Create a test server with dummy HTML
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `<!DOCTYPE html>
<html>
<head>
	<title>Hello World</title>
	<meta name="description" content="Test page">
</head>
<body>
	<h1>Hello World</h1>
	<p>This is a test page.</p>
	<a href="/link1">Link 1</a>
	<a href="/link2">Link 2</a>
	<img src="/image.jpg" alt="Test Image">
</body>
</html>`
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(html))
	}))
	defer server.Close()

	// Create scraper using test helper
	scraper := NewTestStaticScraper()

	// Test fetch
	opts := models.RequestOptions{
		URL:      server.URL,
		Mode:     models.ModeStatic,
		Selector: "body",
		Timeout:  5 * time.Second,
	}

	pageData, err := scraper.Fetch(opts)

	// Assertions
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if pageData.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %d", pageData.StatusCode)
	}

	if pageData.Title != "Hello World" {
		t.Errorf("Expected title 'Hello World', got '%s'", pageData.Title)
	}

	if len(pageData.Links) != 2 {
		t.Errorf("Expected 2 links, got %d", len(pageData.Links))
	}

	if len(pageData.Images) != 1 {
		t.Errorf("Expected 1 image, got %d", len(pageData.Images))
	}

	if pageData.Metadata["description"] != "Test page" {
		t.Errorf("Expected metadata description 'Test page', got '%s'", pageData.Metadata["description"])
	}
}

func TestStaticScraper_Fetch_WithSelector(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body>
	<div class="price-tag">$99.99</div>
	<div class="other">Other content</div>
</body>
</html>`
		w.Write([]byte(html))
	}))
	defer server.Close()

	scraper := NewTestStaticScraper()

	// Test with selector
	opts := models.RequestOptions{
		URL:      server.URL,
		Mode:     models.ModeStatic,
		Selector: ".price-tag",
		Timeout:  5 * time.Second,
	}

	pageData, err := scraper.Fetch(opts)

	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if pageData.Content != "$99.99" {
		t.Errorf("Expected content '$99.99', got '%s'", pageData.Content)
	}
}

func TestStaticScraper_Fetch_InvalidURL(t *testing.T) {
	scraper := NewTestStaticScraper()

	opts := models.RequestOptions{
		URL:      "http://invalid-url-that-does-not-exist-12345.com",
		Mode:     models.ModeStatic,
		Selector: "body",
		Timeout:  2 * time.Second,
	}

	_, err := scraper.Fetch(opts)

	if err == nil {
		t.Error("Expected error for invalid URL, got nil")
	}
}

func TestStaticScraper_Name(t *testing.T) {
	scraper := NewTestStaticScraper()

	if scraper.Name() != "StaticScraper" {
		t.Errorf("Expected name 'StaticScraper', got '%s'", scraper.Name())
	}
}

func TestStaticScraper_Fetch_CustomHeaders(t *testing.T) {
	// Create a test server that checks for custom headers
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent := r.Header.Get("User-Agent")
		customHeader := r.Header.Get("X-Custom-Header")

		html := `<!DOCTYPE html>
<html>
<head><title>Headers Test</title></head>
<body>
	<p>User-Agent: ` + userAgent + `</p>
	<p>Custom: ` + customHeader + `</p>
</body>
</html>`
		w.Write([]byte(html))
	}))
	defer server.Close()

	scraper := NewTestStaticScraper()

	opts := models.RequestOptions{
		URL:      server.URL,
		Mode:     models.ModeStatic,
		Selector: "body",
		Headers: map[string]string{
			"X-Custom-Header": "TestValue",
		},
		Timeout: 5 * time.Second,
	}

	pageData, err := scraper.Fetch(opts)

	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if pageData.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %d", pageData.StatusCode)
	}
}
