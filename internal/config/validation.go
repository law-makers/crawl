package config

import "fmt"

func validate(c *Config) error {
	if c.HTTPTimeout <= 0 {
		return fmt.Errorf("http timeout must be > 0")
	}
	if c.BrowserPoolSize <= 0 || c.BrowserPoolSize > DefaultMaxBrowserPoolSize {
		return fmt.Errorf("browser pool size must be between 1 and %d", DefaultMaxBrowserPoolSize)
	}
	if c.CacheMaxSizeBytes <= 0 {
		return fmt.Errorf("cache max size must be > 0")
	}
	return nil
}
