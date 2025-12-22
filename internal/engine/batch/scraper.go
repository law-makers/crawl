// internal/engine/batch/scraper.go
package batch

import (
	"context"
	"sync"

	"github.com/law-makers/crawl/pkg/models"
)

// Scraper interface defines what a scraper must implement
type ScraperInterface interface {
	Fetch(opts models.RequestOptions) (*models.PageData, error)
}

// Scraper wraps a ScraperInterface to provide concurrent batch processing
type Scraper struct {
	scraper     ScraperInterface
	concurrency int
}

// New creates a new BatchScraper
// If concurrency <= 0, it auto-tunes based on system resources
func New(scraper ScraperInterface, concurrency int) *Scraper {
	if concurrency <= 0 {
		concurrency = OptimalConcurrency()
	}
	return &Scraper{
		scraper:     scraper,
		concurrency: concurrency,
	}
}

// ScrapeBatch processes a list of requests concurrently
// Requests are grouped by domain to leverage HTTP/2 multiplexing
func (s *Scraper) ScrapeBatch(ctx context.Context, requests []models.RequestOptions) <-chan models.ScrapeResult {
	results := make(chan models.ScrapeResult, len(requests))

	// Group requests by domain for better HTTP/2 performance
	domainGroups := GroupByDomain(requests)

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
