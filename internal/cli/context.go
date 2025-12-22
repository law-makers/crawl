// Package cli provides the command-line interface for the crawl application.
package cli

import (
	"context"

	"github.com/law-makers/crawl/internal/app"
	"github.com/spf13/cobra"
)

// ctxKey is used for storing app context in cobra commands
type ctxKey string

const appKey ctxKey = "app"

// SetApp stores the Application in the command's context and keeps a global fallback.
// This uses the `appKey` context key so callers that can access a Cobra command's
// context can retrieve the app without relying on global state.
func SetApp(cmd *cobra.Command, a *app.Application) {
	if cmd == nil {
		return
	}

	// Ensure a non-nil context is present and store the app in it
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	cmd.SetContext(context.WithValue(ctx, appKey, a))

	// Keep global fallback for existing call sites
	globalApp = a
}

// GetApp retrieves the Application from the root command's context if available,
// otherwise falls back to the global variable.
func GetApp() *app.Application {
	// Prefer context-based app if present on the root command
	if rootCmd != nil && rootCmd.Context() != nil {
		if v := rootCmd.Context().Value(appKey); v != nil {
			if a, ok := v.(*app.Application); ok {
				return a
			}
		}
	}

	return globalApp
}

// Global reference - temporary until full context passing is implemented
var globalApp *app.Application
