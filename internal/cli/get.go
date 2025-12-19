// internal/cli/get.go
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/law-makers/crawl/internal/engine"
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
	getCmd.Flags().StringVarP(&output, "output", "o", "", "File path to save output (supports .json, .txt, .html)")
	getCmd.Flags().StringArrayVarP(&headers, "header", "H", []string{}, "Custom headers (e.g., -H \"User-Agent: Bot\")")
	getCmd.Flags().StringVar(&sessionName, "session", "", "Name of a saved auth session to use")
}

func runGet(cmd *cobra.Command, args []string) error {
	url := args[0]

	// Validate URL
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("invalid URL: must start with http:// or https://")
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
	headerMap := make(map[string]string)
	for _, h := range headers {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) == 2 {
			headerMap[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	// Build request options
	opts := models.RequestOptions{
		URL:         url,
		Mode:        scraperMode,
		Selector:    selector,
		Headers:     headerMap,
		SessionName: sessionName,
		Timeout:     30 * time.Second,
		Proxy:       proxy,
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

	// Create scraper based on mode
	var scraper engine.Scraper
	switch scraperMode {
	case models.ModeStatic, models.ModeAuto:
		scraper = engine.NewStaticScraper()
		log.Debug().Msg("Using StaticScraper")
	case models.ModeSPA:
		scraper = engine.NewDynamicScraper()
		log.Debug().Msg("Using DynamicScraper (headless Chrome)")
	}

	// Fetch data
	log.Info().Str("url", url).Str("mode", string(scraperMode)).Msg("Fetching URL")
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

func saveOutput(data *models.PageData, filepath string) error {
	var content []byte
	var err error

	switch {
	case strings.HasSuffix(filepath, ".json"):
		content, err = json.MarshalIndent(data, "", "  ")
	case strings.HasSuffix(filepath, ".html"):
		content = []byte(data.HTML)
	case strings.HasSuffix(filepath, ".txt"):
		content = []byte(data.Content)
	default:
		// Default to JSON
		content, err = json.MarshalIndent(data, "", "  ")
	}

	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	err = os.WriteFile(filepath, content, 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	log.Info().Str("file", filepath).Msg("Output saved")
	fmt.Printf("✓ Saved to %s\n", filepath)
	return nil
}

func printOutput(data *models.PageData) error {
	// If selector was used, print just the content
	if selector != "" && selector != "body" {
		fmt.Println(data.Content)
		return nil
	}

	// Otherwise, print a summary with JSON
	fmt.Printf("\n")
	fmt.Printf("URL:          %s\n", data.URL)
	fmt.Printf("Status:       %d\n", data.StatusCode)
	fmt.Printf("Title:        %s\n", data.Title)
	fmt.Printf("Response Time: %dms\n", data.ResponseTime)
	fmt.Printf("Links:        %d\n", len(data.Links))
	fmt.Printf("Images:       %d\n", len(data.Images))
	fmt.Printf("Scripts:      %d\n", len(data.Scripts))
	fmt.Printf("\n")

	// Print content preview (first 500 chars)
	contentPreview := data.Content
	if len(contentPreview) > 500 {
		contentPreview = contentPreview[:500] + "..."
	}
	fmt.Printf("Content Preview:\n%s\n", contentPreview)

	return nil
}
