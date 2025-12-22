// internal/cli/sessions_import.go
package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/law-makers/crawl/internal/auth"
	"github.com/law-makers/crawl/internal/ui"
	"github.com/spf13/cobra"
)

var (
	importURL    string
	importFormat string
)

// sessionsImportCmd represents the sessions import command
var sessionsImportCmd = &cobra.Command{
	Use:   "import <session-name>",
	Short: "Import cookies from your browser to create a session",
	Long: `Import cookies from your browser's developer tools to create an authenticated session.

This is useful in headless environments (Codespaces, dev containers) where the 
interactive login browser doesn't work properly.

Steps:
1. Open the website in your regular browser
2. Login normally
3. Open DevTools (F12) → Application → Cookies
4. Copy the cookies
5. Use this command to import them`,
	Example: `  # Import cookies interactively
  crawl sessions import mysite --url=https://example.com

  # Import from Netscape/curl format file
  crawl sessions import github --url=https://github.com --format=netscape < cookies.txt

  # Import from JSON
  crawl sessions import mysite --url=https://example.com --format=json < cookies.json`,
	Args: cobra.ExactArgs(1),
	RunE: runSessionsImport,
}

func init() {
	sessionsCmd.AddCommand(sessionsImportCmd)

	sessionsImportCmd.Flags().StringVar(&importURL, "url", "", "Website URL for this session (required)")
	sessionsImportCmd.Flags().StringVar(&importFormat, "format", "interactive", "Import format: interactive, json, netscape")
	sessionsImportCmd.MarkFlagRequired("url")
}

func runSessionsImport(cmd *cobra.Command, args []string) error {
	sessionName := args[0]

	fmt.Printf("\n%s %s\n", ui.Bold("🔐 Import Session:"), ui.ColorBold+sessionName+ui.ColorReset)
	fmt.Printf("%s\n\n", ui.ColorDim+"━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"+ui.ColorReset)

	var cookies []auth.Cookie
	var err error

	switch importFormat {
	case "interactive":
		cookies, err = importInteractive()
	case "json":
		cookies, err = importJSON()
	case "netscape":
		cookies, err = importNetscape()
	default:
		return fmt.Errorf("unsupported format: %s (use: interactive, json, netscape)", importFormat)
	}

	if err != nil {
		return fmt.Errorf("failed to import cookies: %w", err)
	}

	if len(cookies) == 0 {
		return fmt.Errorf("no cookies imported")
	}

	// Create session
	session := &auth.SessionData{
		Name:      sessionName,
		URL:       importURL,
		Cookies:   cookies,
		Headers:   make(map[string]string),
		CreatedAt: time.Now(),
	}

	// Find earliest expiry
	var earliestExpiry time.Time
	for _, c := range cookies {
		if c.Expires > 0 {
			expiry := time.Unix(int64(c.Expires), 0)
			if earliestExpiry.IsZero() || expiry.Before(earliestExpiry) {
				earliestExpiry = expiry
			}
		}
	}
	if !earliestExpiry.IsZero() {
		session.ExpiresAt = earliestExpiry
	}

	// Save session
	err = auth.SaveSessionWithManifest(session)
	if err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	fmt.Printf("\n%s\n", ui.Success(fmt.Sprintf("✅ Session '%s' created successfully!", sessionName)))
	fmt.Printf("   %s %s\n", ui.ColorBold+"Cookies:"+ui.ColorReset, ui.ColorWhite+fmt.Sprintf("%d", len(cookies))+ui.ColorReset)
	if !session.ExpiresAt.IsZero() {
		fmt.Printf("   %s %s\n", ui.ColorBold+"Expires:"+ui.ColorReset, ui.ColorWhite+session.ExpiresAt.Format(time.RFC1123)+ui.ColorReset)
	}
	fmt.Printf("\n%s\n", ui.Bold("Use with:"))
	fmt.Printf("  %s %s\n", ui.ColorCyan+"crawl get <url> --session="+ui.ColorReset, ui.ColorWhite+sessionName+ui.ColorReset)
	fmt.Printf("  %s %s\n\n", ui.ColorCyan+"crawl media <url> --session="+ui.ColorReset, ui.ColorWhite+sessionName+ui.ColorReset)

	return nil
}

func importInteractive() ([]auth.Cookie, error) {
	fmt.Println(ui.Bold("📋 Cookie Import Guide:"))
	fmt.Println()
	fmt.Println(ui.Info("1. Open the website in your browser and login"))
	fmt.Println(ui.Info("2. Press F12 to open DevTools"))
	fmt.Println(ui.Info("3. Go to: Application → Storage → Cookies"))
	fmt.Println(ui.Info("4. For each important cookie, copy the Name and Value"))
	fmt.Println()

	var cookies []auth.Cookie
	scanner := bufio.NewScanner(os.Stdin)

	// Extract domain from URL
	domain := ""
	if importURL != "" {
		if strings.Contains(importURL, "github") {
			domain = ".github.com"
		} else if strings.Contains(importURL, "twitter") || strings.Contains(importURL, "x.com") {
			domain = ".twitter.com"
		} else {
			// Extract from URL
			parts := strings.Split(strings.TrimPrefix(strings.TrimPrefix(importURL, "https://"), "http://"), "/")
			if len(parts) > 0 {
				domain = "." + parts[0]
			}
		}
	}

	for {
		fmt.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Print("\nCookie Name (or press Enter to finish): ")
		if !scanner.Scan() {
			break
		}
		name := strings.TrimSpace(scanner.Text())
		if name == "" {
			break
		}

		fmt.Print("Cookie Value: ")
		if !scanner.Scan() {
			break
		}
		value := strings.TrimSpace(scanner.Text())
		if value == "" {
			fmt.Println(ui.Info("⚠️  Skipping cookie with empty value"))
			continue
		}

		// Ask for domain with default
		fmt.Printf("Domain [%s]: ", domain)
		if !scanner.Scan() {
			break
		}
		cookieDomain := strings.TrimSpace(scanner.Text())
		if cookieDomain == "" {
			cookieDomain = domain
		}

		// Create cookie
		cookie := auth.Cookie{
			Name:     name,
			Value:    value,
			Domain:   cookieDomain,
			Path:     "/",
			Secure:   true,
			HTTPOnly: true,
		}

		cookies = append(cookies, cookie)
		fmt.Printf("%s %s (domain: %s)\n", ui.Success("✅ Added:"), ui.ColorWhite+cookie.Name+ui.ColorReset, ui.ColorWhite+cookie.Domain+ui.ColorReset)
	}

	if len(cookies) == 0 {
		fmt.Println("\n" + ui.Info("⚠️  No cookies added"))
	} else {
		fmt.Println("\n" + ui.Success(fmt.Sprintf("✅ Total cookies added: %d", len(cookies))))
	}

	return cookies, nil
}

func importJSON() ([]auth.Cookie, error) {
	var cookies []auth.Cookie
	decoder := json.NewDecoder(os.Stdin)
	err := decoder.Decode(&cookies)
	if err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return cookies, nil
}

func importNetscape() ([]auth.Cookie, error) {
	var cookies []auth.Cookie
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}

		cookie := auth.Cookie{
			Domain:   fields[0],
			Path:     fields[2],
			Secure:   fields[3] == "TRUE",
			Name:     fields[5],
			Value:    fields[6],
			HTTPOnly: false,
		}

		if fields[4] != "0" {
			if expiry, err := time.Parse("2006-01-02", fields[4]); err == nil {
				cookie.Expires = float64(expiry.Unix())
			}
		}

		cookies = append(cookies, cookie)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return cookies, nil
}
