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
	"github.com/law-makers/crawl/internal/ui"
	headersutil "github.com/law-makers/crawl/internal/utils/headers"
	"github.com/law-makers/crawl/pkg/models"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	mediaType   string
	concurrency int
	outputDir   string
	waitSeconds int
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
	mediaCmd.Flags().IntVar(&waitSeconds, "wait", 0, "Seconds to wait after page loads before scraping (static and SPA)")
	mediaCmd.Flags().StringArrayVarP(&headers, "header", "H", []string{}, "Custom headers")

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
	case "static":
		scraperMode = models.ModeStatic
	case "spa":
		scraperMode = models.ModeSPA
	}

	// Parse custom headers
	headerMap := headersutil.ParseHeaders(headers)

	// Create scraper to fetch the page
	var scraper engine.Scraper

	// Get app from command context
	appCtx := GetAppFromCmd(cmd)
	if appCtx == nil {
		return fmt.Errorf("application not initialized")
	}

	// Use the scraper from the app
	scraper = appCtx.Scraper

	// Fetch the page
	opts := models.RequestOptions{
		URL:     pageURL,
		Mode:    scraperMode,
		Headers: headerMap,
		Timeout: 30 * time.Second,
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
		fmt.Println("\n" + ui.Info("❌ No media files found."))
		fmt.Println("\n" + ui.Info("💡 TIP: Try using --mode=spa for JavaScript-heavy sites"))
		return nil
	}

	log.Debug().Int("count", len(mediaURLs)).Msg("Media URLs extracted")
	// Only show detailed file preview when verbose or JSON logging is enabled.
	if verbose || jsonOutput {
		fmt.Printf("\n%s %s\n", ui.Bold("Found"), ui.ColorWhite+fmt.Sprintf("%d media file(s):", len(mediaURLs))+ui.ColorReset)
		for i, url := range mediaURLs {
			fmt.Printf("  %s %d. %s\n", ui.ColorDim, i+1, ui.ColorWhite+url+ui.ColorReset)
		}
		fmt.Println()
	} else {
		// Minimal output: only show the count so the progress bar remains the primary output.
		fmt.Printf("\n%s %s\n\n", ui.Bold("Found"), ui.ColorWhite+fmt.Sprintf("%d media file(s).", len(mediaURLs))+ui.ColorReset)
	}

	// Create output directory
	absOutputDir, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("invalid output directory: %w", err)
	}

	// Create worker pool
	pool := downloader.NewWorkerPool(concurrency, 60*time.Second, "Crawl/1.0")

	// Start downloads
	fmt.Printf("%s %s\n\n", ui.Info("Starting download with"), ui.ColorWhite+fmt.Sprintf("%d workers...", concurrency)+ui.ColorReset)
	ctx := context.Background()

	downloadOpts := downloader.DownloadOptions{
		OutputDir: absOutputDir,
		Headers:   headerMap,
	}

	// Reduce console logging during the download phase so the progress bar remains the primary output.
	prevLevel := zerolog.GlobalLevel()
	if !verbose && !jsonOutput {
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	}
	results := pool.DownloadBatch(ctx, mediaURLs, downloadOpts)
	// Restore previous log level
	zerolog.SetGlobalLevel(prevLevel)

	// Print results
	successCount := 0
	failCount := 0
	totalSize := int64(0)
	totalDuration := time.Duration(0)

	// Only show detailed results header if verbose or JSON output is enabled.
	if verbose || jsonOutput {
		fmt.Println("\n" + ui.Bold("Download Results:"))
	}

	for i, result := range results {
		if result.Success {
			successCount++
			totalSize += result.Size
			totalDuration += result.Duration
			if verbose || jsonOutput {
				fmt.Printf("%s [%d/%d] %s\n", ui.Success("✓"), i+1, len(results), ui.ColorWhite+filepath.Base(result.FilePath)+ui.ColorReset)
				fmt.Printf("  %s %s  %s %v\n", ui.ColorDim+"Size:", ui.ColorWhite+formatBytes(result.Size)+ui.ColorReset, ui.ColorDim+"Duration:", result.Duration.Round(time.Millisecond))
			}
		} else {
			failCount++
			if verbose || jsonOutput {
				fmt.Printf("%s [%d/%d] %s\n", ui.Error("✗"), i+1, len(results), ui.ColorWhite+result.URL+ui.ColorReset)
				fmt.Printf("  %s %s\n", ui.ColorDim+"Error:", ui.Error(fmt.Sprintf("%v", result.Error)))
			}
		}
	}

	// Compute average duration (0 if no successes) and print the summary via helper
	avgDuration := time.Duration(0)
	if successCount > 0 {
		avgDuration = totalDuration / time.Duration(successCount)
	}
	printSummary(verbose || jsonOutput, len(results), successCount, failCount, totalSize, avgDuration, absOutputDir)

	if failCount > 0 {
		// Avoid printing usage/help when downloads had partial failures; the summary already provides details.
		cmd.SilenceUsage = true
		return fmt.Errorf("%d download(s) failed", failCount)
	}

	return nil
}

// printSummary prints a concise or detailed summary depending on the 'detailed' flag.
func printSummary(detailed bool, total, success, failed int, totalSize int64, avg time.Duration, outDir string) {
	// For non-detailed output ensure a leading blank line so it doesn't attach to the progress bar
	if !detailed {
		fmt.Println()
	}
	fmt.Printf("\n%s\n", ui.Bold("Summary:"))
	fmt.Printf("  %s %s\n", ui.ColorBold+"Total:"+ui.ColorReset, ui.ColorWhite+fmt.Sprintf("%d files", total)+ui.ColorReset)
	fmt.Printf("  %s %s\n", ui.ColorBold+"Success:"+ui.ColorReset, ui.Success(fmt.Sprintf("%d", success)))
	fmt.Printf("  %s %s\n", ui.ColorBold+"Failed:"+ui.ColorReset, ui.Error(fmt.Sprintf("%d", failed)))
	fmt.Printf("  %s %s\n", ui.ColorBold+"Total Size:"+ui.ColorReset, ui.ColorWhite+formatBytes(totalSize)+ui.ColorReset)
	if success > 0 {
		fmt.Printf("  %s %s\n", ui.ColorBold+"Average Time:"+ui.ColorReset, ui.ColorWhite+avg.Round(time.Millisecond).String()+ui.ColorReset)
	}
	fmt.Printf("  %s %s\n", ui.ColorBold+"Output Directory:"+ui.ColorReset, ui.ColorWhite+outDir+ui.ColorReset)
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
