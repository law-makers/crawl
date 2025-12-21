package config

import "time"

// Default constants for application configuration
const (
	DefaultLogLevel              = "info"
	DefaultJSONLog               = false
	DefaultUserAgent             = "Crawl/1.0 (https://github.com/law-makers/crawl)"
	DefaultCacheTTL              = 5 * time.Minute
	DefaultHTTPTimeout           = 30 * time.Second
	DefaultStaticRateLimitRPS    = 5.0
	DefaultStaticRateLimitBurst  = 10
	DefaultDynamicRateLimitRPS   = 3.0
	DefaultDynamicRateLimitBurst = 5
	DefaultBrowserPoolSize       = 3
	DefaultMaxBrowserPoolSize    = 10
	DefaultBrowserHeadless       = true
	DefaultCacheMaxSizeBytes     = 100 * 1024 * 1024 // 100MB
	DefaultJSWaitTime            = 500 * time.Millisecond
	DefaultPoolAcquireTTL        = 10 * time.Second
)
