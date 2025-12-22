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

// SetApp stores the Application in the command's context.
// Calls should prefer retrieving the app from the specific command's context
// using `GetAppFromCmd(cmd)` when possible.
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
}

// GetAppFromCmd retrieves the Application stored in the provided command's context.
func GetAppFromCmd(cmd *cobra.Command) *app.Application {
	if cmd == nil || cmd.Context() == nil {
		return nil
	}
	if v := cmd.Context().Value(appKey); v != nil {
		if a, ok := v.(*app.Application); ok {
			return a
		}
	}
	return nil
}

// GetApp retrieves the Application from the root command's context (convenience wrapper).
func GetApp() *app.Application {
	return GetAppFromCmd(rootCmd)
}
