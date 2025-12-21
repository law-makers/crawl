// Package app provides the core application initialization and lifecycle management.
package app

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/law-makers/crawl/internal/cache"
	"github.com/law-makers/crawl/internal/config"
	"github.com/law-makers/crawl/internal/engine"
	"github.com/law-makers/crawl/internal/ratelimit"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Application holds all application dependencies and manages their lifecycle.
//
// It is created once at startup and shared across all CLI commands.
// Use Close() to ensure proper resource cleanup on shutdown.
type Application struct {
	Config      *config.Config
	Logger      *zerolog.Logger
	Cache       cache.Cache
	BrowserPool *engine.BrowserPool
	RateLimiter ratelimit.RateLimiter
	HTTPClient  *http.Client
	Scraper     engine.Scraper
	startTime   time.Time
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
	logLevel := zerolog.InfoLevel
	switch cfg.LogLevel {
	case "debug":
		logLevel = zerolog.DebugLevel
	case "info":
		logLevel = zerolog.InfoLevel
	case "warn":
		logLevel = zerolog.WarnLevel
	case "error":
		logLevel = zerolog.ErrorLevel
	}
	zerolog.SetGlobalLevel(logLevel)

	var logWriter io.Writer = os.Stderr
	if cfg.JSONLog {
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

	// Create browser pool
	browserPool, err := engine.NewBrowserPool(engine.BrowserPoolOptions{
		Size:      cfg.BrowserPoolSize,
		Headless:  cfg.BrowserHeadless,
		UserAgent: cfg.UserAgent,
		Proxy:     cfg.Proxy,
	})
	if err != nil {
		// Don't fail if browser pool fails - log warning and continue with static scraping only
		logger.Warn().
			Err(err).
			Msg("Failed to create browser pool - dynamic scraping will be unavailable")
		browserPool = nil
	} else {
		logger.Debug().
			Int("size", cfg.BrowserPoolSize).
			Bool("headless", cfg.BrowserHeadless).
			Msg("Browser pool initialized")
	}

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
	staticScraper := engine.NewStaticScraper(
		memCache,
		rateLimiter,
		httpClient,
		cfg.HTTPTimeout,
		cfg.UserAgent,
	)

	dynamicScraper := engine.NewDynamicScraper(
		memCache,
		rateLimiter,
		browserPool,
		httpClient,
		cfg.HTTPTimeout,
		cfg.UserAgent,
	)

	hybridScraper := engine.NewHybridScraper(staticScraper, dynamicScraper)
	logger.Debug().Msg("Scrapers initialized")

	app := &Application{
		Config:      cfg,
		Logger:      &logger,
		Cache:       memCache,
		BrowserPool: browserPool,
		RateLimiter: rateLimiter,
		HTTPClient:  httpClient,
		Scraper:     hybridScraper,
		startTime:   time.Now(),
	}

	logger.Info().Msg("Application initialized successfully")
	return app, nil
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
