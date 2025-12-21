package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/law-makers/crawl/pkg/models"
)

// BenchmarkStaticScraper measures static scraping performance
func BenchmarkStaticScraper(b *testing.B) {
	// Create a test server with realistic HTML
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `<!DOCTYPE html>
<html>
<head><title>Test Page</title></head>
<body>
	<nav>Navigation</nav>
	<header>Header</header>
	<main>
		<article>
			<h1>Main Article Title</h1>
			<p>This is a paragraph with some content.</p>
			<p>Another paragraph with more text content here.</p>
			<div class="content">
				<p>More content in nested divs.</p>
			</div>
		</article>
	</main>
	<footer>Footer content</footer>
	<script>console.log('test');</script>
</body>
</html>`
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	}))
	defer ts.Close()

	scraper := NewTestStaticScraper()

	opts := models.RequestOptions{
		URL:      ts.URL,
		Selector: "body",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := scraper.Fetch(opts)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkStaticScraperParallel measures concurrent static scraping
func BenchmarkStaticScraperParallel(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `<!DOCTYPE html><html><body><h1>Test</h1><p>Content</p></body></html>`
		w.Write([]byte(html))
	}))
	defer ts.Close()

	scraper := NewTestStaticScraper()

	opts := models.RequestOptions{
		URL:      ts.URL,
		Selector: "body",
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := scraper.Fetch(opts)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkBatchScraper measures batch scraping performance
func BenchmarkBatchScraper(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `<!DOCTYPE html><html><body><h1>Test</h1><p>Content</p></body></html>`
		w.Write([]byte(html))
	}))
	defer ts.Close()

	scraper := NewTestStaticScraper()
	batchScraper := NewBatchScraper(scraper, 5)

	// Create batch requests
	requests := make([]models.RequestOptions, 10)
	for i := 0; i < 10; i++ {
		requests[i] = models.RequestOptions{
			URL:      ts.URL,
			Selector: "body",
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		results := batchScraper.ScrapeBatch(ctx, requests)
		for range results {
			// Consume results
		}
	}
}

// BenchmarkDynamicScraper measures dynamic scraping performance
func BenchmarkDynamicScraper(b *testing.B) {
	b.Skip("Skipping Chrome-based benchmark in CI/test environment")
}

// BenchmarkMemoryAllocation measures memory allocation patterns
func BenchmarkMemoryAllocation(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a larger HTML document to measure real allocation
		html := `<!DOCTYPE html><html><body>`
		for i := 0; i < 100; i++ {
			html += `<div class="item"><h2>Title ` + string(rune(i)) + `</h2><p>Content paragraph with text.</p></div>`
		}
		html += `</body></html>`
		w.Write([]byte(html))
	}))
	defer ts.Close()

	scraper := NewTestStaticScraper()

	opts := models.RequestOptions{
		URL:      ts.URL,
		Selector: "body",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		data, err := scraper.Fetch(opts)
		if err != nil {
			b.Fatal(err)
		}
		_ = data.Content // Use the data to prevent optimization
	}
}
