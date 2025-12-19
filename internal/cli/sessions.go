// internal/cli/sessions.go
package cli

import (
	"fmt"
	"time"

	"github.com/law-makers/crawl/internal/auth"
	"github.com/spf13/cobra"
)

// sessionsCmd represents the sessions command
var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Manage saved authentication sessions",
	Long: `List, view, and delete saved authentication sessions.

Sessions are stored securely in your OS keyring and contain cookies
and authentication data for accessing protected content.`,
	Example: `  # List all saved sessions
  $ crawl sessions list

  # View details of a specific session
  $ crawl sessions view github-session

  # Delete a session
  $ crawl sessions delete old-session`,
}

var sessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all saved sessions",
	RunE:  runSessionsList,
}

var sessionsViewCmd = &cobra.Command{
	Use:   "view <session-name>",
	Short: "View details of a saved session",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionsView,
}

var sessionsDeleteCmd = &cobra.Command{
	Use:   "delete <session-name>",
	Short: "Delete a saved session",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionsDelete,
}

func init() {
	rootCmd.AddCommand(sessionsCmd)
	sessionsCmd.AddCommand(sessionsListCmd)
	sessionsCmd.AddCommand(sessionsViewCmd)
	sessionsCmd.AddCommand(sessionsDeleteCmd)
}

func runSessionsList(cmd *cobra.Command, args []string) error {
	sessions, err := auth.ListSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("\nNo saved sessions found.")
		fmt.Println("\nCreate a session with:")
		fmt.Println("  crawl login <url> --session=<name>")
		fmt.Println()
		return nil
	}

	fmt.Printf("\n📋 Saved Sessions (%d)\n", len(sessions))
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	for i, name := range sessions {
		fmt.Printf("%d. %s\n", i+1, name)

		// Try to load session details
		session, err := auth.LoadSession(name)
		if err != nil {
			fmt.Printf("   ⚠️  Error loading: %v\n", err)
			continue
		}

		fmt.Printf("   URL: %s\n", session.URL)
		fmt.Printf("   Cookies: %d\n", len(session.Cookies))
		fmt.Printf("   Created: %s\n", session.CreatedAt.Format(time.RFC1123))

		if !session.ExpiresAt.IsZero() {
			if time.Now().After(session.ExpiresAt) {
				fmt.Printf("   Status: ⚠️  Expired (%s ago)\n", time.Since(session.ExpiresAt).Round(time.Hour))
			} else {
				fmt.Printf("   Expires: %s (in %s)\n",
					session.ExpiresAt.Format(time.RFC1123),
					time.Until(session.ExpiresAt).Round(time.Hour))
			}
		}

		if i < len(sessions)-1 {
			fmt.Println()
		}
	}

	fmt.Println()
	return nil
}

func runSessionsView(cmd *cobra.Command, args []string) error {
	name := args[0]

	session, err := auth.LoadSession(name)
	if err != nil {
		return fmt.Errorf("failed to load session '%s': %w", name, err)
	}

	fmt.Printf("\n🔍 Session Details: %s\n", name)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	fmt.Printf("Name:     %s\n", session.Name)
	fmt.Printf("URL:      %s\n", session.URL)
	fmt.Printf("Created:  %s\n", session.CreatedAt.Format(time.RFC1123))

	if !session.ExpiresAt.IsZero() {
		fmt.Printf("Expires:  %s\n", session.ExpiresAt.Format(time.RFC1123))
		if time.Now().After(session.ExpiresAt) {
			fmt.Printf("Status:   ⚠️  Expired\n")
		} else {
			fmt.Printf("Status:   ✓ Valid (expires in %s)\n", time.Until(session.ExpiresAt).Round(time.Hour))
		}
	}

	fmt.Printf("\nCookies (%d):\n", len(session.Cookies))
	for i, cookie := range session.Cookies {
		if i >= 5 {
			fmt.Printf("  ... and %d more\n", len(session.Cookies)-5)
			break
		}
		fmt.Printf("  • %s (domain: %s)\n", cookie.Name, cookie.Domain)
	}

	if len(session.Headers) > 0 {
		fmt.Printf("\nCustom Headers (%d):\n", len(session.Headers))
		for key, value := range session.Headers {
			fmt.Printf("  • %s: %s\n", key, value)
		}
	}

	fmt.Println()
	return nil
}

func runSessionsDelete(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Confirm deletion
	fmt.Printf("\n⚠️  Delete session '%s'? [y/N]: ", name)
	var confirm string
	fmt.Scanln(&confirm)

	if confirm != "y" && confirm != "Y" {
		fmt.Println("Cancelled.")
		return nil
	}

	err := auth.DeleteSessionWithManifest(name)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	fmt.Printf("\n✓ Session '%s' deleted successfully.\n\n", name)
	return nil
}
