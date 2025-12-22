// internal/engine/static/scraper.go
package static

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/law-makers/crawl/internal/auth"
	"github.com/law-makers/crawl/internal/cache"
	"github.com/law-makers/crawl/internal/engine/metadata"
	"github.com/law-makers/crawl/internal/ratelimit"
	"github.com/law-makers/crawl/pkg/models"
	"github.com/rs/zerolog/log"
)

// Scraper implements the Scraper interface for static HTML pages
// It uses raw HTTP requests and goquery for parsing - extremely fast
type Scraper struct {
	cache     cache.Cache
	limiter   ratelimit.RateLimiter
	client    *http.Client
	timeout   time.Duration
	userAgent string
}

// New creates a new StaticScraper with dependency injection
func New(c cache.Cache, lim ratelimit.RateLimiter, client *http.Client, timeout time.Duration, ua string) *Scraper {
	return &Scraper{
		cache:     c,
		limiter:   lim,
		client:    client,
		timeout:   timeout,
		userAgent: ua,
	}
}

// Name returns the name of this scraper
func (s *Scraper) Name() string {
	return "StaticScraper"
}

// FetchWithDoc retrieves and parses a static HTML page, returning both data and document
func (s *Scraper) FetchWithDoc(opts models.RequestOptions) (*models.PageData, *goquery.Document, error) {
	return s.fetch(opts)
}

// Fetch retrieves and parses a static HTML page
func (s *Scraper) Fetch(opts models.RequestOptions) (*models.PageData, error) {
	data, _, err := s.fetch(opts)
	return data, err
}

func (s *Scraper) fetch(opts models.RequestOptions) (*models.PageData, *goquery.Document, error) {
	start := time.Now()

	log.Debug().
		Str("url", opts.URL).
		Str("scraper", s.Name()).
		Msg("Starting fetch")

	// Load session if specified
	if opts.SessionName != "" {
		log.Debug().Str("session", opts.SessionName).Msg("Loading session")
		session, err := auth.LoadSession(opts.SessionName)
		if err != nil {
			log.Warn().Err(err).Str("session", opts.SessionName).Msg("Failed to load session")
		} else {
			// Inject cookies into client
			jar, err := cookiejar.New(nil)
			if err == nil {
				parsedURL, _ := url.Parse(opts.URL)
				var cookies []*http.Cookie
				for _, c := range session.Cookies {
					cookies = append(cookies, &http.Cookie{
						Name:     c.Name,
						Value:    c.Value,
						Domain:   c.Domain,
						Path:     c.Path,
						Expires:  time.Unix(int64(c.Expires), 0),
						HttpOnly: c.HTTPOnly,
						Secure:   c.Secure,
					})
				}
				jar.SetCookies(parsedURL, cookies)
				s.client.Jar = jar
				log.Debug().Int("cookies", len(cookies)).Msg("Session cookies injected")
			}

			// Add session headers
			if len(session.Headers) > 0 {
				if opts.Headers == nil {
					opts.Headers = make(map[string]string)
				}
				for key, value := range session.Headers {
					opts.Headers[key] = value
				}
			}
		}
	}

	// Create request
	req, err := http.NewRequest("GET", opts.URL, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set default headers
	req.Header.Set("User-Agent", "Crawl/1.0 (https://github.com/law-makers/crawl)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	// Add custom headers
	for key, value := range opts.Headers {
		req.Header.Set(key, value)
	}

	// Set timeout if specified
	if opts.Timeout > 0 {
		s.client.Timeout = opts.Timeout
	}

	// Make request
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	// Parse HTML with goquery
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	responseTime := time.Since(start).Milliseconds()

	// Build PageData
	pageData := &models.PageData{
		URL:          opts.URL,
		StatusCode:   resp.StatusCode,
		FetchedAt:    time.Now(),
		ResponseTime: responseTime,
		Headers:      make(map[string]string),
		Metadata:     make(map[string]string),
	}

	// Extract headers
	for key, values := range resp.Header {
		if len(values) > 0 {
			pageData.Headers[key] = values[0]
		}
	}

	// Extract content based on selector
	pageData.Content, pageData.HTML = metadata.ExtractContent(doc, opts.Selector)

	if opts.Selector != "" && opts.Selector != "body" && pageData.Content == "" {
		log.Warn().
			Str("selector", opts.Selector).
			Msg("Selector not found in document")
	}

	// Extract metadata, links, images, scripts
	metadata.Extract(doc, pageData)

	log.Debug().
		Str("url", opts.URL).
		Int("status", resp.StatusCode).
		Int64("response_time_ms", responseTime).
		Int("links", len(pageData.Links)).
		Int("images", len(pageData.Images)).
		Msg("Fetch completed")

	return pageData, doc, nil
}
