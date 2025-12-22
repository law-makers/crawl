// internal/engine/dynamic/chrome.go
package dynamic

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/rs/zerolog/log"
)

// FindChrome automatically locates Chrome/Chromium executable across platforms
func FindChrome() string {
	// 1. Check environment variable (highest priority)
	if path := os.Getenv("CHROME_PATH"); path != "" {
		if isExecutable(path) {
			log.Debug().Str("path", path).Msg("Chrome found via CHROME_PATH environment variable")
			return path
		}
		log.Warn().Str("path", path).Msg("CHROME_PATH set but not executable")
	}

	// 2. Check standard locations per OS
	var candidates []string

	switch runtime.GOOS {
	case "darwin": // macOS
		candidates = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		}

		// Check user Applications folder
		if home := os.Getenv("HOME"); home != "" {
			candidates = append(candidates,
				filepath.Join(home, "Applications/Google Chrome.app/Contents/MacOS/Google Chrome"),
				filepath.Join(home, "Applications/Chromium.app/Contents/MacOS/Chromium"),
			)
		}

	case "windows":
		programFiles := []string{
			os.Getenv("ProgramFiles"),
			os.Getenv("ProgramFiles(x86)"),
			os.Getenv("LocalAppData"),
		}

		for _, base := range programFiles {
			if base != "" {
				candidates = append(candidates,
					filepath.Join(base, "Google\\Chrome\\Application\\chrome.exe"),
					filepath.Join(base, "Chromium\\Application\\chrome.exe"),
					filepath.Join(base, "Microsoft\\Edge\\Application\\msedge.exe"),
					filepath.Join(base, "BraveSoftware\\Brave-Browser\\Application\\brave.exe"),
				)
			}
		}

	case "linux":
		candidates = []string{
			"/usr/bin/google-chrome-stable",
			"/usr/bin/google-chrome",
			"/usr/bin/chromium-browser",
			"/usr/bin/chromium",
			"/snap/bin/chromium",
			"/usr/bin/microsoft-edge",
			"/usr/bin/brave-browser",
			"/usr/bin/brave",
		}

		// Check Flatpak
		if home := os.Getenv("HOME"); home != "" {
			candidates = append(candidates,
				filepath.Join(home, ".local/share/flatpak/exports/bin/com.google.Chrome"),
				filepath.Join(home, ".local/share/flatpak/exports/bin/org.chromium.Chromium"),
			)
		}

		// Check snap
		if _, err := os.Stat("/snap/bin/chromium"); err == nil {
			candidates = append(candidates, "/snap/bin/chromium")
		}
	}

	// 3. Try each candidate
	for _, path := range candidates {
		if isExecutable(path) {
			log.Debug().Str("path", path).Str("os", runtime.GOOS).Msg("Chrome found at standard location")
			return path
		}
	}

	// 4. Try to find in PATH
	if path := findInPath(); path != "" {
		log.Debug().Str("path", path).Msg("Chrome found in PATH")
		return path
	}

	// 5. Give up - let chromedp try its default
	log.Warn().
		Str("os", runtime.GOOS).
		Msg("Chrome not found, will use chromedp default (may fail)")
	return ""
}

// isExecutable checks if a file exists and is executable
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	if runtime.GOOS == "windows" {
		// On Windows, just check if it's a file
		return !info.IsDir()
	}

	// On Unix-like systems, check execute permission
	return !info.IsDir() && info.Mode()&0111 != 0
}

// findInPath searches for Chrome-like browsers in PATH
func findInPath() string {
	browsers := []string{
		"google-chrome-stable",
		"google-chrome",
		"chromium",
		"chromium-browser",
		"chrome",
		"msedge",
		"brave",
		"brave-browser",
	}

	for _, name := range browsers {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}

	return ""
}

// GetChromeVersion returns the version of Chrome if detectable
func GetChromeVersion(chromePath string) string {
	if chromePath == "" {
		return "unknown"
	}

	var args []string
	switch runtime.GOOS {
	case "windows":
		// Windows Chrome doesn't support --version directly
		return "detected"
	case "darwin":
		args = []string{"--version"}
	default:
		args = []string{"--version"}
	}

	cmd := exec.Command(chromePath, args...)
	output, err := cmd.Output()
	if err != nil {
		return "detected"
	}

	return string(output)
}
