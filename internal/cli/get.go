// internal/cli/get.go
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/law-makers/crawl/internal/engine"
	"github.com/law-makers/crawl/internal/ui"
	headersutil "github.com/law-makers/crawl/internal/utils/headers"
	outpututil "github.com/law-makers/crawl/internal/utils/output"
	urlutil "github.com/law-makers/crawl/internal/utils/url"
	"github.com/law-makers/crawl/pkg/models"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	mode        string
	selector    string
	output      string
	headers     []string
	sessionName string
	fields      string
)

// getCmd represents the get command
var getCmd = &cobra.Command{
	Use:   "get <url>",
	Short: "Retrieve text or data from a URL",
	Long: `Intelligently switches between the "Fast Engine" (HTTP/Static) and 
"Deep Engine" (Headless/SPA) to get raw HTML or structured data.

The scraper will auto-detect whether to use static or SPA mode, or you can 
force a specific mode using the --mode flag.`,
	Example: `  # Basic scrape (auto-detects static vs SPA)
  crawl get https://example.com

  # Force static mode for speed
  crawl get https://example.com --mode=static

  # Extract specific content with CSS selector
  crawl get https://example.com --selector=".price-tag"

  # Save output to JSON file
  crawl get https://example.com --output=data.json

  # Add custom headers
  crawl get https://example.com -H "Authorization: Bearer token"`,
	Args: cobra.ExactArgs(1),
	RunE: runGet,
}

func init() {
	rootCmd.AddCommand(getCmd)

	getCmd.Flags().StringVarP(&mode, "mode", "m", "auto", "Force engine mode: auto, static, or spa")
	getCmd.Flags().StringVarP(&selector, "selector", "s", "body", "CSS selector to extract (e.g., .price, #content)")
	getCmd.Flags().StringVarP(&output, "output", "o", "", "File path to save output (supports .json, .txt, .html, .csv, .md)")
	getCmd.Flags().StringArrayVarP(&headers, "header", "H", []string{}, "Custom headers (e.g., -H \"User-Agent: Bot\")")
	getCmd.Flags().StringVar(&sessionName, "session", "", "Name of a saved auth session to use")
	getCmd.Flags().StringVar(&fields, "fields", "", "Comma-separated fields for CSV export (e.g., name=.name,price=.price)")
}

func runGet(cmd *cobra.Command, args []string) error {
	url := args[0]

	// Validate URL
	if err := urlutil.ValidateURL(url); err != nil {
		return err
	}

	// Warn if using default broad selector
	if selector == "body" {
		log.Warn().Msg("Using default 'body' selector extracts entire page. Use --selector for specific content.")
	}

	// Parse mode
	scraperMode := models.ModeAuto
	switch strings.ToLower(mode) {
	case "auto":
		scraperMode = models.ModeAuto
	case "static":
		scraperMode = models.ModeStatic
	case "spa":
		scraperMode = models.ModeSPA
	default:
		return fmt.Errorf("invalid mode: %s (must be auto, static, or spa)", mode)
	}

	// Parse custom headers
	headerMap := headersutil.ParseHeaders(headers)

	// Add user agent if configured globally
	if userAgent != "" && headerMap["User-Agent"] == "" {
		headerMap["User-Agent"] = userAgent
	}

	// Parse fields
	fieldsMap := make(map[string]string)
	if fields != "" {
		pairs := strings.Split(fields, ",")
		for _, pair := range pairs {
			parts := strings.SplitN(pair, "=", 2)
			if len(parts) == 2 {
				fieldsMap[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
	}

	// Build request options
	opts := models.RequestOptions{
		URL:         url,
		Mode:        scraperMode,
		Selector:    selector,
		Fields:      fieldsMap,
		Headers:     headerMap,
		SessionName: sessionName,
		Timeout:     30 * time.Second,
		Proxy:       proxy, // Global proxy flag
	}

	// Parse timeout from global flag
	if timeout != "" {
		duration, err := time.ParseDuration(timeout)
		if err != nil {
			log.Warn().Str("timeout", timeout).Msg("Invalid timeout format, using default 30s")
		} else {
			opts.Timeout = duration
		}
	}

	// Select scraper based on requested mode
	var scraper engine.Scraper

	// Get app from command context
	appCtx := GetAppFromCmd(cmd)
	if appCtx == nil {
		return fmt.Errorf("application not initialized")
	}

	// Default: application-level scraper (hybrid)
	scraper = appCtx.Scraper

	switch scraperMode {
	case models.ModeStatic:
		if appCtx.StaticScraper != nil {
			scraper = appCtx.StaticScraper
			log.Debug().Msg("Using StaticScraper")
		}
	case models.ModeSPA:
		// Ensure browser pool exists before using the dynamic scraper
		if appCtx.DynamicScraper == nil {
			return fmt.Errorf("dynamic scraper is unavailable")
		}

		ctx, cancel := context.WithTimeout(context.Background(), appCtx.Config.HTTPTimeout*2)
		defer cancel()
		if appCtx.BrowserPool == nil {
			if err := appCtx.EnsureBrowserPool(ctx); err != nil {
				// If pool init fails, warn and continue - dynamic scraper can still
				// operate without a pooled browser (per-request chromedp alloc).
				log.Warn().Err(err).Msg("Failed to initialize browser pool; proceeding with per-request dynamic initialization")
			}
		}
		scraper = appCtx.DynamicScraper
		log.Debug().Msg("Using DynamicScraper (headless Chrome)")
	default:
		// ModeAuto - hybrid behavior (default)
		log.Debug().Msg("Using HybridScraper (auto)")
	}
	// Fetch data
	log.Debug().Str("url", url).Str("mode", string(scraperMode)).Msg("Fetching URL")
	pageData, err := scraper.Fetch(opts)
	if err != nil {
		return fmt.Errorf("failed to fetch URL: %w", err)
	}

	// Handle output
	if output != "" {
		return saveOutput(pageData, output)
	}

	// Print to stdout
	return printOutput(pageData)
}

func saveOutput(data *models.PageData, pathStr string) error {
	// Normalize extension checks to be case-insensitive
	path := strings.ToLower(pathStr)

	switch {
	case strings.HasSuffix(path, ".json"):
		if err := outpututil.SaveJSON(data, pathStr); err != nil {
			return fmt.Errorf("failed to save JSON: %w", err)
		}
	case strings.HasSuffix(path, ".html"):
		cleaned, err := outpututil.CleanHTML(data.HTML)
		if err != nil {
			return fmt.Errorf("failed to clean HTML: %w", err)
		}
		if err := os.WriteFile(pathStr, []byte(cleaned), 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
	case strings.HasSuffix(path, ".txt"):
		if err := os.WriteFile(pathStr, []byte(data.Content), 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
	case strings.HasSuffix(path, ".csv"):
		if err := outpututil.SaveCSV(data, pathStr); err != nil {
			return fmt.Errorf("failed to save CSV: %w", err)
		}
	case strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".markdown"):
		if err := outpututil.SaveMarkdown(data, pathStr); err != nil {
			return fmt.Errorf("failed to save Markdown: %w", err)
		}
	default:
		// Fallback to JSON for unknown extensions
		if err := outpututil.SaveJSON(data, pathStr); err != nil {
			return fmt.Errorf("failed to save JSON: %w", err)
		}
	}

	// Print metadata summary for saved outputs (single call)
	printMetadataSummary(data)

	// Make clickable link when possible using OSC 8 terminal hyperlink
	link := terminalHyperlink(filepath.Base(pathStr), pathStr)
	fmt.Printf("%s %s\n", ui.Success("✓ Saved to"), ui.ColorBold+link+ui.ColorReset)
	fmt.Printf("\n")
	log.Info().Str("file", pathStr).Msg("Output saved")
	return nil
}

// printMetadataSummary prints key metadata fields from PageData using colors and aligns columns
func printMetadataSummary(data *models.PageData) {
	labelStyled := func(s string) string { return ui.ColorBold + s + ui.ColorReset }
	valStyled := func(s string) string { return ui.ColorWhite + s + ui.ColorReset }

	// 1. Define the rows structure and populate data
	// We do this first so we can iterate over it to find the max width
	rows := []struct {
		Label string
		Value string
	}{
		{"URL", data.URL},
		{"Status", fmt.Sprintf("%d", data.StatusCode)},
		{"Title", data.Title},
		{"Response Time", fmt.Sprintf("%dms", data.ResponseTime)},
		{"Links", fmt.Sprintf("%d", len(data.Links))},
		{"Images", fmt.Sprintf("%d", len(data.Images))},
		{"Scripts", fmt.Sprintf("%d", len(data.Scripts))},
	}

	// 2. Calculate the maximum label width dynamically
	var maxLen int
	for _, r := range rows {
		if len(r.Label) > maxLen {
			maxLen = len(r.Label)
		}
	}

	// 3. Print with alignment
	fmt.Printf("\n")
	for _, r := range rows {
		// Calculate padding needed to reach maxLen
		pad := strings.Repeat(" ", maxLen-len(r.Label))

		// Print: Label + Padding + " : " + Value
		fmt.Printf("%s%s : %s\n", labelStyled(r.Label), pad, valStyled(r.Value))
	}
	fmt.Printf("\n")
}

// terminalHyperlink returns an OSC 8 hyperlink if supported, falling back to plain path
func terminalHyperlink(label, target string) string {
	abs, err := filepath.Abs(target)
	if err != nil {
		abs = target
	}
	// OSC 8 hyperlink: ESC ] 8 ;; url BEL label ESC ] 8 ;; BEL
	// Use file:// scheme for local files
	return fmt.Sprintf("\x1b]8;;file://%s\x1b\\%s\x1b]8;;\x1b\\", abs, label)
}

func printOutput(data *models.PageData) error {
	// If JSON output is requested
	if jsonOutput {
		// Create a copy to avoid modifying the original data
		exportData := *data
		exportData.HTML = "" // Remove HTML from JSON export
		urlutil.ResolveRelativeLinks(&exportData)
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(exportData)
	}

	// If selector was used, print just the content
	if selector != "" && selector != "body" {
		fmt.Println(data.Content)
		return nil
	}

	// Otherwise, print a summary with colors
	printMetadataSummary(data)

	// Print content preview (first 500 chars) with subtle formatting
	contentPreview := data.Content
	if len(contentPreview) > 500 {
		contentPreview = contentPreview[:500] + "..."
	}
	fmt.Printf("%s\n%s\n\n", ui.ColorBold+"Content Preview:", ui.ColorWhite+contentPreview+ui.ColorReset)

	// Helpful hint for saving to a file
	fmt.Printf("%s\n", ui.Info("Use --output=<file> to save to a specific format (available: .json, .txt, .html, .csv, .md)"))
	fmt.Printf("\n")

	return nil
}
