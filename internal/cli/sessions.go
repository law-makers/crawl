// internal/cli/sessions.go
package cli

import (
	"fmt"
	"time"

	"github.com/law-makers/crawl/internal/auth"
	"github.com/law-makers/crawl/internal/ui"
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
		fmt.Println("\n" + ui.Info("No saved sessions found."))
		fmt.Println("\n" + ui.Bold("Create a session with:"))
		fmt.Println("  " + ui.ColorCyan + "crawl login <url> --session=<name>" + ui.ColorReset)
		fmt.Println()
		return nil
	}

	fmt.Printf("\n%s %d\n", ui.Bold("📋 Saved Sessions"), len(sessions))
	fmt.Println(ui.ColorDim + "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" + ui.ColorReset)
	fmt.Println()

	for i, name := range sessions {
		fmt.Printf("%d. %s\n", i+1, ui.ColorWhite+name+ui.ColorReset)

		// Try to load session details
		session, err := auth.LoadSession(name)
		if err != nil {
			fmt.Printf("   %s %v\n", ui.Info("⚠️ Error loading:"), err)
			continue
		}

		fmt.Printf("   %s %s\n", ui.ColorBold+"URL:"+ui.ColorReset, ui.ColorWhite+session.URL+ui.ColorReset)
		fmt.Printf("   %s %s\n", ui.ColorBold+"Cookies:"+ui.ColorReset, ui.ColorWhite+fmt.Sprintf("%d", len(session.Cookies))+ui.ColorReset)
		fmt.Printf("   %s %s\n", ui.ColorBold+"Created:"+ui.ColorReset, ui.ColorWhite+session.CreatedAt.Format(time.RFC1123)+ui.ColorReset)

		if !session.ExpiresAt.IsZero() {
			if time.Now().After(session.ExpiresAt) {
				fmt.Printf("   %s %s\n", ui.Info("Status:"), ui.Error("⚠️ Expired"))
			} else {
				fmt.Printf("   %s %s (in %s)\n",
					ui.ColorBold+"Expires:"+ui.ColorReset,
					ui.ColorWhite+session.ExpiresAt.Format(time.RFC1123)+ui.ColorReset,
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

	fmt.Printf("\n%s %s\n", ui.Bold("🔍 Session Details:"), ui.ColorWhite+name+ui.ColorReset)
	fmt.Println(ui.ColorDim + "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" + ui.ColorReset)
	fmt.Println()

	fmt.Printf("%s %s\n", ui.ColorBold+"Name:"+ui.ColorReset, ui.ColorWhite+session.Name+ui.ColorReset)
	fmt.Printf("%s %s\n", ui.ColorBold+"URL:"+ui.ColorReset, ui.ColorWhite+session.URL+ui.ColorReset)
	fmt.Printf("%s %s\n", ui.ColorBold+"Created:"+ui.ColorReset, ui.ColorWhite+session.CreatedAt.Format(time.RFC1123)+ui.ColorReset)

	if !session.ExpiresAt.IsZero() {
		fmt.Printf("%s %s\n", ui.ColorBold+"Expires:"+ui.ColorReset, ui.ColorWhite+session.ExpiresAt.Format(time.RFC1123)+ui.ColorReset)
		if time.Now().After(session.ExpiresAt) {
			fmt.Printf("%s %s\n", ui.Info("Status:"), ui.Error("⚠️ Expired"))
		} else {
			fmt.Printf("%s %s %s\n", ui.ColorBold+"Status:"+ui.ColorReset, ui.Success("✓ Valid"), ui.ColorDim+fmt.Sprintf("(expires in %s)", time.Until(session.ExpiresAt).Round(time.Hour))+ui.ColorReset)
		}
	}

	fmt.Printf("\n%s (%d):\n", ui.ColorBold+"Cookies"+ui.ColorReset, len(session.Cookies))
	for i, cookie := range session.Cookies {
		if i >= 5 {
			fmt.Printf("  %s %s\n", ui.ColorDim+"... and"+ui.ColorReset, ui.ColorWhite+fmt.Sprintf("%d more", len(session.Cookies)-5)+ui.ColorReset)
			break
		}
		fmt.Printf("  • %s (domain: %s)\n", ui.ColorWhite+cookie.Name+ui.ColorReset, ui.ColorWhite+cookie.Domain+ui.ColorReset)
	}

	if len(session.Headers) > 0 {
		fmt.Printf("\n%s (%d):\n", ui.ColorBold+"Custom Headers"+ui.ColorReset, len(session.Headers))
		for key, value := range session.Headers {
			fmt.Printf("  • %s: %s\n", ui.ColorWhite+key+ui.ColorReset, ui.ColorWhite+value+ui.ColorReset)
		}
	}

	fmt.Println()
	return nil
}

func runSessionsDelete(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Confirm deletion
	fmt.Printf("\n%s %s [y/N]: ", ui.Info("⚠️  Delete session"), ui.ColorWhite+name+ui.ColorReset)
	var confirm string
	fmt.Scanln(&confirm)

	if confirm != "y" && confirm != "Y" {
		fmt.Println(ui.Info("Cancelled."))
		return nil
	}

	err := auth.DeleteSessionWithManifest(name)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	fmt.Println(ui.Success(fmt.Sprintf("\n✓ Session '%s' deleted successfully.\n", name)))
	return nil
}
