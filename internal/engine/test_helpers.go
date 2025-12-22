// internal/engine/test_helpers.go
package engine

import (
	"net/http"
	"time"

	"github.com/law-makers/crawl/internal/cache"
	"github.com/law-makers/crawl/internal/engine/dynamic"
	"github.com/law-makers/crawl/internal/engine/static"
	"github.com/law-makers/crawl/internal/ratelimit"
)

// NewTestStaticScraper creates a StaticScraper for testing with default dependencies
func NewTestStaticScraper() *static.Scraper {
	return static.New(
		cache.NewMemoryCache(100*1024*1024),
		ratelimit.NewDomainLimiter(5.0, 10),
		&http.Client{Timeout: 30 * time.Second},
		30*time.Second,
		"TestScraper/1.0",
	)
}

// NewTestDynamicScraper creates a DynamicScraper for testing with default dependencies
func NewTestDynamicScraper() *dynamic.Scraper {
	return dynamic.New(
		cache.NewMemoryCache(100*1024*1024),
		ratelimit.NewDomainLimiter(3.0, 5),
		nil, // No browser pool for unit tests
		30*time.Second,
		"TestScraper/1.0",
	)
}
