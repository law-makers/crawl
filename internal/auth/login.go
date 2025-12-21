// internal/auth/login.go
package auth

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/rs/zerolog/log"
)

// LoginOptions configures the interactive login behavior
type LoginOptions struct {
	// SessionName is the name to save the session as
	SessionName string
	// URL to navigate to for login
	URL string
	// WaitSelector is the CSS selector to wait for after login (e.g., "#dashboard")
	WaitSelector string
	// Timeout for the entire login process
	Timeout time.Duration
	// CustomHeaders to send with requests
	Headers map[string]string
	// RemoteDebuggingPort enables Chrome DevTools on this port (e.g., 9222)
	RemoteDebuggingPort int
}

// InteractiveLogin launches a visible browser for manual login
func InteractiveLogin(opts LoginOptions) (*SessionData, error) {
	if opts.SessionName == "" {
		return nil, fmt.Errorf("session name is required")
	}
	if opts.URL == "" {
		return nil, fmt.Errorf("URL is required")
	}
	if opts.Timeout == 0 {
		opts.Timeout = 5 * time.Minute // Default 5 minutes
	}

	// Check if DISPLAY is available (required for visible browser)
	display := os.Getenv("DISPLAY")
	if display == "" {
		return nil, fmt.Errorf("interactive login requires a display server (DISPLAY not set)\n\n" +
			"💡 In headless environments (Codespaces, cloud IDEs), use:\n" +
			"   crawl sessions import <name> --url=<url>\n\n" +
			"   This allows you to import cookies from your browser's DevTools.")
	}

	log.Info().
		Str("session", opts.SessionName).
		Str("url", opts.URL).
		Msg("Starting interactive login")

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	// Create allocator with visible browser
	allocOpts := []chromedp.ExecAllocatorOption{
		chromedp.ExecPath("/usr/bin/google-chrome-stable"),
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("headless", false), // Visible browser
		chromedp.Flag("disable-gpu", false),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("log-level", "3"),
		chromedp.WindowSize(1280, 720),
	}

	// Add remote debugging if port specified
	if opts.RemoteDebuggingPort > 0 {
		allocOpts = append(allocOpts,
			chromedp.Flag("remote-debugging-port", fmt.Sprintf("%d", opts.RemoteDebuggingPort)),
			chromedp.Flag("remote-debugging-address", "0.0.0.0"),
		)
		log.Info().Int("port", opts.RemoteDebuggingPort).Msg("Remote debugging enabled")
		fmt.Printf("\n🔧 Remote debugging enabled on port %d\n", opts.RemoteDebuggingPort)
		fmt.Printf("   1. Forward port %d in VS Code (Ports tab)\n", opts.RemoteDebuggingPort)
		fmt.Printf("   2. Open chrome://inspect in your LOCAL Chrome\n")
		fmt.Printf("   3. Configure target: localhost:%d\n", opts.RemoteDebuggingPort)
		fmt.Printf("   4. Click 'inspect' on the page to interact\n")
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, allocOpts...)
	defer allocCancel()

	// Create browser context
	browserCtx, browserCancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	defer browserCancel()

	// Navigate to login page
	log.Info().Msg("Opening browser for login...")
	fmt.Println("\n🌐 Browser opened. Please complete the login process manually.")
	fmt.Println("   The browser will close automatically once you're logged in.")

	var cookies []*network.Cookie
	err := chromedp.Run(browserCtx,
		network.Enable(),
		chromedp.Navigate(opts.URL),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to navigate: %w", err)
	}

	// Wait for user to login
	if opts.WaitSelector != "" {
		log.Info().Str("selector", opts.WaitSelector).Msg("Waiting for login completion...")
		fmt.Printf("   Waiting for element: %s\n", opts.WaitSelector)

		err = chromedp.Run(browserCtx,
			chromedp.WaitVisible(opts.WaitSelector, chromedp.ByQuery),
		)

		if err != nil {
			return nil, fmt.Errorf("login timeout or failed: %w", err)
		}
	} else {
		// If no selector specified, wait for user confirmation
		fmt.Println("\n   Press Enter once you have completed login...")
		fmt.Scanln()
	}

	log.Info().Msg("Login completed, extracting cookies...")

	// Extract all cookies
	err = chromedp.Run(browserCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			cookies, err = network.GetCookies().Do(ctx)
			return err
		}),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to extract cookies: %w", err)
	}

	if len(cookies) == 0 {
		return nil, fmt.Errorf("no cookies found - login may have failed")
	}

	log.Info().Int("cookie_count", len(cookies)).Msg("Cookies extracted")
	fmt.Printf("\n✓ Successfully captured %d cookies\n", len(cookies))

	// Convert chromedp cookies to our format
	sessionCookies := make([]Cookie, len(cookies))
	for i, c := range cookies {
		sessionCookies[i] = Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  c.Expires,
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
			SameSite: string(c.SameSite),
		}
	}

	// Create session
	session := &SessionData{
		Name:      opts.SessionName,
		URL:       opts.URL,
		Cookies:   sessionCookies,
		Headers:   opts.Headers,
		CreatedAt: time.Now(),
	}

	// Calculate expiration based on cookies
	if len(cookies) > 0 {
		maxExpires := 0.0
		for _, c := range cookies {
			if c.Expires > maxExpires {
				maxExpires = c.Expires
			}
		}
		if maxExpires > 0 {
			session.ExpiresAt = time.Unix(int64(maxExpires), 0)
		}
	}

	return session, nil
}
