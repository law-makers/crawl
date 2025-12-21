package config

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// Config holds application configuration values
type Config struct {
	// Logging
	LogLevel string
	JSONLog  bool

	// HTTP/Scraping
	HTTPTimeout time.Duration
	UserAgent   string
	Proxy       string

	// Rate Limiting
	StaticRateLimitRPS    float64
	StaticRateLimitBurst  int
	DynamicRateLimitRPS   float64
	DynamicRateLimitBurst int

	// Browser Pool
	BrowserPoolSize int
	BrowserHeadless bool
	ChromePath      string

	// Caching
	CacheTTL          time.Duration
	CacheMaxSizeBytes int64

	// Feature Flags
	EnableBatch bool
}

// Load builds a Config by combining defaults, an optional config file, environment variables, and CLI flags.
// Caller should pass the root *cobra.Command so flags can be read.
func Load(cmd *cobra.Command) (*Config, error) {
	cfg := &Config{
		LogLevel:              DefaultLogLevel,
		JSONLog:               DefaultJSONLog,
		HTTPTimeout:           DefaultHTTPTimeout,
		UserAgent:             DefaultUserAgent,
		StaticRateLimitRPS:    DefaultStaticRateLimitRPS,
		StaticRateLimitBurst:  DefaultStaticRateLimitBurst,
		DynamicRateLimitRPS:   DefaultDynamicRateLimitRPS,
		DynamicRateLimitBurst: DefaultDynamicRateLimitBurst,
		BrowserPoolSize:       DefaultBrowserPoolSize,
		BrowserHeadless:       DefaultBrowserHeadless,
		CacheTTL:              DefaultCacheTTL,
		CacheMaxSizeBytes:     DefaultCacheMaxSizeBytes,
	}

	// Override from environment variables (simple helpers)
	if v := os.Getenv("CRAWL_USER_AGENT"); v != "" {
		cfg.UserAgent = v
	}
	if v := os.Getenv("CRAWL_PROXY"); v != "" {
		cfg.Proxy = v
	}
	if v := os.Getenv("CRAWL_CHROME_PATH"); v != "" {
		cfg.ChromePath = v
	}

	// Read CLI flags if provided
	if cmd != nil {
		if f := cmd.Flags().Lookup("user-agent"); f != nil {
			if s := f.Value.String(); s != "" {
				cfg.UserAgent = s
			}
		}
		if f := cmd.Flags().Lookup("proxy"); f != nil {
			if s := f.Value.String(); s != "" {
				cfg.Proxy = s
			}
		}
		if f := cmd.Flags().Lookup("timeout"); f != nil {
			if s := f.Value.String(); s != "" {
				if d, err := time.ParseDuration(s); err == nil {
					cfg.HTTPTimeout = d
				}
			}
		}
		if f := cmd.Flags().Lookup("json"); f != nil {
			if f.Value.String() == "true" {
				cfg.JSONLog = true
			}
		}
		if f := cmd.Flags().Lookup("verbose"); f != nil {
			if f.Value.String() == "true" {
				cfg.LogLevel = "debug"
			}
		}
	}

	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}
