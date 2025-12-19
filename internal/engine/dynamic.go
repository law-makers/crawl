// internal/engine/dynamic.go
package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/law-makers/crawl/internal/auth"
	"github.com/law-makers/crawl/pkg/models"
	"github.com/rs/zerolog/log"
)

// DynamicScraper implements the Scraper interface using headless Chrome
// It uses chromedp to render JavaScript and handle SPAs (React/Vue/Angular)
type DynamicScraper struct {
	// headless controls whether to show browser window
	headless bool
	// userAgent to use for requests
	userAgent string
}

// NewDynamicScraper creates a new DynamicScraper with default settings
func NewDynamicScraper() *DynamicScraper {
	return &DynamicScraper{
		headless:  true,
		userAgent: "Crawl/1.0 (https://github.com/law-makers/crawl)",
	}
}

// NewDynamicScraperVisible creates a DynamicScraper with visible browser (for debugging/auth)
func NewDynamicScraperVisible() *DynamicScraper {
	return &DynamicScraper{
		headless:  false,
		userAgent: "Crawl/1.0 (https://github.com/law-makers/crawl)",
	}
}

// Name returns the name of this scraper
func (d *DynamicScraper) Name() string {
	return "DynamicScraper"
}

// Fetch retrieves and parses a page using headless Chrome
func (d *DynamicScraper) Fetch(opts models.RequestOptions) (*models.PageData, error) {
	start := time.Now()

	log.Debug().
		Str("url", opts.URL).
		Str("scraper", d.Name()).
		Bool("headless", d.headless).
		Msg("Starting fetch")

	// Set timeout - use a reasonable timeout
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	log.Debug().Dur("elapsed_ms", time.Since(start)).Msg("Context created")

	// Allocator options - optimized for speed with fast shutdown
	allocOpts := []chromedp.ExecAllocatorOption{
		chromedp.ExecPath("/usr/bin/google-chrome-stable"),
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("headless", d.headless),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-breakpad", true),
		chromedp.Flag("disable-client-side-phishing-detection", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("disable-hang-monitor", true),
		chromedp.Flag("disable-ipc-flooding-protection", true),
		chromedp.Flag("disable-prompt-on-repost", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("disable-translate", true),
		chromedp.Flag("force-color-profile", "srgb"),
		chromedp.Flag("metrics-recording-only", true),
		chromedp.Flag("mute-audio", true),
		chromedp.Flag("safebrowsing-disable-auto-update", true),
		chromedp.Flag("single-process", true), // Critical for fast shutdown
		chromedp.UserAgent(d.userAgent),
	}

	// Add proxy if specified
	if opts.Proxy != "" {
		allocOpts = append(allocOpts, chromedp.ProxyServer(opts.Proxy))
	}

	// Create allocator context
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, allocOpts...)
	defer allocCancel()

	log.Debug().Dur("elapsed_ms", time.Since(start)).Msg("Allocator created")

	// Create browser context
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()

	log.Debug().Dur("elapsed_ms", time.Since(start)).Msg("Browser context created")

	// Load session if specified and inject cookies before navigation
	var sessionCookies []*network.CookieParam
	if opts.SessionName != "" {
		log.Debug().Str("session", opts.SessionName).Msg("Loading session")
		session, err := auth.LoadSession(opts.SessionName)
		if err != nil {
			log.Warn().Err(err).Str("session", opts.SessionName).Msg("Failed to load session")
		} else {
			// Convert auth cookies to chromedp cookie format
			for _, c := range session.Cookies {
				cookie := &network.CookieParam{
					Name:     c.Name,
					Value:    c.Value,
					Domain:   c.Domain,
					Path:     c.Path,
					HTTPOnly: c.HTTPOnly,
					Secure:   c.Secure,
				}
				if c.Expires > 0 {
					// Convert Unix timestamp to cdp.TimeSinceEpoch
					t := time.Unix(int64(c.Expires), 0)
					expires := cdp.TimeSinceEpoch(t)
					cookie.Expires = &expires
				}
				switch c.SameSite {
				case "Strict":
					cookie.SameSite = network.CookieSameSiteStrict
				case "Lax":
					cookie.SameSite = network.CookieSameSiteLax
				case "None":
					cookie.SameSite = network.CookieSameSiteNone
				}
				sessionCookies = append(sessionCookies, cookie)
			}
			log.Debug().Int("cookies", len(sessionCookies)).Msg("Session cookies prepared")
		}
	}

	// Build PageData
	pageData := &models.PageData{
		URL:       opts.URL,
		FetchedAt: time.Now(),
		Headers:   make(map[string]string),
		Metadata:  make(map[string]string),
		Links:     []string{},
		Images:    []string{},
		Scripts:   []string{},
	}

	// Variables to capture
	var htmlContent string
	var title string
	var statusCode int64

	navigateStart := time.Now()
	log.Debug().Msg("Starting chromedp.Run")

	// Listen for network events to capture status code and headers
	chromedp.ListenTarget(browserCtx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *network.EventResponseReceived:
			resp := ev.Response
			if resp.URL == opts.URL {
				statusCode = resp.Status
				// Capture headers
				for key, value := range resp.Headers {
					if strValue, ok := value.(string); ok {
						pageData.Headers[key] = strValue
					}
				}
			}
		}
	})

	// Prepare selector to wait for (if specified)
	selector := opts.Selector
	if selector == "" || selector == "body" {
		selector = "body"
	}

	// Build task list
	tasks := []chromedp.Action{network.Enable()}

	// Inject session cookies before navigation
	if len(sessionCookies) > 0 {
		tasks = append(tasks, network.SetCookies(sessionCookies))
	}

	// Execute navigation and content extraction
	tasks = append(tasks,
		chromedp.Navigate(opts.URL),
		// Just wait for DOM to be ready, not for specific selectors
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Small sleep to let initial JS execute
			time.Sleep(300 * time.Millisecond)
			return nil
		}),
		chromedp.Title(&title),
		chromedp.OuterHTML("html", &htmlContent, chromedp.ByQuery),
	)

	// Execute tasks with fast rendering - no blocking waits
	err := chromedp.Run(browserCtx, tasks...)

	log.Debug().Dur("elapsed_ms", time.Since(navigateStart)).Msg("chromedp.Run completed")

	if err != nil {
		return nil, fmt.Errorf("chromedp execution failed: %w", err)
	}

	responseTime := time.Since(start).Milliseconds()

	// Update page data
	pageData.Title = title
	pageData.HTML = htmlContent
	pageData.StatusCode = int(statusCode)
	pageData.ResponseTime = responseTime

	// Parse HTML to extract additional data
	err = d.extractDataFromHTML(browserCtx, opts, pageData)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to extract additional data")
	}

	log.Info().
		Str("url", opts.URL).
		Int("status", pageData.StatusCode).
		Int64("response_time_ms", responseTime).
		Int("links", len(pageData.Links)).
		Int("images", len(pageData.Images)).
		Msg("Fetch completed")

	return pageData, nil
}

// extractDataFromHTML extracts links, images, scripts, and content from the page
func (d *DynamicScraper) extractDataFromHTML(ctx context.Context, opts models.RequestOptions, pageData *models.PageData) error {
	// Extract content based on selector
	selector := opts.Selector
	if selector != "" && selector != "body" {
		var content string
		var html string
		err := chromedp.Run(ctx,
			chromedp.Text(selector, &content, chromedp.ByQuery),
			chromedp.OuterHTML(selector, &html, chromedp.ByQuery),
		)
		if err == nil {
			pageData.Content = strings.TrimSpace(content)
		} else {
			log.Warn().Str("selector", selector).Msg("Selector not found")
		}
	} else {
		// Extract body text
		var bodyText string
		err := chromedp.Run(ctx, chromedp.Text("body", &bodyText, chromedp.ByQuery))
		if err == nil {
			pageData.Content = strings.TrimSpace(bodyText)
		}
	}

	// Extract links
	var links []*cdp.Node
	err := chromedp.Run(ctx, chromedp.Nodes("a[href]", &links, chromedp.ByQueryAll))
	if err == nil {
		for _, node := range links {
			if href, ok := node.Attribute("href"); ok && href != "" {
				pageData.Links = append(pageData.Links, href)
			}
		}
	}

	// Extract images
	var images []*cdp.Node
	err = chromedp.Run(ctx, chromedp.Nodes("img[src]", &images, chromedp.ByQueryAll))
	if err == nil {
		for _, node := range images {
			if src, ok := node.Attribute("src"); ok && src != "" {
				pageData.Images = append(pageData.Images, src)
			}
		}
	}

	// Extract scripts
	var scripts []*cdp.Node
	err = chromedp.Run(ctx, chromedp.Nodes("script[src]", &scripts, chromedp.ByQueryAll))
	if err == nil {
		for _, node := range scripts {
			if src, ok := node.Attribute("src"); ok && src != "" {
				pageData.Scripts = append(pageData.Scripts, src)
			}
		}
	}

	// Extract metadata
	var metaTags []*cdp.Node
	err = chromedp.Run(ctx, chromedp.Nodes("meta", &metaTags, chromedp.ByQueryAll))
	if err == nil {
		for _, node := range metaTags {
			if name, ok := node.Attribute("name"); ok {
				if content, ok := node.Attribute("content"); ok {
					pageData.Metadata[name] = content
				}
			}
			if property, ok := node.Attribute("property"); ok {
				if content, ok := node.Attribute("content"); ok {
					pageData.Metadata[property] = content
				}
			}
		}
	}

	return nil
}
