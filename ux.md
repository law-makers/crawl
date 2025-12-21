# User Experience Improvements

## Overview
This document outlines UX improvements to make the tool more accessible, cross-platform, and user-friendly.

---

## CRITICAL: Cross-Platform Issues

### 1. Hardcoded Chrome Path (BLOCKING)

**Issue**: Chrome path is hardcoded to Linux location, breaking Windows and macOS users.

**Current Code**:
```go
chromedp.ExecPath("/usr/bin/google-chrome-stable")
```

**Impact**: 
- ❌ Windows users: App fails to start
- ❌ macOS users: App fails to start
- ⚠️ Linux users: Works only if Chrome is in exact path

**Solution**: Auto-detect Chrome location across platforms

```go
package browser

import (
    "os"
    "path/filepath"
    "runtime"
)

// FindChrome automatically locates Chrome/Chromium executable
func FindChrome() string {
    // 1. Check environment variable (highest priority)
    if path := os.Getenv("CHROME_PATH"); path != "" {
        if isExecutable(path) {
            return path
        }
    }
    
    // 2. Check standard locations per OS
    var candidates []string
    
    switch runtime.GOOS {
    case "darwin": // macOS
        candidates = []string{
            "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
            "/Applications/Chromium.app/Contents/MacOS/Chromium",
            "/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
            filepath.Join(os.Getenv("HOME"), "Applications/Google Chrome.app/Contents/MacOS/Google Chrome"),
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
        }
        
        // Check Flatpak
        if home := os.Getenv("HOME"); home != "" {
            candidates = append(candidates,
                filepath.Join(home, ".local/share/flatpak/exports/bin/com.google.Chrome"),
            )
        }
    }
    
    // 3. Try each candidate
    for _, path := range candidates {
        if isExecutable(path) {
            return path
        }
    }
    
    // 4. Try PATH
    if path := findInPath(); path != "" {
        return path
    }
    
    // 5. Give up - let chromedp try its default
    return ""
}

func isExecutable(path string) bool {
    info, err := os.Stat(path)
    if err != nil {
        return false
    }
    
    if runtime.GOOS == "windows" {
        return !info.IsDir()
    }
    
    return !info.IsDir() && info.Mode()&0111 != 0
}

func findInPath() string {
    browsers := []string{"google-chrome", "chromium", "chromium-browser", "chrome", "msedge", "brave"}
    
    for _, name := range browsers {
        if path, err := exec.LookPath(name); err == nil {
            return path
        }
    }
    
    return ""
}
```

**Usage**:
```go
chromePath := browser.FindChrome()
if chromePath != "" {
    allocOpts = append(allocOpts, chromedp.ExecPath(chromePath))
} else {
    log.Warn().Msg("Chrome not found, using system default")
}
```

**Estimated Effort**: 1 day

---

### 2. Path Handling for Windows

**Issue**: Unix-style paths hardcoded throughout codebase.

**Examples**:
```go
FallbackDir = ".crawl/sessions"  // Should use filepath.Join
dir := filepath.Join(home, FallbackDir)  // ✅ Already correct!
```

**Action**: Audit all path operations and ensure `filepath.Join` is used.

**Estimated Effort**: 2 hours

---

### 3. Windows-Specific Keyring Issues

**Issue**: Keyring library may not work on Windows without additional setup.

**Current Behavior**:
```go
err := keyring.Set(KeyringService, session.Name, string(data))
```

**Windows Requirements**: 
- Uses Windows Credential Manager
- May require admin permissions
- Different behavior in dev containers

**Solution**: Better error messages and fallback instructions

```go
func SaveSession(session *SessionData) error {
    err := keyring.Set(KeyringService, session.Name, string(data))
    if err != nil {
        if runtime.GOOS == "windows" {
            return fmt.Errorf("failed to save to Windows Credential Manager: %w\n"+
                "Try running as administrator or use file-based storage:\n"+
                "  set CRAWL_STORAGE=file\n"+
                "  crawl login ...", err)
        }
        return err
    }
    return nil
}
```

**Estimated Effort**: 1 hour

---

## HIGH Priority: Installation & Setup UX

### 4. One-Line Installation Script

**Issue**: No easy installation method documented.

**Current**: Users must manually build from source.

**Proposed**:

**Linux/macOS**:
```bash
curl -fsSL https://raw.githubusercontent.com/law-makers/crawl/main/install.sh | sh
```

**Windows (PowerShell)**:
```powershell
iwr -useb https://raw.githubusercontent.com/law-makers/crawl/main/install.ps1 | iex
```

**Homebrew (macOS/Linux)**:
```bash
brew tap law-makers/crawl
brew install crawl
```

**Scoop (Windows)**:
```powershell
scoop bucket add crawl https://github.com/law-makers/scoop-crawl
scoop install crawl
```

**Go Install**:
```bash
go install github.com/law-makers/crawl/cmd/crawl@latest
```

**Docker**:
```bash
docker run --rm -v $(pwd):/data lawmakers/crawl get https://example.com
```

**Estimated Effort**: 3-4 days (including packaging setup)

---

### 5. First-Run Experience

**Issue**: No welcome message or setup wizard for first-time users.

**Proposed**:

```bash
$ crawl

👋 Welcome to Crawl!

It looks like this is your first time running Crawl.
Let me help you get started...

✓ Chrome detected at /usr/bin/google-chrome-stable
✓ Session storage: Using file-based storage (~/.crawl/sessions)
✓ Cache directory created: ~/.crawl/cache

Quick Start:
  1. Scrape a webpage:     crawl get https://example.com
  2. Extract specific data: crawl get URL --selector=".price"
  3. Download media:       crawl media URL
  4. Get help:             crawl --help

Documentation: https://github.com/law-makers/crawl/wiki
```

**Implementation**:
```go
func checkFirstRun() bool {
    configDir := filepath.Join(os.UserHomeDir(), ".crawl")
    firstRunMarker := filepath.Join(configDir, ".initialized")
    
    if _, err := os.Stat(firstRunMarker); os.IsNotExist(err) {
        showWelcome()
        os.MkdirAll(configDir, 0755)
        os.WriteFile(firstRunMarker, []byte(time.Now().String()), 0644)
        return true
    }
    return false
}
```

**Estimated Effort**: 1 day

---

### 6. Configuration File Support

**Issue**: All configuration via CLI flags. No config file support.

**Proposed**: `~/.crawl/config.yaml`

```yaml
# ~/.crawl/config.yaml
defaults:
  mode: auto
  timeout: 30s
  concurrency: 5
  user_agent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"
  
# Chrome path override
chrome_path: /Applications/Google Chrome.app/Contents/MacOS/Google Chrome

# Output preferences
output:
  default_format: json
  pretty_print: true
  
# Rate limiting
rate_limits:
  default: 5/s
  example.com: 2/s
  api.site.com: 10/s
  
# Proxy settings
proxy:
  enabled: false
  url: http://localhost:8080
  
# Cache settings
cache:
  enabled: true
  max_size_mb: 100
  ttl: 5m
  
# Session storage
sessions:
  storage: keyring  # or 'file'
  warn_expiry_days: 7

# Profiles for different sites
profiles:
  ecommerce:
    mode: static
    selector: ".product"
    fields:
      title: "h1.product-title"
      price: ".price"
      image: "img.product-image@src"
      
  news:
    mode: spa
    wait: 2s
    selector: "article"
```

**Usage**:
```bash
# Use profile
crawl get https://shop.com --profile=ecommerce

# Override config
crawl get URL --timeout=60s  # Overrides config.yaml

# Use different config file
crawl get URL --config=/path/to/config.yaml
```

**Estimated Effort**: 2 days

---

## MEDIUM Priority: CLI UX Improvements

### 7. Interactive Mode (REPL)

**Issue**: Have to run full command each time. No interactive exploration.

**Proposed**:
```bash
$ crawl interactive
Crawl Interactive Shell (type 'help' for commands)

> fetch https://example.com
✓ Fetched in 1.2s (200 OK)

> select h1
Example Domain

> select p
This domain is for use in illustrative examples...

> extract .price --as json
{"price": "$29.99"}

> screenshot page.png
✓ Screenshot saved to page.png

> save data.json
✓ Saved to data.json

> exit
```

**Implementation**: Use `github.com/chzyer/readline` or `github.com/c-bata/go-prompt`

**Estimated Effort**: 2-3 days

---

### 8. Better Progress Indicators

**Issue**: Limited progress feedback for long operations.

**Current**: Simple progress bar for downloads only.

**Proposed**:

```bash
$ crawl get https://example.com --verbose

⣾ Starting Chrome browser...
✓ Chrome started (1.2s)

⣾ Navigating to https://example.com...
✓ Page loaded (0.8s)

⣾ Waiting for content...
✓ Content ready (0.5s)

⣾ Extracting data...
✓ Found 25 items (0.2s)

✓ Done! Total time: 2.7s
```

**Multi-step operations**:
```bash
$ crawl sitemap https://example.com

Found 125 URLs in sitemap

Scraping [████████████████████░░░░░░] 82/125 (66%)
  ✓ Completed: 75
  ⣾ In progress: 7
  ✗ Failed: 3
  ⏳ Pending: 40
  
ETA: 2m 15s | Rate: 2.5 pages/s
```

**Implementation**: Use `github.com/schollz/progressbar/v3` (already used) + `github.com/briandowns/spinner`

**Estimated Effort**: 1 day

---

### 9. Command Aliases & Shortcuts

**Issue**: Long command names. Frequent operations are verbose.

**Proposed**:
```bash
# Aliases
crawl g URL              # alias for 'get'
crawl m URL              # alias for 'media'
crawl s list             # alias for 'sessions'

# Common operations
crawl URL                # Smart detection (get vs media)
crawl @profile URL       # Use saved profile
crawl :session URL       # Use saved session

# Chaining
crawl get URL | jq .title
crawl get URL -o - | grep "price"  # stdout output
```

**Configuration**:
```yaml
# In config.yaml
aliases:
  g: get
  m: media
  s: sessions
  
shortcuts:
  amazon: "get --profile=ecommerce --session=amazon"
  news: "get --profile=news --mode=spa"
```

**Estimated Effort**: 1 day

---

### 10. Better Error Messages

**Issue**: Generic error messages don't help users fix issues.

**Current**:
```
Error: failed to fetch URL: context deadline exceeded
```

**Proposed**:
```
✗ Failed to scrape https://example.com

  Error: Request timeout after 30 seconds
  
  This usually means:
    • The server is slow or unresponsive
    • The page requires JavaScript (try --mode=spa)
    • Network connectivity issues
    
  Try:
    • Increase timeout: --timeout=60s
    • Use SPA mode: --mode=spa
    • Check your connection: curl -I https://example.com
    
  Need help? https://github.com/law-makers/crawl/issues
```

**For Chrome errors**:
```
✗ Chrome failed to start

  Error: Chrome executable not found
  
  Install Chrome:
    • macOS:   brew install --cask google-chrome
    • Linux:   sudo apt install google-chrome-stable
    • Windows: Download from https://chrome.google.com
    
  Or set custom path:
    export CHROME_PATH=/path/to/chrome
```

**Implementation**:
```go
type UserFriendlyError struct {
    Title       string
    Error       error
    Explanation []string
    Suggestions []string
    HelpURL     string
}

func (e *UserFriendlyError) Print() {
    fmt.Printf("\n✗ %s\n\n", e.Title)
    fmt.Printf("  Error: %s\n\n", e.Error)
    
    if len(e.Explanation) > 0 {
        fmt.Println("  This usually means:")
        for _, exp := range e.Explanation {
            fmt.Printf("    • %s\n", exp)
        }
        fmt.Println()
    }
    
    if len(e.Suggestions) > 0 {
        fmt.Println("  Try:")
        for _, sug := range e.Suggestions {
            fmt.Printf("    • %s\n", sug)
        }
        fmt.Println()
    }
    
    if e.HelpURL != "" {
        fmt.Printf("  Need help? %s\n\n", e.HelpURL)
    }
}
```

**Estimated Effort**: 2-3 days

---

### 11. Dry Run Mode

**Issue**: No way to preview what will be scraped without actually scraping.

**Proposed**:
```bash
$ crawl get URL --dry-run

Dry Run (no actual scraping)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

URL:       https://example.com
Mode:      auto (will detect: likely static)
Selector:  body
Timeout:   30s
Cache:     enabled
Session:   none

Would execute:
  1. HTTP GET https://example.com
  2. Parse HTML with goquery
  3. Extract content matching 'body'
  4. Return results

No data will be fetched.
```

**Estimated Effort**: 1 day

---

## LOW Priority: Polish & QoL

### 12. Shell Completion

**Issue**: No shell autocomplete support.

**Implementation**: Use cobra's built-in completion

```bash
# Generate completion
crawl completion bash > /etc/bash_completion.d/crawl
crawl completion zsh > ~/.zsh/completions/_crawl
crawl completion fish > ~/.config/fish/completions/crawl.fish

# Auto-install
crawl completion install
```

**Estimated Effort**: 4 hours (cobra handles most of it)

---

### 13. Update Checker

**Issue**: Users don't know when new versions are available.

**Proposed**:
```bash
$ crawl get URL

ℹ️ Update available: v0.2.0 → v0.3.0
   Run 'crawl update' to upgrade

[... normal output ...]
```

**Implementation**:
```go
func checkForUpdates() {
    // Check GitHub releases API
    // Cache result for 24 hours
    // Display non-intrusively
}
```

**Estimated Effort**: 1 day

---

### 14. Colorized Output Options

**Issue**: Output color scheme not customizable.

**Proposed**:
```bash
# Disable colors (for CI/scripts)
crawl get URL --no-color

# Different themes
crawl get URL --theme=dark
crawl get URL --theme=light
crawl get URL --theme=minimal

# Respect NO_COLOR env
export NO_COLOR=1
crawl get URL  # Automatically no colors
```

**Estimated Effort**: 1 day

---

### 15. Telemetry & Analytics (Opt-in)

**Issue**: No visibility into usage patterns for improving tool.

**Proposed**: Opt-in anonymous usage statistics

```bash
# On first run, ask
Would you like to help improve Crawl by sending anonymous usage statistics?
This includes: command usage, performance metrics, error rates
Does NOT include: URLs, scraped data, or personal information

[Y/n]
```

**What to track**:
- Command usage (which commands are popular)
- Error rates (which features have most issues)
- Performance metrics (average scrape time)
- Platform distribution (OS, Go version)
- Feature adoption (which flags are used)

**Implementation**: 
- Use PostHog or Segment
- Clear opt-in/opt-out
- Disable with env var: `CRAWL_TELEMETRY=0`

**Estimated Effort**: 2 days

---

### 16. Verbose Output Levels

**Issue**: Only two modes: quiet or verbose. Need granularity.

**Proposed**:
```bash
crawl get URL                # Normal (info only)
crawl get URL -v             # Verbose (info + debug)
crawl get URL -vv            # Very verbose (trace)
crawl get URL -q             # Quiet (errors only)
crawl get URL --log-level=trace  # Explicit level
```

**Implementation**:
```go
func initLogging() {
    var level zerolog.Level
    
    switch {
    case quiet:
        level = zerolog.ErrorLevel
    case verboseCount >= 2:
        level = zerolog.TraceLevel
    case verbose:
        level = zerolog.DebugLevel
    default:
        level = zerolog.InfoLevel
    }
    
    zerolog.SetGlobalLevel(level)
}
```

**Estimated Effort**: 2 hours

---

### 17. ASCII Art Banner (Optional)

**Issue**: CLI is bland. No visual identity.

**Proposed**:
```bash
$ crawl --version

   ▄████▄   ██▀███   ▄▄▄       █     █░ ██▓    
  ▒██▀ ▀█  ▓██ ▒ ██▒▒████▄    ▓█░ █ ░█░▓██▒    
  ▒▓█    ▄ ▓██ ░▄█ ▒▒██  ▀█▄  ▒█░ █ ░█ ▒██░    
  ▒▓▓▄ ▄██▒▒██▀▀█▄  ░██▄▄▄▄██ ░█░ █ ░█ ▒██░    
  ▒ ▓███▀ ░░██▓ ▒██▒ ▓█   ▓██▒░░██▒██▓ ░██████▒
  ░ ░▒ ▒  ░░ ▒▓ ░▒▓░ ▒▒   ▓▒█░░ ▓░▒ ▒  ░ ▒░▓  ░
    ░  ▒     ░▒ ░ ▒░  ▒   ▒▒ ░  ▒ ░ ░  ░ ░ ▒  ░
  ░          ░░   ░   ░   ▒     ░   ░    ░ ░   
  ░ ░         ░           ░  ░    ░        ░  ░

  Fast cross-platform web scraper
  Version 0.1.0 | https://github.com/law-makers/crawl
```

**Optional**: Can be disabled with `--no-banner`

**Estimated Effort**: 1 hour

---

## Testing & Validation

### 18. Platform-Specific Test Matrix

**Add GitHub Actions matrix**:

```yaml
# .github/workflows/test.yml
strategy:
  matrix:
    os: [ubuntu-latest, windows-latest, macos-latest]
    go: ['1.23', '1.24', '1.25']
    
steps:
  - name: Test Chrome Detection
    run: go test ./internal/browser -v
    
  - name: Test CLI
    run: |
      ./bin/crawl get https://example.com
      ./bin/crawl media https://example.com --type=image
      ./bin/crawl --version
```

**Estimated Effort**: 1 day

---

### 19. Integration Tests for All Platforms

**Create test suite**:

```bash
tests/
  platform/
    test_chrome_detection.sh
    test_paths.sh
    test_keyring.sh
  cli/
    test_basic_commands.sh
    test_output_formats.sh
  e2e/
    test_full_workflow.sh
```

**Estimated Effort**: 2-3 days

---

## Documentation Improvements

### 20. Comprehensive Examples

**Create examples directory**:

```
examples/
  basic/
    simple_scrape.sh
    with_selector.sh
    multiple_pages.sh
  advanced/
    authenticated_scraping.sh
    media_download.sh
    batch_processing.sh
  integrations/
    github_actions.yml
    docker_compose.yml
    kubernetes.yaml
```

**Estimated Effort**: 2 days

---

### 21. Video Tutorials

**Create screencast tutorials**:
1. Getting Started (5 min)
2. Authentication & Sessions (5 min)
3. Batch Scraping (10 min)
4. Advanced Selectors (10 min)
5. Troubleshooting (5 min)

**Estimated Effort**: 1 week

---

## Summary by Priority

### Critical (Blocks adoption)
1. ✅ Cross-platform Chrome detection
2. ✅ Windows path handling
3. ✅ Keyring fallback for all platforms

### High Priority (Major UX issues)
1. Easy installation (Homebrew, Scoop, etc.)
2. First-run experience
3. Configuration file support
4. Better error messages

### Medium Priority (Nice to have)
1. Interactive mode (REPL)
2. Progress indicators
3. Command aliases
4. Dry run mode
5. Shell completion

### Low Priority (Polish)
1. Update checker
2. Color themes
3. Telemetry (opt-in)
4. ASCII banner
5. Video tutorials

---

## Implementation Roadmap

**Phase 1: Cross-Platform Support (Critical)**
- Week 1: Chrome auto-detection
- Week 2: Windows testing & fixes
- Week 3: macOS testing & fixes
- Week 4: Linux variant testing

**Phase 2: Easy Onboarding**
- Week 5-6: Installation scripts & packages
- Week 7: First-run experience
- Week 8: Configuration file support

**Phase 3: UX Polish**
- Week 9-10: Better errors & progress
- Week 11: Interactive mode
- Week 12: Documentation & examples

**Total: 3 months for complete UX overhaul**

---

## Success Metrics

Track these metrics before/after improvements:

1. **Installation success rate**: % of users who successfully install
2. **First-run success rate**: % who complete first command
3. **Error resolution time**: Time to resolve common errors
4. **Platform distribution**: Windows/macOS/Linux adoption
5. **Feature adoption**: Which features are actually used
6. **Support requests**: Reduction in common issues

**Target Goals**:
- 95% installation success rate
- <5 minute time-to-first-scrape
- 50% reduction in support requests
- Equal platform distribution (33/33/33)
