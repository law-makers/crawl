// internal/downloader/pool.go
package downloader

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/law-makers/crawl/internal/ratelimit"
	"github.com/rs/zerolog/log"
	"github.com/schollz/progressbar/v3"
)

// WorkerPool manages concurrent downloads using a worker pool pattern
type WorkerPool struct {
	downloader  *Downloader
	concurrency int
	rateLimiter *ratelimit.DomainLimiter
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
		rateLimiter: ratelimit.NewDomainLimiter(5.0, 10), // 5 req/sec per domain
	}
}

// DownloadBatch downloads multiple files concurrently using the worker pool
func (wp *WorkerPool) DownloadBatch(ctx context.Context, urls []string, opts DownloadOptions) []*DownloadResult {
	if len(urls) == 0 {
		return []*DownloadResult{}
	}

	// Create progress bar
	bar := progressbar.NewOptions(len(urls),
		progressbar.OptionSetDescription("Downloading"),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetItsString("files"),
		progressbar.OptionThrottle(100*time.Millisecond),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionSetRenderBlankState(true),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)

	// Create channels for job distribution
	jobs := make(chan string, len(urls))
	results := make(chan *DownloadResult, len(urls))

	// Start workers
	var wg sync.WaitGroup
	for w := 1; w <= wp.concurrency; w++ {
		wg.Add(1)
		go wp.worker(ctx, w, jobs, results, opts, &wg, bar)
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

	// Finish progress bar
	bar.Finish()

	return allResults
}

// worker processes download jobs from the jobs channel
func (wp *WorkerPool) worker(ctx context.Context, id int, jobs <-chan string, results chan<- *DownloadResult, opts DownloadOptions, wg *sync.WaitGroup, bar *progressbar.ProgressBar) {
	defer wg.Done()

	log.Debug().Int("worker_id", id).Msg("Worker started")

	for url := range jobs {
		currentURL := url

		// Check if context is cancelled
		select {
		case <-ctx.Done():
			log.Debug().Int("worker_id", id).Msg("Worker cancelled")
			return
		default:
		}

		// Process each job inside its own recover-protected function so a panic doesn't kill the worker
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Capture stack trace for diagnostics and emit a short error message for normal runs.
					stack := debug.Stack()
					// Emit a concise error at ERROR level so the console remains readable. Full stack trace goes to DEBUG only.
					log.Error().Int("worker_id", id).Str("url", currentURL).Str("panic", fmt.Sprintf("%v", r)).Msg("Worker panicked; enabling debug logs will show full stack")
					// Log the full stack at DEBUG level for diagnostics
					log.Debug().Bytes("stack", stack).Msg("Worker panic stack trace")

					// Safely advance progress bar if available. Protect bar.Add from causing additional panics.
					if bar != nil {
						func() {
							defer func() { recover() }()
							bar.Add(1)
						}()
					}

					// Ensure we produce a failed result for the current URL so callers receive a result for every job.
					if results != nil {
						func() {
							defer func() { recover() }()
							results <- &DownloadResult{URL: currentURL, Success: false, Error: fmt.Errorf("worker panic: %v", r)}
						}()
					}
				}
			}()

			log.Debug().
				Int("worker_id", id).
				Str("url", url).
				Msg("Worker processing download")

			// Apply rate limiting before download
			if wp.rateLimiter != nil {
				if err := wp.rateLimiter.Wait(ctx, url); err != nil {
					log.Warn().Err(err).Str("url", url).Msg("Rate limit error")
				}
			}

			// Download the file
			result := wp.downloader.Download(ctx, url, opts)

			// Update progress bar
			if bar != nil {
				bar.Add(1)
			}

			// Send result back
			select {
			case results <- result:
			case <-ctx.Done():
				return
			}
		}()
	}

	log.Debug().Int("worker_id", id).Msg("Worker finished")
}
