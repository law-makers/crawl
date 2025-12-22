// internal/engine/batch/concurrency.go
package batch

import (
	"runtime"
)

// OptimalConcurrency calculates optimal concurrency based on CPU and memory
func OptimalConcurrency() int {
	numCPU := runtime.NumCPU()

	// For I/O bound operations (scraping), use 2-4x CPU count
	optimal := numCPU * 3

	// Cap based on available memory
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	availMB := (m.Sys - m.Alloc) / 1024 / 1024

	// Assume ~50MB per browser context for dynamic scraping
	maxByMemory := int(availMB / 50)

	// Don't go below CPU count or above 50
	if optimal < numCPU {
		optimal = numCPU
	}
	if optimal > 50 {
		optimal = 50
	}

	if maxByMemory > 0 && maxByMemory < optimal {
		return maxByMemory
	}
	return optimal
}
