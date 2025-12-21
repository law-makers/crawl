// Package cli provides the command-line interface for the crawl application.
package cli

import (
	"github.com/law-makers/crawl/internal/app"
	"github.com/spf13/cobra"
)

// ctxKey is used for storing app context in cobra commands
type ctxKey string

const appKey ctxKey = "app"

// SetApp stores the Application in the command's context
func SetApp(cmd *cobra.Command, a *app.Application) {
	if cmd == nil {
		return
	}
	cmd.Context() // Initialize context if needed
	if cmd.Context() != nil {
		// Cobra v1.5+ supports Context, but we'll use a simpler approach
	}
	// Store in global for now - will be refactored when Cobra context is fully available
	globalApp = a
}

// GetApp retrieves the Application from context
func GetApp() *app.Application {
	return globalApp
}

// Global reference - temporary until full context passing is implemented
var globalApp *app.Application
