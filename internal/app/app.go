// Package app provides the core application initialization and lifecycle management.
package app

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/law-makers/crawl/internal/cache"
	"github.com/law-makers/crawl/internal/config"
	"github.com/law-makers/crawl/internal/engine"
	"github.com/law-makers/crawl/internal/engine/dynamic"
	"github.com/law-makers/crawl/internal/engine/hybrid"
	"github.com/law-makers/crawl/internal/engine/static"
	"github.com/law-makers/crawl/internal/ratelimit"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Application holds all application dependencies and manages their lifecycle.
//
// It is created once at startup and shared across all CLI commands.
// Use Close() to ensure proper resource cleanup on shutdown.
type Application struct {
	Config         *config.Config
	Logger         *zerolog.Logger
	Cache          cache.Cache
	BrowserPool    *dynamic.BrowserPool
	poolMu         sync.Mutex
	RateLimiter    ratelimit.RateLimiter
	HTTPClient     *http.Client
	StaticScraper  *static.Scraper
	DynamicScraper *dynamic.Scraper
	Scraper        engine.Scraper
	startTime      time.Time
}

// New creates and initializes a new Application with all dependencies.
//
// It performs the following initialization steps:
//   - Configures logging based on the provided config
//   - Creates and initializes the in-memory cache
//   - Creates and initializes the browser pool
//   - Creates the rate limiter for domain-based request throttling
//   - Initializes the HTTP client with proper timeouts
//   - Creates the hybrid scraper with all dependencies
//
// If any step fails, an error is returned and no resources are allocated.
func New(ctx context.Context, cfg *config.Config) (*Application, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	// Initialize logger based on config
	logLevel := zerolog.ErrorLevel // default: suppress non-verbose info logs
	switch cfg.LogLevel {
	case "debug":
		logLevel = zerolog.DebugLevel
	case "warn":
		logLevel = zerolog.WarnLevel
	case "error":
		logLevel = zerolog.ErrorLevel
	// Treat "info" as non-verbose (don't display info logs unless -v is used)
	default:
		logLevel = zerolog.ErrorLevel
	}
	zerolog.SetGlobalLevel(logLevel)

	var logWriter io.Writer
	if cfg.JSONLog {
		// JSON logs to stderr
		logWriter = os.Stderr
	} else {
		// Human-friendly console output otherwise
		logWriter = zerolog.NewConsoleWriter()
	}

	logger := log.Output(logWriter).With().Timestamp().Logger()

	logger.Debug().
		Str("level", cfg.LogLevel).
		Bool("json", cfg.JSONLog).
		Msg("Logger initialized")

	// Create cache
	memCache := cache.NewMemoryCache(cfg.CacheMaxSizeBytes)
	logger.Debug().
		Int64("max_size_bytes", cfg.CacheMaxSizeBytes).
		Msg("Memory cache initialized")

	// Browser pool initialization is now lazy (only created when SPA/dynamic scraping is requested).
	// We still keep the field on Application for later creation via EnsureBrowserPool.
	browserPool := (*dynamic.BrowserPool)(nil)

	// Create rate limiter
	rateLimiter := ratelimit.NewDomainLimiter(cfg.StaticRateLimitRPS, cfg.StaticRateLimitBurst)
	logger.Debug().
		Float64("static_rps", cfg.StaticRateLimitRPS).
		Int("static_burst", cfg.StaticRateLimitBurst).
		Msg("Rate limiter initialized")

	// Create HTTP client
	httpClient := &http.Client{
		Timeout: cfg.HTTPTimeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			DisableKeepAlives:   false,
		},
	}
	logger.Debug().
		Dur("timeout", cfg.HTTPTimeout).
		Msg("HTTP client initialized")

	// Create scrapers
	staticScraper := static.New(
		memCache,
		rateLimiter,
		httpClient,
		cfg.HTTPTimeout,
		cfg.UserAgent,
	)

	// Create dynamic scraper without an active pool. The pool will be created lazily
	// when SPA mode is actually requested to avoid starting browsers unnecessarily.
	dynamicScraper := dynamic.New(
		memCache,
		rateLimiter,
		nil, // pool created on demand
		cfg.HTTPTimeout,
		cfg.UserAgent,
	)

	hybridScraper := hybrid.New(staticScraper, dynamicScraper)
	logger.Debug().Msg("Scrapers initialized")

	app := &Application{
		Config:         cfg,
		Logger:         &logger,
		Cache:          memCache,
		BrowserPool:    browserPool,
		RateLimiter:    rateLimiter,
		HTTPClient:     httpClient,
		StaticScraper:  staticScraper,
		DynamicScraper: dynamicScraper,
		Scraper:        hybridScraper,
		startTime:      time.Now(),
	}

	logger.Info().Msg("Application initialized successfully")
	return app, nil
}

// EnsureBrowserPool lazily creates the browser pool if it has not already been
// initialized. Callers should provide a context with an appropriate timeout.
func (a *Application) EnsureBrowserPool(ctx context.Context) error {
	if a == nil {
		return fmt.Errorf("application is nil")
	}

	a.poolMu.Lock()
	defer a.poolMu.Unlock()

	if a.BrowserPool != nil {
		return nil
	}

	logger := a.Logger
	logger.Debug().Msg("Initializing browser pool on demand")
	pool, err := dynamic.NewBrowserPool(dynamic.BrowserPoolOptions{
		Size:      a.Config.BrowserPoolSize,
		Headless:  a.Config.BrowserHeadless,
		UserAgent: a.Config.UserAgent,
		Proxy:     a.Config.Proxy,
	})
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to create browser pool on demand")
		return err
	}

	a.BrowserPool = pool
	// Attach to dynamic scraper so it can reuse contexts
	if a.DynamicScraper != nil {
		a.DynamicScraper.SetBrowserPool(pool)
	}

	logger.Info().Int("pool_size", pool.Size()).Msg("Browser pool initialized on demand")
	return nil
}

// Close gracefully shuts down the application and all its resources.
//
// It performs the following cleanup steps in order:
//   - Stops accepting new requests
//   - Waits briefly for in-flight requests to complete
//   - Closes the browser pool
//   - Closes the cache
//   - Closes the HTTP client
//
// A context with a timeout should be provided to prevent indefinite blocking.
// Any errors during shutdown are logged but do not prevent other shutdown steps.
func (a *Application) Close(ctx context.Context) error {
	a.Logger.Info().Msg("Shutting down application")

	// Close browser pool (will interrupt any running operations)
	if a.BrowserPool != nil {
		if err := a.BrowserPool.Close(); err != nil {
			a.Logger.Warn().Err(err).Msg("Error closing browser pool")
		}
	}

	// Close cache
	if a.Cache != nil {
		a.Cache.Close()
	}

	// Close HTTP client (connection pooling cleanup)
	if a.HTTPClient != nil {
		a.HTTPClient.CloseIdleConnections()
	}

	uptime := time.Since(a.startTime)
	a.Logger.Info().Dur("uptime", uptime).Msg("Application shutdown complete")
	return nil
}

// Uptime returns how long the application has been running.
func (a *Application) Uptime() time.Duration {
	return time.Since(a.startTime)
}
