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
	"time"

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

	// Validate URL
	if _, err := url.Parse(fileURL); err != nil {
		result.Error = fmt.Errorf("invalid URL: %w", err)
		result.Duration = time.Since(result.StartTime)
		return result
	}

	// Create output directory
	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		result.Error = fmt.Errorf("failed to create output directory: %w", err)
		result.Duration = time.Since(result.StartTime)
		return result
	}

	// Determine filename
	filename := opts.Filename
	if filename == "" {
		filename = sanitizeFilename(fileURL)
	} else {
		filename = sanitizeFilename(filename)
	}

	filePath := filepath.Join(opts.OutputDir, filename)
	result.FilePath = filePath

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", fileURL, nil)
	if err != nil {
		result.Error = fmt.Errorf("failed to create request: %w", err)
		result.Duration = time.Since(result.StartTime)
		return result
	}

	// Set headers
	req.Header.Set("User-Agent", d.userAgent)
	for key, value := range opts.Headers {
		req.Header.Set(key, value)
	}

	// Execute request
	resp, err := d.client.Do(req)
	if err != nil {
		result.Error = fmt.Errorf("request failed: %w", err)
		result.Duration = time.Since(result.StartTime)
		return result
	}
	defer resp.Body.Close()

	// Check status
	if resp.StatusCode != http.StatusOK {
		result.Error = fmt.Errorf("bad status: %d %s", resp.StatusCode, resp.Status)
		result.Duration = time.Since(result.StartTime)
		return result
	}

	// Create file
	outFile, err := os.Create(filePath)
	if err != nil {
		result.Error = fmt.Errorf("failed to create file: %w", err)
		result.Duration = time.Since(result.StartTime)
		return result
	}
	defer outFile.Close()

	// Stream to disk
	bytesWritten, err := io.Copy(outFile, resp.Body)
	if err != nil {
		result.Error = fmt.Errorf("failed to write file: %w", err)
		result.Duration = time.Since(result.StartTime)
		os.Remove(filePath)
		return result
	}

	result.Size = bytesWritten
	result.Success = true
	result.Duration = time.Since(result.StartTime)

	log.Debug().
		Str("url", fileURL).
		Str("file", filePath).
		Int64("bytes", bytesWritten).
		Dur("duration", result.Duration).
		Msg("Download completed")

	return result
}

// sanitizeFilename prevents path traversal attacks
func sanitizeFilename(input string) string {
	// Extract filename from URL
	var queryHash string
	if u, err := url.Parse(input); err == nil && u.Host != "" {
		parts := strings.Split(u.Path, "/")
		if len(parts) > 0 {
			input = parts[len(parts)-1]
		}
		if u.RawQuery != "" {
			queryHash = "_" + hashString(u.RawQuery)
		}
	}

	// Remove dangerous characters
	input = strings.ReplaceAll(input, "/", "_")
	input = strings.ReplaceAll(input, "\\", "_")
	input = strings.ReplaceAll(input, "..", "_")
	input = strings.ReplaceAll(input, ":", "_")
	input = strings.ReplaceAll(input, "*", "_")
	input = strings.ReplaceAll(input, "?", "_")
	input = strings.ReplaceAll(input, "\"", "_")
	input = strings.ReplaceAll(input, "<", "_")
	input = strings.ReplaceAll(input, ">", "_")
	input = strings.ReplaceAll(input, "|", "_")

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
	return fmt.Sprintf("%x", hash)[:8]
}
