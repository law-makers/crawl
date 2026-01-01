package downloader

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestDownload_Success(t *testing.T) {
	content := "test file content"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(content))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	dl := NewDownloader(10*time.Second, "Test/1.0")
	ctx := context.Background()

	result := dl.Download(ctx, server.URL+"/test.txt", DownloadOptions{
		OutputDir: tempDir,
	})

	if !result.Success {
		t.Fatalf("Download failed: %v", result.Error)
	}

	data, err := os.ReadFile(result.FilePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(data) != content {
		t.Errorf("Content mismatch: got %q, want %q", string(data), content)
	}
}

func TestSanitizeFilename_Security(t *testing.T) {
	dangerous := []string{
		"../../etc/passwd",
		"/etc/shadow",
		"file:with:colons",
	}

	for _, input := range dangerous {
		t.Run(input, func(t *testing.T) {
			result := sanitizeFilename(input, nil)
			if strings.Contains(result, "/") || strings.Contains(result, "\\") {
				t.Errorf("Sanitized filename contains path separator: %q", result)
			}
			if strings.Contains(result, "..") {
				t.Errorf("Sanitized filename contains '..': %q", result)
			}
		})
	}
}

func TestWorkerPool_Concurrency(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.Write([]byte("data"))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	urls := []string{
		server.URL + "/1.txt",
		server.URL + "/2.txt",
		server.URL + "/3.txt",
	}

	pool := NewWorkerPool(2, 10*time.Second, "Test/1.0")
	ctx := context.Background()

	results := pool.DownloadBatch(ctx, urls, DownloadOptions{
		OutputDir: tempDir,
	})

	if len(results) != len(urls) {
		t.Errorf("Result count mismatch: got %d, want %d", len(results), len(urls))
	}

	successCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		}
	}

	if successCount != len(urls) {
		t.Errorf("Not all downloads succeeded: %d/%d", successCount, len(urls))
	}
}

func TestWorkerPool_RecoversFromWorkerPanic(t *testing.T) {
	// Create a pool with a nil downloader to force a nil pointer deref panic inside the worker
	pool := &WorkerPool{
		downloader:  nil,
		concurrency: 2,
		rateLimiter: nil,
	}

	urls := []string{"http://example.com/1.jpg", "http://example.com/2.jpg", "http://example.com/3.jpg"}
	tmp := t.TempDir()
	results := pool.DownloadBatch(context.Background(), urls, DownloadOptions{OutputDir: tmp})

	if len(results) != len(urls) {
		t.Fatalf("Expected %d results, got %d", len(urls), len(results))
	}

	failedCount := 0
	for _, r := range results {
		if !r.Success {
			failedCount++
			if r.Error == nil || !strings.Contains(r.Error.Error(), "worker panic") {
				t.Errorf("Expected worker panic error, got: %v", r.Error)
			}
		}
	}

	if failedCount == 0 {
		t.Errorf("Expected at least one failed result due to worker panic")
	}
}

func BenchmarkSanitizeFilename(b *testing.B) {
	input := "https://example.com/path/to/file.mp4?query=param"
	u, _ := url.Parse(input)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sanitizeFilename(input, u)
	}
}

func TestHashString_Length(t *testing.T) {
	cases := []string{"", "a", "short", "withquery=1"}
	for _, c := range cases {
		h := hashString(c)
		if len(h) != 8 {
			t.Fatalf("hashString(%q) length = %d, want 8", c, len(h))
		}
	}
}
