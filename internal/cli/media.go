// internal/cli/media.go
package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/law-makers/crawl/internal/downloader"
	"github.com/law-makers/crawl/internal/engine"
	"github.com/law-makers/crawl/pkg/models"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	mediaType   string
	concurrency int
	outputDir   string
)

// mediaCmd represents the media command
var mediaCmd = &cobra.Command{
	Use:   "media <url>",
	Short: "Download media files (images, videos, audio) from a URL",
	Long: `Extracts and downloads media files from a web page using concurrent workers.

The media command can:
  - Extract images, videos, or audio files from any web page
  - Download multiple files concurrently using a worker pool
  - Handle both static HTML and JavaScript-rendered SPAs
  - Stream large files to disk without loading into RAM

Perfect for downloading galleries, video collections, or scraping media-heavy sites.`,
	Example: `  # Download all images from a page
  crawl media https://example.com --type=image

  # Download videos with 10 concurrent workers
  crawl media https://example.com/videos --type=video --concurrency=10

  # Download all media types to a specific directory
  crawl media https://example.com --type=all --output=./downloads

  # Download from a SPA that requires JavaScript
  crawl media https://spa-site.com --mode=spa --type=video`,
	Args: cobra.ExactArgs(1),
	RunE: runMedia,
}

func init() {
	rootCmd.AddCommand(mediaCmd)

	mediaCmd.Flags().StringVarP(&mediaType, "type", "t", "all", "Media type to download: image, video, audio, or all")
	mediaCmd.Flags().IntVarP(&concurrency, "concurrency", "c", 5, "Number of concurrent download workers (1-50)")
	mediaCmd.Flags().StringVarP(&outputDir, "output", "o", "./downloads", "Directory to save downloaded files")
	mediaCmd.Flags().StringVarP(&mode, "mode", "m", "auto", "Scraper mode: auto, static, or spa")
	mediaCmd.Flags().StringArrayVarP(&headers, "header", "H", []string{}, "Custom headers")
	mediaCmd.Flags().StringVarP(&sessionName, "session", "s", "", "Name of a saved auth session to use")
}

func runMedia(cmd *cobra.Command, args []string) error {
	pageURL := args[0]

	// Validate URL
	if !strings.HasPrefix(pageURL, "http://") && !strings.HasPrefix(pageURL, "https://") {
		return fmt.Errorf("invalid URL: must start with http:// or https://")
	}

	// Validate media type
	var mediaTypeEnum downloader.MediaType
	switch strings.ToLower(mediaType) {
	case "image", "img":
		mediaTypeEnum = downloader.MediaTypeImage
	case "video", "vid":
		mediaTypeEnum = downloader.MediaTypeVideo
	case "audio":
		mediaTypeEnum = downloader.MediaTypeAudio
	case "all":
		mediaTypeEnum = downloader.MediaTypeAll
	default:
		return fmt.Errorf("invalid media type: %s (must be image, video, audio, or all)", mediaType)
	}

	// Validate concurrency
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > 50 {
		concurrency = 50
	}

	log.Debug().
		Str("url", pageURL).
		Str("type", string(mediaTypeEnum)).
		Int("concurrency", concurrency).
		Str("output", outputDir).
		Msg("Starting media extraction")

	// Parse mode
	scraperMode := models.ModeAuto
	switch strings.ToLower(mode) {
	case "auto":
		scraperMode = models.ModeAuto
		// Auto-detect Instagram - always use SPA mode
		if strings.Contains(pageURL, "instagram.com") {
			scraperMode = models.ModeSPA
			fmt.Println("\n🔍 Instagram detected - using SPA mode...")
		}
	case "static":
		scraperMode = models.ModeStatic
	case "spa":
		scraperMode = models.ModeSPA
	}

	// Parse custom headers
	headerMap := make(map[string]string)
	for _, h := range headers {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) == 2 {
			headerMap[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	// Create scraper to fetch the page
	var scraper engine.Scraper
	switch scraperMode {
	case models.ModeStatic, models.ModeAuto:
		scraper = engine.NewStaticScraper()
	case models.ModeSPA:
		scraper = engine.NewDynamicScraper()
	}

	// Fetch the page
	opts := models.RequestOptions{
		URL:         pageURL,
		Mode:        scraperMode,
		Headers:     headerMap,
		Timeout:     30 * time.Second,
		SessionName: sessionName,
	}

	log.Debug().Str("scraper", scraper.Name()).Msg("Fetching page")
	pageData, err := scraper.Fetch(opts)
	if err != nil {
		return fmt.Errorf("failed to fetch page: %w", err)
	}

	log.Debug().
		Int("status", pageData.StatusCode).
		Int64("response_time_ms", pageData.ResponseTime).
		Msg("Page fetched successfully")

	// Extract media URLs from the HTML
	log.Debug().Msg("Extracting media URLs")
	mediaURLs, err := downloader.ExtractMedia(pageData.HTML, pageURL, mediaTypeEnum)
	if err != nil {
		return fmt.Errorf("failed to extract media: %w", err)
	}

	if len(mediaURLs) == 0 {
		log.Debug().Msg("No media files found on this page")
		fmt.Println("\n❌ No media files found.")
		fmt.Println("\n💡 TIP: Instagram requires SPA mode. Try:")
		fmt.Printf("   ./crawl media %s --session=instagram --mode=spa\n\n", pageURL)
		return nil
	}

	log.Debug().Int("count", len(mediaURLs)).Msg("Media URLs extracted")
	fmt.Printf("\nFound %d media file(s):\n", len(mediaURLs))
	for i, url := range mediaURLs {
		fmt.Printf("  %d. %s\n", i+1, url)
	}
	fmt.Println()

	// Create output directory
	absOutputDir, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("invalid output directory: %w", err)
	}

	// Create worker pool
	pool := downloader.NewWorkerPool(concurrency, 60*time.Second, "Crawl/1.0")

	// Start downloads
	fmt.Printf("Starting download with %d workers...\n\n", concurrency)
	ctx := context.Background()

	downloadOpts := downloader.DownloadOptions{
		OutputDir: absOutputDir,
		Headers:   headerMap,
	}

	results := pool.DownloadBatch(ctx, mediaURLs, downloadOpts)

	// Print results
	successCount := 0
	failCount := 0
	totalSize := int64(0)
	totalDuration := time.Duration(0)

	fmt.Println("\nDownload Results:")
	fmt.Println(strings.Repeat("=", 80))

	for i, result := range results {
		if result.Success {
			successCount++
			totalSize += result.Size
			totalDuration += result.Duration

			fmt.Printf("✓ [%d/%d] %s\n", i+1, len(results), filepath.Base(result.FilePath))
			fmt.Printf("  Size: %s  Duration: %v\n", formatBytes(result.Size), result.Duration.Round(time.Millisecond))
		} else {
			failCount++
			fmt.Printf("✗ [%d/%d] %s\n", i+1, len(results), result.URL)
			fmt.Printf("  Error: %v\n", result.Error)
		}
	}

	// Print summary
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("\nSummary:\n")
	fmt.Printf("  Total: %d files\n", len(results))
	fmt.Printf("  Success: %d\n", successCount)
	fmt.Printf("  Failed: %d\n", failCount)
	fmt.Printf("  Total Size: %s\n", formatBytes(totalSize))
	if successCount > 0 {
		avgDuration := totalDuration / time.Duration(successCount)
		fmt.Printf("  Average Time: %v per file\n", avgDuration.Round(time.Millisecond))
	}
	fmt.Printf("  Output Directory: %s\n", absOutputDir)

	if failCount > 0 {
		return fmt.Errorf("%d download(s) failed", failCount)
	}

	return nil
}

// formatBytes formats byte count as human-readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
