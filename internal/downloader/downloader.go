// internal/downloader/downloader.go
package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/law-makers/crawl/internal/retry"
	"github.com/rs/zerolog/log"
)

// DownloadResult represents the result of a download operation
type DownloadResult struct {
	URL       string
	FilePath  string
	Size      int64
	Success   bool
	Error     error
	StartTime time.Time
	Duration  time.Duration
}

// DownloadError provides detailed context about download failures
type DownloadError struct {
	URL         string
	StatusCode  int
	Message     string
	BodySnippet string // First 500 chars
	Underlying  error
}

func (e *DownloadError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("download failed for %s: %s (status: %d)", e.URL, e.Message, e.StatusCode)
	}
	return fmt.Sprintf("download failed for %s: %s", e.URL, e.Message)
}

func (e *DownloadError) Unwrap() error { return e.Underlying }

// GetStatusCode returns the HTTP status code if applicable
func (e *DownloadError) GetStatusCode() int {
	return e.StatusCode
}

// DownloadOptions configures the download behavior
type DownloadOptions struct {
	OutputDir string
	Timeout   time.Duration
	Filename  string
	UserAgent string
	Headers   map[string]string
}

// Downloader handles concurrent media downloads with streaming I/O
type Downloader struct {
	client    *http.Client
	userAgent string
}

var bufferPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 32*1024) // 32KB
		return &b
	},
}

var filenameReplacer = strings.NewReplacer(
	"/", "_",
	"\\", "_",
	"..", "_",
	":", "_",
	"*", "_",
	"?", "_",
	"\"", "_",
	"<", "_",
	">", "_",
	"|", "_",
)

// NewDownloader creates a new Downloader instance
func NewDownloader(timeout time.Duration, userAgent string) *Downloader {
	if userAgent == "" {
		userAgent = "Crawl/1.0 (https://github.com/law-makers/crawl)"
	}

	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	return &Downloader{
		client:    client,
		userAgent: userAgent,
	}
}

// Download downloads a single file with streaming I/O
func (d *Downloader) Download(ctx context.Context, fileURL string, opts DownloadOptions) *DownloadResult {
	result := &DownloadResult{
		URL:       fileURL,
		StartTime: time.Now(),
		Success:   false,
	}

	// Wrap download with retry logic
	retryConfig := retry.Config{
		MaxAttempts:    3,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     10 * time.Second,
		Multiplier:     2.0,
		RetryableStatusCodes: []int{
			http.StatusTooManyRequests,
			http.StatusInternalServerError,
			http.StatusBadGateway,
			http.StatusServiceUnavailable,
			http.StatusGatewayTimeout,
		},
	}

	err := retry.WithRetry(ctx, retryConfig, func() error {
		return d.downloadOnce(ctx, fileURL, opts, result)
	})

	if err != nil {
		result.Error = err
		result.Success = false
	}

	result.Duration = time.Since(result.StartTime)
	return result
}

// downloadOnce performs a single download attempt
func (d *Downloader) downloadOnce(ctx context.Context, fileURL string, opts DownloadOptions, result *DownloadResult) error {
	// Validate URL
	u, err := url.Parse(fileURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Create output directory
	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Determine filename
	filename := opts.Filename
	if filename == "" {
		filename = sanitizeFilename(fileURL, u)
	} else {
		filename = sanitizeFilename(filename, nil)
	}

	filePath := filepath.Join(opts.OutputDir, filename)
	result.FilePath = filePath

	// Check for existing file to support resume
	var startByte int64
	if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
		startByte = info.Size()
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", fileURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("User-Agent", d.userAgent)
	for key, value := range opts.Headers {
		req.Header.Set(key, value)
	}

	// Add Range header if we have existing data
	if startByte > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", startByte))
	}

	// Execute request
	resp, err := d.client.Do(req)
	if err != nil {
		return &DownloadError{
			URL:        fileURL,
			Message:    "request failed",
			Underlying: err,
		}
	}
	defer resp.Body.Close()

	// Handle response status
	var outFile *os.File
	var appendMode bool

	switch resp.StatusCode {
	case http.StatusOK:
		// Server doesn't support range or file didn't exist, overwrite
		outFile, err = os.Create(filePath)
		appendMode = false
	case http.StatusPartialContent:
		// Server supports range, append
		outFile, err = os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0644)
		appendMode = true
	case http.StatusRequestedRangeNotSatisfiable:
		// File is likely already complete
		result.Size = startByte
		result.Success = true
		return nil
	default:
		// Read snippet of body for context
		snippet := make([]byte, 500)
		n, _ := io.ReadFull(resp.Body, snippet)

		return &DownloadError{
			URL:         fileURL,
			StatusCode:  resp.StatusCode,
			Message:     resp.Status,
			BodySnippet: string(snippet[:n]),
		}
	}

	if err != nil {
		return &DownloadError{
			URL:        fileURL,
			Message:    "failed to open file",
			Underlying: err,
		}
	}
	defer outFile.Close()

	// Stream to disk
	buf := bufferPool.Get().(*[]byte)
	defer bufferPool.Put(buf)
	bytesWritten, err := io.CopyBuffer(outFile, resp.Body, *buf)
	if err != nil {
		return &DownloadError{
			URL:        fileURL,
			Message:    "failed to write file",
			Underlying: err,
		}
	}

	result.Size = bytesWritten
	if appendMode {
		result.Size += startByte
	}
	result.Success = true

	log.Debug().
		Str("url", fileURL).
		Str("file", filePath).
		Int64("bytes", result.Size).
		Msg("Download completed")

	return nil
}

// sanitizeFilename prevents path traversal attacks
func sanitizeFilename(input string, u *url.URL) string {
	// Extract filename from URL
	var queryHash string

	if u != nil {
		if u.Host != "" {
			parts := strings.Split(u.Path, "/")
			if len(parts) > 0 {
				input = parts[len(parts)-1]
			}
			if u.RawQuery != "" {
				queryHash = "_" + hashString(u.RawQuery)
			}
		}
	} else {
		if parsed, err := url.Parse(input); err == nil && parsed.Host != "" {
			parts := strings.Split(parsed.Path, "/")
			if len(parts) > 0 {
				input = parts[len(parts)-1]
			}
			if parsed.RawQuery != "" {
				queryHash = "_" + hashString(parsed.RawQuery)
			}
		}
	}

	// Remove dangerous characters
	input = filenameReplacer.Replace(input)

	input = strings.TrimSpace(input)
	input = strings.Trim(input, ".")

	// Extract extension before appending query hash
	ext := filepath.Ext(input)
	stem := strings.TrimSuffix(input, ext)

	// Append query hash before extension
	if queryHash != "" {
		input = stem + queryHash + ext
	}

	if input == "" {
		input = fmt.Sprintf("download_%d", time.Now().Unix())
	}
	if len(input) > 200 {
		input = input[:200]
	}

	return input
}

// hashString creates a simple hash for unique filenames
func hashString(s string) string {
	hash := 0
	for _, c := range s {
		hash = ((hash << 5) - hash) + int(c)
	}
	if hash < 0 {
		hash = -hash
	}
	hex := fmt.Sprintf("%x", hash)
	if len(hex) < 8 {
		hex = strings.Repeat("0", 8-len(hex)) + hex
	}
	return hex[:8]
}
