// internal/cli/get.go
package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/JohannesKaufmann/html-to-markdown/plugin"
	"github.com/PuerkitoBio/goquery"
	"github.com/law-makers/crawl/internal/engine"
	"github.com/law-makers/crawl/pkg/models"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"golang.org/x/net/html"
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

// validateURL performs comprehensive URL validation
func validateURL(urlStr string) error {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("invalid URL scheme: must be http or https, got %s", parsed.Scheme)
	}

	if parsed.Host == "" {
		return fmt.Errorf("invalid URL: missing host")
	}

	return nil
}

func runGet(cmd *cobra.Command, args []string) error {
	url := args[0]

	// Validate URL
	if err := validateURL(url); err != nil {
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
	headerMap := make(map[string]string)
	for _, h := range headers {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) == 2 {
			headerMap[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

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

	// Create scraper based on mode
	var scraper engine.Scraper

	// Get app from context
	appCtx := GetApp()
	if appCtx == nil {
		return fmt.Errorf("application not initialized")
	}

	// Use the scraper from the app
	scraper = appCtx.Scraper

	// Override if static mode is requested
	switch scraperMode {
	case models.ModeStatic:
		log.Debug().Msg("Using StaticScraper")
	case models.ModeSPA:
		log.Debug().Msg("Using DynamicScraper (headless Chrome)")
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

func saveOutput(data *models.PageData, filepath string) error {
	var content []byte
	var err error

	switch {
	case strings.HasSuffix(filepath, ".json"):
		// Create a copy to avoid modifying the original data
		exportData := *data
		exportData.HTML = "" // Remove HTML from JSON export
		resolveRelativeLinks(&exportData)
		content, err = json.MarshalIndent(exportData, "", "  ")
	case strings.HasSuffix(filepath, ".html"):
		cleaned, cleanErr := cleanHTML(data.HTML)
		if cleanErr != nil {
			return fmt.Errorf("failed to clean HTML: %w", cleanErr)
		}
		content = []byte(cleaned)
	case strings.HasSuffix(filepath, ".txt"):
		content = []byte(data.Content)
	case strings.HasSuffix(filepath, ".csv"):
		return saveCSV(data, filepath)
	case strings.HasSuffix(filepath, ".md"):
		return saveMarkdown(data, filepath)
	default:
		// Default to JSON
		// Create a copy to avoid modifying the original data
		exportData := *data
		exportData.HTML = "" // Remove HTML from JSON export
		resolveRelativeLinks(&exportData)
		content, err = json.MarshalIndent(exportData, "", "  ")
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

func saveCSV(data *models.PageData, filepath string) error {
	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// If we have structured data (from --fields), use that
	if len(data.Structured) > 0 {
		// Get headers from the first item
		var headers []string
		firstItem := data.Structured[0]
		for k := range firstItem {
			headers = append(headers, k)
		}
		sort.Strings(headers)

		if err := writer.Write(headers); err != nil {
			return err
		}

		for _, item := range data.Structured {
			var row []string
			for _, h := range headers {
				row = append(row, item[h])
			}
			if err := writer.Write(row); err != nil {
				return err
			}
		}
	} else if len(data.Data) > 0 {
		// If we have list data but no fields, just dump Text and HTML
		if err := writer.Write([]string{"Text", "HTML"}); err != nil {
			return err
		}
		for _, item := range data.Data {
			if err := writer.Write([]string{item.Text, item.HTML}); err != nil {
				return err
			}
		}
	} else {
		// Fallback for single page content
		if err := writer.Write([]string{"Content", "HTML"}); err != nil {
			return err
		}
		if err := writer.Write([]string{data.Content, data.HTML}); err != nil {
			return err
		}
	}

	log.Info().Str("file", filepath).Msg("Output saved")
	fmt.Printf("✓ Saved to %s\n", filepath)
	return nil
}

func saveMarkdown(data *models.PageData, filepath string) error {
	converter := md.NewConverter("", true, nil)
	converter.Use(plugin.GitHubFlavored())

	// Add rule to resolve relative links
	converter.AddRules(md.Rule{
		Filter: []string{"a"},
		Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
			href, exists := selec.Attr("href")
			if !exists {
				return nil
			}

			resolved := resolveURL(data.URL, href)
			title, hasTitle := selec.Attr("title")
			var titlePart string
			if hasTitle {
				titlePart = fmt.Sprintf(" %q", title)
			}

			// Clean up content: replace newlines with spaces and trim
			cleanContent := strings.Join(strings.Fields(content), " ")
			link := fmt.Sprintf("[%s](%s%s)", cleanContent, resolved, titlePart)
			return &link
		},
	})

	// Add rule to resolve relative images
	converter.AddRules(md.Rule{
		Filter: []string{"img"},
		Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
			src, exists := selec.Attr("src")
			if !exists {
				return nil
			}

			resolved := resolveURL(data.URL, src)
			alt, _ := selec.Attr("alt")
			title, hasTitle := selec.Attr("title")
			var titlePart string
			if hasTitle {
				titlePart = fmt.Sprintf(" %q", title)
			}

			link := fmt.Sprintf("![%s](%s%s)", alt, resolved, titlePart)
			return &link
		},
	})

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Scrape Result: %s\n\n", data.Title))
	sb.WriteString(fmt.Sprintf("**URL:** %s  \n", data.URL))
	sb.WriteString(fmt.Sprintf("**Date:** %s\n\n", data.FetchedAt.Format(time.RFC1123)))

	if len(data.Structured) > 0 {
		// Create a table for structured data
		// Get headers
		var headers []string
		if len(data.Structured) > 0 {
			for k := range data.Structured[0] {
				headers = append(headers, k)
			}
			sort.Strings(headers)
		}

		// Header row
		sb.WriteString("| " + strings.Join(headers, " | ") + " |\n")
		// Separator row
		sb.WriteString("|" + strings.Repeat("---|", len(headers)) + "\n")

		// Data rows
		for _, item := range data.Structured {
			var row []string
			for _, h := range headers {
				// Escape pipes in content
				val := strings.ReplaceAll(item[h], "|", "\\|")
				// Replace newlines with space to keep table structure
				val = strings.ReplaceAll(val, "\n", " ")
				row = append(row, val)
			}
			sb.WriteString("| " + strings.Join(row, " | ") + " |\n")
		}
		sb.WriteString("\n")
	} else if len(data.Data) > 0 {
		// List of items
		for _, item := range data.Data {
			markdown, err := converter.ConvertString(item.HTML)
			if err != nil {
				// Fallback to text if conversion fails
				markdown = item.Text
			}
			sb.WriteString(fmt.Sprintf("- %s\n", markdown))
		}
		sb.WriteString("\n")
	} else {
		// Full page content
		markdown, err := converter.ConvertString(data.HTML)
		if err != nil {
			return fmt.Errorf("failed to convert HTML to Markdown: %w", err)
		}
		sb.WriteString(markdown)
		sb.WriteString("\n")
	}

	err := os.WriteFile(filepath, []byte(sb.String()), 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	log.Info().Str("file", filepath).Msg("Output saved")
	fmt.Printf("✓ Saved to %s\n", filepath)
	return nil
}

func printOutput(data *models.PageData) error {
	// If JSON output is requested
	if jsonOutput {
		// Create a copy to avoid modifying the original data
		exportData := *data
		exportData.HTML = "" // Remove HTML from JSON export
		resolveRelativeLinks(&exportData)
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(exportData)
	}

	// If selector was used, print just the content
	if selector != "" && selector != "body" {
		fmt.Println(data.Content)
		return nil
	}

	// Otherwise, print a summary with JSON
	fmt.Printf("\n")
	fmt.Printf("URL:           %s\n", data.URL)
	fmt.Printf("Status:        %d\n", data.StatusCode)
	fmt.Printf("Title:         %s\n", data.Title)
	fmt.Printf("Response Time: %dms\n", data.ResponseTime)
	fmt.Printf("Links:         %d\n", len(data.Links))
	fmt.Printf("Images:        %d\n", len(data.Images))
	fmt.Printf("Scripts:       %d\n", len(data.Scripts))
	fmt.Printf("\n")

	// Print content preview (first 500 chars)
	contentPreview := data.Content
	if len(contentPreview) > 500 {
		contentPreview = contentPreview[:500] + "..."
	}
	fmt.Printf("Content Preview:\n%s\n", contentPreview)

	return nil
}

func resolveRelativeLinks(data *models.PageData) {
	// Resolve Links
	resolvedLinks := make([]string, len(data.Links))
	for i, link := range data.Links {
		resolvedLinks[i] = resolveURL(data.URL, link)
	}
	data.Links = resolvedLinks

	// Resolve Images
	resolvedImages := make([]string, len(data.Images))
	for i, img := range data.Images {
		resolvedImages[i] = resolveURL(data.URL, img)
	}
	data.Images = resolvedImages

	// Resolve Scripts
	resolvedScripts := make([]string, len(data.Scripts))
	for i, script := range data.Scripts {
		resolvedScripts[i] = resolveURL(data.URL, script)
	}
	data.Scripts = resolvedScripts
}

func resolveURL(base, href string) string {
	u, err := url.Parse(href)
	if err != nil {
		return href
	}
	if u.IsAbs() {
		return href
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return href
	}
	return baseURL.ResolveReference(u).String()
}

func cleanHTML(htmlContent string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return "", err
	}

	// Remove unwanted tags
	doc.Find("script, style, link, meta, noscript, iframe, svg, form, input, button, select, textarea, canvas").Remove()

	// Clean attributes
	doc.Find("*").Each(func(i int, s *goquery.Selection) {
		if len(s.Nodes) == 0 {
			return
		}
		node := s.Nodes[0]
		var newAttrs []html.Attribute
		for _, attr := range node.Attr {
			keep := false
			switch node.Data {
			case "a":
				if attr.Key == "href" || attr.Key == "title" {
					keep = true
				}
			case "img":
				if attr.Key == "src" || attr.Key == "alt" || attr.Key == "title" {
					keep = true
				}
			}
			if attr.Key == "colspan" || attr.Key == "rowspan" {
				keep = true
			}
			if keep {
				newAttrs = append(newAttrs, attr)
			}
		}
		node.Attr = newAttrs
	})

	// Get the html node
	var root *html.Node
	if len(doc.Nodes) > 0 {
		root = doc.Nodes[0]
	} else {
		return "", fmt.Errorf("empty document")
	}

	return prettyPrint(root), nil
}

func prettyPrint(n *html.Node) string {
	var sb strings.Builder
	var f func(*html.Node, int)
	f = func(n *html.Node, depth int) {
		indent := strings.Repeat("  ", depth)
		switch n.Type {
		case html.DocumentNode:
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				f(c, depth)
			}
		case html.ElementNode:
			sb.WriteString(fmt.Sprintf("%s<%s", indent, n.Data))
			for _, a := range n.Attr {
				sb.WriteString(fmt.Sprintf(" %s=\"%s\"", a.Key, a.Val))
			}
			if isVoidElement(n.Data) {
				sb.WriteString(">\n")
				return
			}
			sb.WriteString(">\n")

			for c := n.FirstChild; c != nil; c = c.NextSibling {
				f(c, depth+1)
			}
			sb.WriteString(fmt.Sprintf("%s</%s>\n", indent, n.Data))
		case html.TextNode:
			text := strings.TrimSpace(n.Data)
			if text != "" {
				sb.WriteString(fmt.Sprintf("%s%s\n", indent, text))
			}
		case html.DoctypeNode:
			sb.WriteString(fmt.Sprintf("<!DOCTYPE %s>\n", n.Data))
		}
	}
	f(n, 0)
	return sb.String()
}

func isVoidElement(tag string) bool {
	switch tag {
	case "area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr":
		return true
	}
	return false
}
