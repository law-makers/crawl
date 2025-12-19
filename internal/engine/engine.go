package engine

import "github.com/law-makers/crawl/pkg/models"

// Scraper is the interface that all scraping engines must implement
type Scraper interface {
	// Fetch retrieves and parses data from the given URL
	Fetch(opts models.RequestOptions) (*models.PageData, error)

	// Name returns the name of the scraper implementation
	Name() string
}
