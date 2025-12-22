package ui

// ANSI color and style constants for CLI output
const (
	ColorReset = "\033[0m"
	ColorBold  = "\033[1m"
	ColorDim   = "\033[2m"

	ColorCyan   = "\033[36m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorWhite  = "\033[97m"
	ColorRed    = "\033[31m"
)

// Convenience helper to build styled strings. Keep minimal so tests can use constants directly.
func Bold(s string) string {
	return ColorBold + s + ColorReset
}

func Success(s string) string {
	return ColorGreen + s + ColorReset
}

func Info(s string) string {
	return ColorDim + ColorYellow + s + ColorReset
}

func Error(s string) string {
	return ColorRed + s + ColorReset
}
