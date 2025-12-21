// internal/cache/cache.go
package cache

import (
	"container/list"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/law-makers/crawl/pkg/models"
	"github.com/rs/zerolog/log"
)

// Cache defines the interface for response caching implementations.
//
// Implementations should provide efficient retrieval and eviction strategies.
// Common implementations include:
//   - MemoryCache: In-memory cache with LRU eviction
type Cache interface {
	// Get retrieves a cached response by key.
	// Returns the cached PageData and a boolean indicating if the key was found.
	Get(key string) (*models.PageData, bool)

	// Set stores a response in cache with the specified TTL.
	// If the key already exists, it should be updated.
	// Implementations may evict entries based on their eviction strategy.
	Set(key string, data *models.PageData, ttl time.Duration) error

	// Delete removes a cached response by key.
	// Should not error if the key doesn't exist.
	Delete(key string) error

	// Clear removes all cached responses.
	Clear() error

	// Close performs cleanup and closes the cache.
	// Implementations must ensure background goroutines are stopped.
	Close()
}

// cacheEntry represents a cached page response with metadata
type cacheEntry struct {
	Data      *models.PageData
	ExpiresAt time.Time
	Key       string // For LRU tracking
}

// Point 7: LRU cache implementation for smart eviction
// MemoryCache implements in-memory response caching with LRU eviction
type MemoryCache struct {
	store   map[string]*list.Element // Map key to list element
	lruList *list.List               // Doubly-linked list for LRU ordering
	mu      sync.RWMutex
	maxSize int64 // Maximum cache size in bytes
	size    int64 // Current size in bytes
	ctx     context.Context
	cancel  context.CancelFunc
	hits    uint64 // Cache hit counter
	misses  uint64 // Cache miss counter
}

// NewMemoryCache creates a new in-memory cache with LRU eviction
func NewMemoryCache(maxSizeBytes int64) *MemoryCache {
	if maxSizeBytes <= 0 {
		maxSizeBytes = 100 * 1024 * 1024 // Default: 100MB
	}

	ctx, cancel := context.WithCancel(context.Background())

	cache := &MemoryCache{
		store:   make(map[string]*list.Element),
		lruList: list.New(),
		maxSize: maxSizeBytes,
		size:    0,
		ctx:     ctx,
		cancel:  cancel,
	}

	// Start background cleanup routine with context
	go cache.cleanupExpired()

	return cache
}

// Get retrieves a cached response
// Point 8B: Manual unlock instead of defer for hot path optimization
// Point 7: LRU - moves accessed item to front of list
func (mc *MemoryCache) Get(key string) (*models.PageData, bool) {
	mc.mu.Lock() // Need write lock for LRU update
	element, exists := mc.store[key]
	if !exists {
		mc.misses++
		mc.mu.Unlock()
		return nil, false
	}

	entry := element.Value.(*cacheEntry)

	// Check if expired
	if time.Now().After(entry.ExpiresAt) {
		mc.misses++
		mc.mu.Unlock()
		// Expired, delete it
		go mc.Delete(key)
		return nil, false
	}

	// Move to front (most recently used)
	mc.lruList.MoveToFront(element)
	mc.hits++
	mc.mu.Unlock()

	log.Info().Str("key", key).Msg("Cache hit")
	return entry.Data, true
}

// Set stores a response in cache with TTL
// Point 7: LRU - adds to front of list
func (mc *MemoryCache) Set(key string, data *models.PageData, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = 5 * time.Minute // Default: 5 minutes
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Estimate size (rough approximation)
	size := int64(len(data.HTML) + len(data.Content) + len(data.Title))
	size += 1024 // Add ~1KB overhead for struct, pointers, maps, slices

	// Check if key already exists - update it
	if element, exists := mc.store[key]; exists {
		oldEntry := element.Value.(*cacheEntry)
		oldSize := int64(len(oldEntry.Data.HTML) + len(oldEntry.Data.Content))
		mc.size -= oldSize

		// Update entry
		entry := &cacheEntry{
			Data:      data,
			ExpiresAt: time.Now().Add(ttl),
			Key:       key,
		}
		element.Value = entry
		mc.lruList.MoveToFront(element)
		mc.size += size

		log.Debug().
			Str("key", key).
			Dur("ttl", ttl).
			Int64("size_bytes", size).
			Msg("Updated cache entry")

		return nil
	}

	// Check if we need to evict entries (LRU eviction)
	for mc.size+size > mc.maxSize && mc.lruList.Len() > 0 {
		mc.evictLRU()
	}

	entry := &cacheEntry{
		Data:      data,
		ExpiresAt: time.Now().Add(ttl),
		Key:       key,
	}

	// Add to front of list (most recently used)
	element := mc.lruList.PushFront(entry)
	mc.store[key] = element
	mc.size += size

	log.Debug().
		Str("key", key).
		Dur("ttl", ttl).
		Int64("size_bytes", size).
		Msg("Cached response")

	return nil
}

// Delete removes a cached response
func (mc *MemoryCache) Delete(key string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if element, exists := mc.store[key]; exists {
		entry := element.Value.(*cacheEntry)
		size := int64(len(entry.Data.HTML) + len(entry.Data.Content))
		mc.lruList.Remove(element)
		delete(mc.store, key)
		mc.size -= size
		log.Debug().Str("key", key).Msg("Deleted from cache")
	}

	return nil
}

// Clear removes all cached responses
func (mc *MemoryCache) Clear() error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.store = make(map[string]*list.Element)
	mc.lruList = list.New()
	mc.size = 0
	mc.hits = 0
	mc.misses = 0

	log.Debug().Msg("Cache cleared")
	return nil
}

// Close stops the background cleanup goroutine
func (mc *MemoryCache) Close() {
	mc.cancel()
	log.Debug().Msg("Cache closed")
}

// evictLRU removes the least recently used entry from cache (must be called with lock held)
// Point 7: LRU eviction - removes from back of list
func (mc *MemoryCache) evictLRU() {
	element := mc.lruList.Back()
	if element == nil {
		return
	}

	entry := element.Value.(*cacheEntry)
	size := int64(len(entry.Data.HTML) + len(entry.Data.Content))

	mc.lruList.Remove(element)
	delete(mc.store, entry.Key)
	mc.size -= size

	log.Debug().Str("key", entry.Key).Msg("Evicted from cache (LRU)")
}

// cleanupExpired periodically removes expired entries
func (mc *MemoryCache) cleanupExpired() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mc.mu.Lock()
			now := time.Now()

			// Iterate over list to find expired entries
			var next *list.Element
			for element := mc.lruList.Front(); element != nil; element = next {
				next = element.Next()
				entry := element.Value.(*cacheEntry)

				if now.After(entry.ExpiresAt) {
					size := int64(len(entry.Data.HTML) + len(entry.Data.Content))
					mc.lruList.Remove(element)
					delete(mc.store, entry.Key)
					mc.size -= size
				}
			}
			mc.mu.Unlock()
		case <-mc.ctx.Done():
			log.Debug().Msg("Cache cleanup routine stopped")
			return
		}
	}
}

// Stats returns cache statistics including hit rate
func (mc *MemoryCache) Stats() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	hitRate := 0.0
	total := mc.hits + mc.misses
	if total > 0 {
		hitRate = float64(mc.hits) / float64(total) * 100
	}

	return map[string]interface{}{
		"entries":     mc.lruList.Len(),
		"size_bytes":  mc.size,
		"max_size":    mc.maxSize,
		"utilization": float64(mc.size) / float64(mc.maxSize) * 100,
		"hits":        mc.hits,
		"misses":      mc.misses,
		"hit_rate":    hitRate,
	}
}

// CacheKeyFromURL generates a cache key from a URL and selector
func CacheKeyFromURL(url, selector string) string {
	if selector != "" && selector != "body" {
		return fmt.Sprintf("%s::%s", url, selector)
	}
	return url
}
