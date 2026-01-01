// internal/cli/root.go
package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/law-makers/crawl/internal/app"
	"github.com/law-makers/crawl/internal/config"
	"github.com/law-makers/crawl/internal/ui"
)

var (
	verbose    bool
	quiet      bool
	jsonOutput bool
	proxy      string
	timeout    string
	userAgent  string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "crawl",
	Short:   "A fast and cross-platform CLI for scraping websites",
	Long:    `Crawl is a unified data extraction tool designed to scrape static and SPA sites.`,
	Version: "0.1.0",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
// It initializes the application and passes it to all commands.
func Execute() {
	// Execute CLI (application is initialized lazily in PersistentPreRunE)
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Lazily initialize the application before running commands (avoid starting app for -h/help)
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if GetAppFromCmd(cmd) != nil {
			return nil
		}

		cfg, err := config.Load(rootCmd)
		if err != nil {
			log.Warn().Err(err).Msg("failed to load configuration, using defaults")
			cfg = &config.Config{}
		}

		ctx, cancel := context.WithTimeout(context.Background(), cfg.HTTPTimeout*10)
		defer cancel()
		// If initialization fails, cancel immediately
		appCtx, err := app.New(ctx, cfg)
		if err != nil {
			return err
		}

		// Store app in the current command's context for commands to access
		SetApp(cmd, appCtx)
		// Also store on root command for compatibility
		SetApp(rootCmd, appCtx)
		return nil
	}

	// Ensure app is closed after command runs
	rootCmd.PersistentPostRun = func(cmd *cobra.Command, args []string) {
		appCtx := GetAppFromCmd(cmd)
		if appCtx == nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), appCtx.Config.HTTPTimeout*10)
		defer cancel()
		_ = appCtx.Close(ctx)
		// Clear the app from the current command's context and the root command
		SetApp(cmd, nil)
		SetApp(rootCmd, nil)
	}
}

func init() {
	// Register centralized flags
	config.RegisterFlags(rootCmd)
	cobra.OnInitialize(initConfig)

	// Customize help and version flag descriptions
	rootCmd.Flags().BoolP("help", "h", false, "Help for Crawl")
	rootCmd.Flags().Bool("version", false, "Version for Crawl")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// Load configuration (reads flags and env)
	cfg, err := config.Load(rootCmd)
	if err != nil {
		// Fall back to defaults but log the issue
		log.Warn().Err(err).Msg("failed to load configuration, using defaults")
		cfg = &config.Config{}
	}
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		verbose = true
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
		quiet = true
	default:
		// Default to suppressing info logs unless verbose is explicitly requested
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	}

	if cfg.JSONLog {
		log.Logger = zerolog.New(os.Stderr).With().Timestamp().Logger()
		jsonOutput = true
	} else {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	// Populate legacy globals so existing commands work
	userAgent = cfg.UserAgent
	proxy = cfg.Proxy
	timeout = cfg.HTTPTimeout.String()

	log.Debug().Str("user_agent", cfg.UserAgent).Msg("Configuration loaded")
}

// GetUserAgent returns the configured user agent string
func GetUserAgent() string {
	if userAgent != "" {
		return userAgent
	}
	return "Crawl/1.0 (https://github.com/law-makers/crawl)"
}

func init() {
	// Disable the default completion command
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// Set custom help function
	rootCmd.SetHelpFunc(customHelpFunc)
	rootCmd.SetUsageFunc(customUsageFunc)
}

// customHelpFunc provides a colorized help output
func customHelpFunc(cmd *cobra.Command, args []string) {
	// Header with command name
	fmt.Fprintf(os.Stdout, "\n%s%s%s\n", ui.ColorBold+ui.ColorCyan, strings.ToUpper(cmd.Name()), ui.ColorReset)

	// Short description
	if cmd.Short != "" {
		fmt.Fprintf(os.Stdout, "%s\n", cmd.Short)
	}

	// Long description
	if cmd.Long != "" && cmd.Long != cmd.Short {
		fmt.Fprintf(os.Stdout, "\n%s\n", wrapText(cmd.Long, 80))
	}

	// Usage section
	fmt.Fprintf(os.Stdout, "\n%sUsage%s\n", ui.ColorBold+ui.ColorWhite, ui.ColorReset)
	if cmd.Runnable() {
		fmt.Fprintf(os.Stdout, "  %s%s%s\n", ui.ColorCyan, cmd.UseLine(), ui.ColorReset)
	}
	if cmd.HasAvailableSubCommands() {
		fmt.Fprintf(os.Stdout, "  %s%s%s %s<command>%s %s[flags]%s\n",
			ui.ColorCyan, cmd.CommandPath(), ui.ColorReset,
			ui.ColorYellow, ui.ColorReset,
			ui.ColorDim, ui.ColorReset)
	}

	// Examples section
	if cmd.HasExample() {
		fmt.Fprintf(os.Stdout, "\n%sExamples%s\n", ui.ColorBold+ui.ColorWhite, ui.ColorReset)
		examples := strings.Split(cmd.Example, "\n")
		lastWasCommand := false
		for _, example := range examples {
			trimmed := strings.TrimSpace(example)
			if trimmed == "" {
				continue
			}
			if strings.HasPrefix(trimmed, "#") {
				// Add spacing before comment if previous line was a command
				if lastWasCommand {
					fmt.Fprintln(os.Stdout)
				}
				// Comment line
				fmt.Fprintf(os.Stdout, "  %s%s%s\n", ui.ColorDim, trimmed, ui.ColorReset)
				lastWasCommand = false
			} else {
				// Command line
				fmt.Fprintf(os.Stdout, "  %s$ %s%s\n", ui.ColorGreen, trimmed, ui.ColorReset)
			}
		}
	}

	// Available commands section
	if cmd.HasAvailableSubCommands() {
		fmt.Fprintf(os.Stdout, "\n%sCommands%s\n", ui.ColorBold+ui.ColorWhite, ui.ColorReset)

		maxLen := 0
		availableCommands := []*cobra.Command{}
		for _, c := range cmd.Commands() {
			if c.IsAvailableCommand() && c.Name() != "help" {
				availableCommands = append(availableCommands, c)
				if len(c.Name()) > maxLen {
					maxLen = len(c.Name())
				}
			}
		}

		for _, c := range availableCommands {
			padding := strings.Repeat(" ", maxLen-len(c.Name())+2)
			fmt.Fprintf(os.Stdout, "  %s%s%s%s%s%s%s\n",
				ui.ColorCyan, c.Name(), ui.ColorReset,
				padding,
				ui.ColorDim, c.Short, ui.ColorReset)
		}
	}

	// Flags sections
	hasLocalFlags := cmd.HasAvailableLocalFlags()
	hasInheritedFlags := cmd.HasAvailableInheritedFlags()

	if hasLocalFlags {
		fmt.Fprintf(os.Stdout, "\n%sFlags%s\n", ui.ColorBold+ui.ColorWhite, ui.ColorReset)
		printFlags(cmd.LocalFlags().FlagUsages())
	}

	if hasInheritedFlags {
		fmt.Fprintf(os.Stdout, "\n%sGlobal Flags%s\n", ui.ColorBold+ui.ColorWhite, ui.ColorReset)
		printFlags(cmd.InheritedFlags().FlagUsages())
	}

	// Footer
	if cmd.HasAvailableSubCommands() {
		fmt.Fprintf(os.Stdout, "\n%sUse \"%s%s%s %s<command>%s %s--help%s\" for more information about a command.%s\n",
			ui.ColorDim,
			ui.ColorCyan, cmd.CommandPath(), ui.ColorReset+ui.ColorDim,
			ui.ColorYellow, ui.ColorReset+ui.ColorDim,
			ui.ColorGreen, ui.ColorReset+ui.ColorDim,
			ui.ColorReset)
	}
	fmt.Fprintln(os.Stdout)
}

// customUsageFunc provides a colorized usage output
func customUsageFunc(cmd *cobra.Command) error {
	fmt.Fprintf(os.Stderr, "\n%sUsage%s\n", ui.ColorBold+ui.ColorWhite, ui.ColorReset)
	if cmd.Runnable() {
		fmt.Fprintf(os.Stderr, "  %s%s%s\n", ui.ColorCyan, cmd.UseLine(), ui.ColorReset)
	}
	if cmd.HasAvailableSubCommands() {
		fmt.Fprintf(os.Stderr, "  %s%s%s %s<command>%s %s[flags]%s\n",
			ui.ColorCyan, cmd.CommandPath(), ui.ColorReset,
			ui.ColorYellow, ui.ColorReset,
			ui.ColorDim, ui.ColorReset)
	}

	if cmd.HasAvailableSubCommands() {
		fmt.Fprintf(os.Stderr, "\n%sCommands%s\n", ui.ColorBold+ui.ColorWhite, ui.ColorReset)

		maxLen := 0
		availableCommands := []*cobra.Command{}
		for _, c := range cmd.Commands() {
			if c.IsAvailableCommand() && c.Name() != "help" {
				availableCommands = append(availableCommands, c)
				if len(c.Name()) > maxLen {
					maxLen = len(c.Name())
				}
			}
		}

		for _, c := range availableCommands {
			padding := strings.Repeat(" ", maxLen-len(c.Name())+2)
			fmt.Fprintf(os.Stderr, "  %s%s%s%s%s%s%s\n",
				ui.ColorCyan, c.Name(), ui.ColorReset,
				padding,
				ui.ColorDim, c.Short, ui.ColorReset)
		}
	}

	if cmd.HasAvailableLocalFlags() {
		fmt.Fprintf(os.Stderr, "\n%sFlags%s\n", ui.ColorBold+ui.ColorWhite, ui.ColorReset)
		printFlagsToStderr(cmd.LocalFlags().FlagUsages())
	}

	fmt.Fprintf(os.Stderr, "\n%sUse \"%s%s%s %s--help%s\" for more information.%s\n",
		ui.ColorDim,
		ui.ColorCyan, cmd.CommandPath(), ui.ColorReset+ui.ColorDim,
		ui.ColorGreen, ui.ColorReset+ui.ColorDim,
		ui.ColorReset)

	return nil
}

// printFlags prints flag usages with color formatting to stdout
func printFlags(flagUsages string) {
	printFlagsTo(os.Stdout, flagUsages)
}

// printFlagsToStderr prints flag usages with color formatting to stderr
func printFlagsToStderr(flagUsages string) {
	printFlagsTo(os.Stderr, flagUsages)
}

// printFlagsTo prints flag usages with color formatting to the specified writer
func printFlagsTo(writer *os.File, flagUsages string) {
	lines := strings.Split(flagUsages, "\n")

	// Find maximum flag length for alignment
	maxFlagLen := 0
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " ")
		if strings.HasPrefix(trimmed, "-") {
			parts := strings.SplitN(trimmed, "  ", 2)
			if len(parts) >= 1 {
				flagPart := strings.TrimSpace(parts[0])
				if len(flagPart) > maxFlagLen {
					maxFlagLen = len(flagPart)
				}
			}
		}
	}

	// Set minimum width for alignment
	if maxFlagLen < 28 {
		maxFlagLen = 28
	}

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		trimmed := strings.TrimLeft(line, " ")

		// Check if this is a flag definition line or a continuation
		if strings.HasPrefix(trimmed, "-") {
			parts := strings.SplitN(trimmed, "  ", 2)
			if len(parts) == 2 {
				flagPart := strings.TrimSpace(parts[0])
				descPart := strings.TrimSpace(parts[1])

				padding := strings.Repeat(" ", maxFlagLen-len(flagPart)+2)

				fmt.Fprintf(writer, "  %s%s%s%s%s%s%s\n",
					ui.ColorGreen, flagPart, ui.ColorReset,
					padding,
					ui.ColorDim, descPart, ui.ColorReset)
			} else {
				fmt.Fprintf(writer, "  %s%s%s\n", ui.ColorGreen, trimmed, ui.ColorReset)
			}
		} else {
			// Continuation line (description continues)
			indentSpaces := strings.Repeat(" ", maxFlagLen+4)
			fmt.Fprintf(writer, "%s%s%s%s\n",
				indentSpaces,
				ui.ColorDim, trimmed, ui.ColorReset)
		}
	}
}

// wrapText wraps text at the specified width while preserving paragraphs
func wrapText(text string, width int) string {
	// Split by double newlines to preserve paragraphs
	paragraphs := strings.Split(text, "\n\n")
	var wrappedParagraphs []string

	for _, para := range paragraphs {
		// Split by single newlines to preserve intentional line breaks
		lines := strings.Split(para, "\n")
		var wrappedLines []string

		for _, line := range lines {
			trimmedLine := strings.TrimSpace(line)
			if trimmedLine == "" {
				continue
			}

			// Check if this is a bullet point or list item
			if strings.HasPrefix(trimmedLine, "-") || strings.HasPrefix(trimmedLine, "•") || strings.HasPrefix(trimmedLine, "*") {
				// Don't wrap bullet points with previous content
				wrappedLines = append(wrappedLines, trimmedLine)
				continue
			}

			// Wrap regular lines
			words := strings.Fields(trimmedLine)
			if len(words) == 0 {
				continue
			}

			var currentLine strings.Builder
			for _, word := range words {
				if currentLine.Len() == 0 {
					currentLine.WriteString(word)
				} else if currentLine.Len()+1+len(word) <= width {
					currentLine.WriteString(" ")
					currentLine.WriteString(word)
				} else {
					wrappedLines = append(wrappedLines, currentLine.String())
					currentLine.Reset()
					currentLine.WriteString(word)
				}
			}

			if currentLine.Len() > 0 {
				wrappedLines = append(wrappedLines, currentLine.String())
			}
		}

		if len(wrappedLines) > 0 {
			wrappedParagraphs = append(wrappedParagraphs, strings.Join(wrappedLines, "\n"))
		}
	}

	return strings.Join(wrappedParagraphs, "\n\n")
}
