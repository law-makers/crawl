// internal/engine/dynamic/scraper.go
package dynamic

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/law-makers/crawl/internal/cache"
	"github.com/law-makers/crawl/internal/ratelimit"
	"github.com/law-makers/crawl/pkg/models"
	"github.com/rs/zerolog/log"
)

// Scraper implements the Scraper interface using headless Chrome
// It uses chromedp to render JavaScript and handle SPAs (React/Vue/Angular)
type Scraper struct {
	cache       cache.Cache
	limiter     ratelimit.RateLimiter
	browserPool *BrowserPool
	client      interface{} // Keep for compatibility
	timeout     time.Duration
	userAgent   string
	mu          sync.Mutex
}

// New creates a new DynamicScraper with dependency injection
func New(c cache.Cache, lim ratelimit.RateLimiter, pool *BrowserPool, timeout time.Duration, ua string) *Scraper {
	return &Scraper{
		cache:       c,
		limiter:     lim,
		browserPool: pool,
		timeout:     timeout,
		userAgent:   ua,
	}
}

// SetBrowserPool updates the browser pool used by the scraper (thread-safe)
func (d *Scraper) SetBrowserPool(bp *BrowserPool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.browserPool = bp
}

// Name returns the name of this scraper
func (d *Scraper) Name() string {
	return "DynamicScraper"
}

// Fetch retrieves and parses a page using headless Chrome
func (d *Scraper) Fetch(opts models.RequestOptions) (*models.PageData, error) {
	start := time.Now()

	log.Debug().
		Str("url", opts.URL).
		Str("scraper", d.Name()).
		Msg("Starting fetch")

	// Set timeout - use a reasonable timeout
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	var ctx context.Context
	var cancel context.CancelFunc

	// 1. Try to use browser pool (faster and more stable)
	if d.browserPool != nil {
		bCtx, err := d.browserPool.Acquire(timeout)
		if err != nil {
			return nil, fmt.Errorf("failed to acquire browser from pool: %w", err)
		}
		// Release back to pool when function exits
		defer d.browserPool.Release(bCtx)

		// Create timeout context for this specific request
		ctx, cancel = context.WithTimeout(bCtx.Ctx, timeout)
		defer cancel()

		log.Debug().Dur("elapsed_ms", time.Since(start)).Msg("Acquired browser from pool")
	} else {
		// 2. Fallback: Create new allocator and context (slower)
		// We mirror the robust flags from browser_pool.go here to ensure stability on Windows

		// Create base context with timeout
		var baseCancel context.CancelFunc
		ctx, baseCancel = context.WithTimeout(context.Background(), timeout)
		defer baseCancel()

		chromePath := FindChrome()
		allocOpts := []chromedp.ExecAllocatorOption{
			chromedp.NoFirstRun,
			chromedp.NoDefaultBrowserCheck,
			chromedp.Flag("headless", "new"),
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
			// Robustness flags (critical for Windows stability)
			chromedp.Flag("disable-features", "site-per-process,TranslateUI,BlinkGenPropertyTrees"),
			chromedp.Flag("enable-features", "NetworkService,NetworkServiceInProcess"),
			chromedp.Flag("disable-blink-features", "AutomationControlled"),
			chromedp.Flag("disable-infobars", true),
			chromedp.Flag("window-size", "1920,1080"),
			chromedp.Flag("disk-cache-size", "0"),
			chromedp.Flag("media-cache-size", "0"),
			chromedp.UserAgent(d.userAgent),
		}

		// Set chrome path if found
		if chromePath != "" {
			allocOpts = append([]chromedp.ExecAllocatorOption{chromedp.ExecPath(chromePath)}, allocOpts...)
		}

		// Add proxy if specified
		if opts.Proxy != "" {
			allocOpts = append(allocOpts, chromedp.ProxyServer(opts.Proxy))
		}

		// Create allocator context
		var allocCancel context.CancelFunc
		ctx, allocCancel = chromedp.NewExecAllocator(ctx, allocOpts...)
		// We defer allocCancel in a way that it runs when the function returns
		defer allocCancel()

		// Create browser context
		ctx, cancel = chromedp.NewContext(ctx)
		defer cancel()

		log.Debug().Dur("elapsed_ms", time.Since(start)).Msg("Created new browser context (fallback)")
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
	chromedp.ListenTarget(ctx, func(ev interface{}) {
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

	// Execute navigation and content extraction
	tasks = append(tasks,
		chromedp.Navigate(opts.URL),
		// Wait a short initial period and any user-specified wait (opts.WaitSeconds)
		chromedp.ActionFunc(func(ctx context.Context) error {
			// Small sleep to let initial JS execute
			time.Sleep(300 * time.Millisecond)
			if opts.WaitSeconds > 0 {
				log.Debug().Int("wait_seconds", opts.WaitSeconds).Msg("Waiting after navigation before scraping (dynamic)")
				time.Sleep(time.Duration(opts.WaitSeconds) * time.Second)
			}
			return nil
		}),
		chromedp.Title(&title),
		chromedp.OuterHTML("html", &htmlContent, chromedp.ByQuery),
	)

	// Execute tasks with fast rendering - no blocking waits
	err := chromedp.Run(ctx, tasks...)

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
	err = extractDataFromHTML(ctx, opts, pageData)
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
