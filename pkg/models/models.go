package models

import "time"

// SelectionData represents a single item extracted from a list
type SelectionData struct {
	Text string `json:"text"`
	HTML string `json:"html"`
}

// PageData represents the scraped data from a web page.
//
// It contains the raw HTML, extracted content, metadata, and resource URLs
// discovered during the scraping operation.
type PageData struct {
	URL          string              `json:"url"`                  // The URL that was scraped
	StatusCode   int                 `json:"status_code"`          // HTTP status code (e.g., 200, 404)
	Title        string              `json:"title,omitempty"`      // Page title from <title> tag
	Content      string              `json:"content,omitempty"`    // Extracted text content based on selector
	HTML         string              `json:"html,omitempty"`       // Raw HTML of the page or selected element
	Data         []SelectionData     `json:"data,omitempty"`       // Multiple extracted items (for lists)
	Structured   []map[string]string `json:"structured,omitempty"` // Structured data extracted with field mapping
	Headers      map[string]string   `json:"headers,omitempty"`    // HTTP response headers
	Metadata     map[string]string   `json:"metadata,omitempty"`   // Page metadata (description, keywords, etc.)
	Links        []string            `json:"links,omitempty"`      // All links found on the page
	Images       []string            `json:"images,omitempty"`     // All image URLs found on the page
	Scripts      []string            `json:"scripts,omitempty"`    // All script URLs found on the page
	FetchedAt    time.Time           `json:"fetched_at"`           // Timestamp when the page was fetched
	ResponseTime int64               `json:"response_time_ms"`     // Time taken to fetch and parse (milliseconds)
}

// ScrapeResult represents the result of a scraping operation
type ScrapeResult struct {
	Data  *PageData
	Error error
}

// ScraperMode defines the engine mode to use
type ScraperMode string

const (
	ModeAuto   ScraperMode = "auto"
	ModeStatic ScraperMode = "static"
	ModeSPA    ScraperMode = "spa"
)

// RequestOptions contains options for making scraping requests
type RequestOptions struct {
	URL         string
	Mode        ScraperMode
	Selector    string
	Fields      map[string]string
	Headers     map[string]string
	SessionName string
	Timeout     time.Duration
	Proxy       string
	WaitSeconds int // Number of seconds to wait after browser opens before scraping
}
