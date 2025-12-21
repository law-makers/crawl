// internal/engine/test_helpers.go
package engine

import (
	"net/http"
	"time"

	"github.com/law-makers/crawl/internal/cache"
	"github.com/law-makers/crawl/internal/ratelimit"
)

// NewTestStaticScraper creates a StaticScraper for testing with default dependencies
func NewTestStaticScraper() *StaticScraper {
	return NewStaticScraper(
		cache.NewMemoryCache(100*1024*1024),
		ratelimit.NewDomainLimiter(5.0, 10),
		&http.Client{Timeout: 30 * time.Second},
		30*time.Second,
		"TestScraper/1.0",
	)
}

// NewTestDynamicScraper creates a DynamicScraper for testing with default dependencies
func NewTestDynamicScraper() *DynamicScraper {
	return NewDynamicScraper(
		cache.NewMemoryCache(100*1024*1024),
		ratelimit.NewDomainLimiter(3.0, 5),
		nil, // No browser pool for unit tests
		&http.Client{Timeout: 30 * time.Second},
		30*time.Second,
		"TestScraper/1.0",
	)
}
