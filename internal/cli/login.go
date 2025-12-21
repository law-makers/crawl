// internal/cli/login.go
package cli

import (
	"fmt"
	"time"

	"github.com/law-makers/crawl/internal/auth"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	waitSelector        string
	loginTimeout        string
	remoteDebuggingPort int
)

// loginCmd represents the login command
var loginCmd = &cobra.Command{
	Use:   "login <url>",
	Short: "Interactively login to a website and save the session",
	Long: `Opens a visible browser window for you to manually log in to a website.
After successful login, cookies are extracted and securely stored in your OS keyring.

The stored session can then be used with other commands (get, media) to access
authenticated content without logging in again.

For headless environments (dev containers), use --remote-debug to access the browser
via web interface on a forwarded port.`,
	Example: `  # Login to GitHub and save as "github-session"
  $ crawl login https://github.com/login --session=github-session --wait="#dashboard"

  # Login in dev container with remote debugging
  $ crawl login https://example.com/login --session=example --remote-debug=9222

  # Login without waiting for specific element (manual confirmation)
  $ crawl login https://example.com/login --session=example

  # Use the saved session
  $ crawl get https://github.com/settings/profile --session=github-session`,
	Args: cobra.ExactArgs(1),
	RunE: runLogin,
}

func init() {
	rootCmd.AddCommand(loginCmd)

	loginCmd.Flags().StringVarP(&sessionName, "session", "s", "", "Session name to save (required)")
	loginCmd.Flags().StringVarP(&waitSelector, "wait", "w", "", "CSS selector to wait for after login (e.g., '#dashboard')")
	loginCmd.Flags().StringVar(&loginTimeout, "login-timeout", "5m", "Timeout for login process")
	loginCmd.Flags().IntVar(&remoteDebuggingPort, "remote-debug", 0, "Enable Chrome remote debugging on this port (e.g., 9222)")
	loginCmd.MarkFlagRequired("session")
}

func runLogin(cmd *cobra.Command, args []string) error {
	url := args[0]

	// Parse timeout
	timeout, err := time.ParseDuration(loginTimeout)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}

	log.Info().
		Str("url", url).
		Str("session", sessionName).
		Msg("Initiating login")

	fmt.Printf("\n🔐 Interactive Login\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
	fmt.Printf("  Session:  %s\n", sessionName)
	fmt.Printf("  URL:      %s\n", url)
	if waitSelector != "" {
		fmt.Printf("  Waiting:  %s\n", waitSelector)
	}
	fmt.Printf("  Timeout:  %s\n\n", timeout)

	// Perform interactive login
	opts := auth.LoginOptions{
		SessionName:         sessionName,
		URL:                 url,
		WaitSelector:        waitSelector,
		Timeout:             timeout,
		RemoteDebuggingPort: remoteDebuggingPort,
	}

	session, err := auth.InteractiveLogin(opts)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	// Save session to keyring
	log.Info().Msg("Saving session to keyring")
	err = auth.SaveSessionWithManifest(session)
	if err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	fmt.Printf("\n✓ Session saved successfully!\n")
	fmt.Printf("\nYou can now use this session with:\n")
	fmt.Printf("  crawl get <url> --session=%s\n", sessionName)
	fmt.Printf("  crawl media <url> --session=%s\n\n", sessionName)

	// Show expiration if available
	if !session.ExpiresAt.IsZero() {
		fmt.Printf("Session expires: %s\n\n", session.ExpiresAt.Format(time.RFC1123))
	}

	return nil
}
