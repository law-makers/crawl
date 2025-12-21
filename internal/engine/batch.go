package engine

import (
	"context"
	"net/url"
	"runtime"
	"sync"

	"github.com/law-makers/crawl/pkg/models"
)

// OptimalConcurrency calculates optimal concurrency based on CPU and memory
func OptimalConcurrency() int {
	numCPU := runtime.NumCPU()

	// For I/O bound operations (scraping), use 2-4x CPU count
	optimal := numCPU * 3

	// Cap based on available memory
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	availMB := (m.Sys - m.Alloc) / 1024 / 1024

	// Assume ~50MB per browser context for dynamic scraping
	maxByMemory := int(availMB / 50)

	// Don't go below CPU count or above 50
	if optimal < numCPU {
		optimal = numCPU
	}
	if optimal > 50 {
		optimal = 50
	}

	if maxByMemory > 0 && maxByMemory < optimal {
		return maxByMemory
	}
	return optimal
}

// BatchScraper wraps a Scraper to provide concurrent batch processing
type BatchScraper struct {
	scraper     Scraper
	concurrency int
}

// NewBatchScraper creates a new BatchScraper
// If concurrency <= 0, it auto-tunes based on system resources
func NewBatchScraper(scraper Scraper, concurrency int) *BatchScraper {
	if concurrency <= 0 {
		concurrency = OptimalConcurrency()
	}
	return &BatchScraper{
		scraper:     scraper,
		concurrency: concurrency,
	}
}

// groupByDomain groups requests by their domain for better HTTP/2 multiplexing
func groupByDomain(requests []models.RequestOptions) map[string][]models.RequestOptions {
	groups := make(map[string][]models.RequestOptions)

	for _, req := range requests {
		u, err := url.Parse(req.URL)
		if err != nil {
			// If URL parsing fails, use a default group
			groups["default"] = append(groups["default"], req)
			continue
		}

		domain := u.Host
		groups[domain] = append(groups[domain], req)
	}

	return groups
}

// ScrapeBatch processes a list of requests concurrently
// Requests are grouped by domain to leverage HTTP/2 multiplexing
func (s *BatchScraper) ScrapeBatch(ctx context.Context, requests []models.RequestOptions) <-chan models.ScrapeResult {
	results := make(chan models.ScrapeResult, len(requests))

	// Group requests by domain for better HTTP/2 performance
	domainGroups := groupByDomain(requests)

	var wg sync.WaitGroup

	go func() {
		// Process each domain group
		for domain, groupRequests := range domainGroups {
			select {
			case <-ctx.Done():
				wg.Wait()
				close(results)
				return
			default:
			}

			// Process requests within the same domain with limited concurrency
			sem := make(chan struct{}, s.concurrency)

			for _, req := range groupRequests {
				wg.Add(1)
				sem <- struct{}{} // Acquire semaphore

				go func(r models.RequestOptions, d string) {
					defer wg.Done()
					defer func() { <-sem }() // Release semaphore

					data, err := s.scraper.Fetch(r)
					results <- models.ScrapeResult{
						Data:  data,
						Error: err,
					}
				}(req, domain)
			}
		}

		wg.Wait()
		close(results)
	}()

	return results
}
