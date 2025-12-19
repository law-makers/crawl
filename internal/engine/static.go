// internal/engine/static.go
package engine

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/law-makers/crawl/internal/auth"
	"github.com/law-makers/crawl/pkg/models"
	"github.com/rs/zerolog/log"
)

// StaticScraper implements the Scraper interface for static HTML pages
// It uses raw HTTP requests and goquery for parsing - extremely fast
type StaticScraper struct {
	client *http.Client
}

// NewStaticScraper creates a new StaticScraper with optimized HTTP client
func NewStaticScraper() *StaticScraper {
	// Configure HTTP client with Keep-Alive for connection reuse
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	return &StaticScraper{
		client: client,
	}
}

// Name returns the name of this scraper
func (s *StaticScraper) Name() string {
	return "StaticScraper"
}

// Fetch retrieves and parses a static HTML page
func (s *StaticScraper) Fetch(opts models.RequestOptions) (*models.PageData, error) {
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
		return nil, fmt.Errorf("failed to create request: %w", err)
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
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	// Parse HTML with goquery
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
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

	// Extract title
	pageData.Title = doc.Find("title").First().Text()

	// Extract content based on selector
	if opts.Selector != "" && opts.Selector != "body" {
		// Extract specific selector
		selection := doc.Find(opts.Selector)
		if selection.Length() > 0 {
			pageData.Content = strings.TrimSpace(selection.Text())
			pageData.HTML, _ = selection.Html()
		} else {
			log.Warn().
				Str("selector", opts.Selector).
				Msg("Selector not found in document")
		}
	} else {
		// Extract body content
		pageData.Content = strings.TrimSpace(doc.Find("body").Text())
		pageData.HTML, _ = doc.Find("html").Html()
	}

	// Extract metadata
	doc.Find("meta").Each(func(i int, sel *goquery.Selection) {
		if name, exists := sel.Attr("name"); exists {
			content, _ := sel.Attr("content")
			pageData.Metadata[name] = content
		}
		if property, exists := sel.Attr("property"); exists {
			content, _ := sel.Attr("content")
			pageData.Metadata[property] = content
		}
	})

	// Extract links
	doc.Find("a[href]").Each(func(i int, sel *goquery.Selection) {
		if href, exists := sel.Attr("href"); exists && href != "" {
			pageData.Links = append(pageData.Links, href)
		}
	})

	// Extract images
	doc.Find("img[src]").Each(func(i int, sel *goquery.Selection) {
		if src, exists := sel.Attr("src"); exists && src != "" {
			pageData.Images = append(pageData.Images, src)
		}
	})

	// Extract scripts
	doc.Find("script[src]").Each(func(i int, sel *goquery.Selection) {
		if src, exists := sel.Attr("src"); exists && src != "" {
			pageData.Scripts = append(pageData.Scripts, src)
		}
	})

	log.Debug().
		Str("url", opts.URL).
		Int("status", resp.StatusCode).
		Int64("response_time_ms", responseTime).
		Int("links", len(pageData.Links)).
		Int("images", len(pageData.Images)).
		Msg("Fetch completed")

	return pageData, nil
}
