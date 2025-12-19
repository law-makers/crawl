package models

import "time"

// PageData represents the scraped data from a web page
type PageData struct {
	URL          string            `json:"url"`
	StatusCode   int               `json:"status_code"`
	Title        string            `json:"title,omitempty"`
	Content      string            `json:"content,omitempty"`
	HTML         string            `json:"html,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Links        []string          `json:"links,omitempty"`
	Images       []string          `json:"images,omitempty"`
	Scripts      []string          `json:"scripts,omitempty"`
	FetchedAt    time.Time         `json:"fetched_at"`
	ResponseTime int64             `json:"response_time_ms"`
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
	Headers     map[string]string
	SessionName string
	Timeout     time.Duration
	Proxy       string
}
