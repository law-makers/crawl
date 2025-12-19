// internal/downloader/pool.go
package downloader

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// WorkerPool manages concurrent downloads using a worker pool pattern
type WorkerPool struct {
	downloader  *Downloader
	concurrency int
}

// NewWorkerPool creates a new worker pool with specified concurrency
func NewWorkerPool(concurrency int, timeout time.Duration, userAgent string) *WorkerPool {
	if concurrency <= 0 {
		concurrency = 5 // Default to 5 workers
	}
	if concurrency > 50 {
		concurrency = 50 // Max 50 workers to avoid overwhelming the system
	}

	return &WorkerPool{
		downloader:  NewDownloader(timeout, userAgent),
		concurrency: concurrency,
	}
}

// DownloadBatch downloads multiple files concurrently using the worker pool
func (wp *WorkerPool) DownloadBatch(ctx context.Context, urls []string, opts DownloadOptions) []*DownloadResult {
	if len(urls) == 0 {
		return []*DownloadResult{}
	}

	// Create channels for job distribution
	jobs := make(chan string, len(urls))
	results := make(chan *DownloadResult, len(urls))

	// Start workers
	var wg sync.WaitGroup
	for w := 1; w <= wp.concurrency; w++ {
		wg.Add(1)
		go wp.worker(ctx, w, jobs, results, opts, &wg)
	}

	// Send jobs to workers
	go func() {
		for _, url := range urls {
			jobs <- url
		}
		close(jobs)
	}()

	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	allResults := make([]*DownloadResult, 0, len(urls))
	for result := range results {
		allResults = append(allResults, result)
	}

	return allResults
}

// worker processes download jobs from the jobs channel
func (wp *WorkerPool) worker(ctx context.Context, id int, jobs <-chan string, results chan<- *DownloadResult, opts DownloadOptions, wg *sync.WaitGroup) {
	defer wg.Done()

	log.Debug().Int("worker_id", id).Msg("Worker started")

	for url := range jobs {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			log.Debug().Int("worker_id", id).Msg("Worker cancelled")
			return
		default:
		}

		log.Debug().
			Int("worker_id", id).
			Str("url", url).
			Msg("Worker processing download")

		// Download the file
		result := wp.downloader.Download(ctx, url, opts)

		// Send result back
		select {
		case results <- result:
		case <-ctx.Done():
			return
		}
	}

	log.Debug().Int("worker_id", id).Msg("Worker finished")
}
