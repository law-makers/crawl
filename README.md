<div align="center">

# 🕷️ Crawl

**A fast, cross-platform CLI for intelligent web scraping**

[![Go Version](https://img.shields.io/badge/Go-1.25.4+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![GitHub Release](https://img.shields.io/github/v/release/law-makers/crawl?style=flat)](https://github.com/law-makers/crawl/releases)
[![Build Status](https://img.shields.io/github/actions/workflow/status/law-makers/crawl/test.yml?branch=main)](https://github.com/law-makers/crawl/actions)

[Features](#-features) •
[Installation](#-installation) •
[Quick Start](#-quick-start) •
[Usage](#-usage) •
[Documentation](#-documentation)

</div>

---

## 📖 Overview

**Crawl** is a unified data extraction tool designed to scrape static sites, SPAs (Single Page Applications), authenticated sessions, and rich media from social platforms. It intelligently switches between fast HTTP requests and headless browser automation depending on the content type, making it the perfect tool for data extraction, monitoring, testing, and archiving.

### Why Crawl?

- **🚀 Intelligent Mode Detection**: Automatically detects whether to use fast static scraping or headless browser for SPAs
- **🔐 Authentication Support**: Login interactively and reuse sessions across scrapes
- **📥 Media Downloader**: Extract and download images, videos, and audio with concurrent workers
- **⚡ High Performance**: Built-in rate limiting, caching, and connection pooling
- **🎯 Flexible Extraction**: CSS selectors, custom fields, multiple output formats (JSON, CSV, Markdown, HTML)
- **🌐 Cross-Platform**: Works seamlessly on Linux, macOS, and Windows

---

## ✨ Features

### Core Capabilities

- **Dual-Engine Architecture**
  - **Fast Engine**: Lightning-fast HTTP/HTML parsing for static content
  - **Deep Engine**: Headless Chrome for JavaScript-heavy SPAs
  - **Auto-detection**: Intelligently switches between engines

- **Authentication & Sessions**
  - Interactive browser-based login
  - Secure session storage (OS keyring)
  - Reusable sessions across commands
  - Cookie and localStorage support

- **Media Extraction**
  - Download images, videos, and audio files
  - Concurrent download workers (1-50)
  - Progress tracking with visual indicators
  - Smart file naming and organization

- **Data Extraction**
  - CSS selector-based scraping
  - Custom field mapping
  - Multiple output formats (JSON, CSV, Markdown, HTML)
  - Structured data extraction

- **Performance Features**
  - Built-in caching with TTL
  - Rate limiting per domain
  - Connection pooling
  - Proxy support
  - Retry logic with exponential backoff

---

## 🚀 Installation

### Using Go Install (Recommended)

```bash
go install github.com/law-makers/crawl/cmd/crawl@latest
```

### From Source

```bash
git clone https://github.com/law-makers/crawl.git
cd crawl
go build -o crawl ./cmd/crawl
sudo mv crawl /usr/local/bin/
```

### Prerequisites

- **Go 1.25.4+** (for building from source)
- **Google Chrome or Chromium** (for SPA scraping)
  - Linux: `sudo apt install google-chrome-stable`
  - macOS: `brew install --cask google-chrome`
  - Windows: Download from [chrome.google.com](https://www.google.com/chrome/)

---

## 🎯 Quick Start

### Basic Scraping

```bash
# Scrape a static website (auto-detects mode)
crawl get https://example.com

# Scrape with CSS selector
crawl get https://example.com --selector=".article"

# Force SPA mode for JavaScript-heavy sites
crawl get https://spa-site.com --mode=spa

# Save output to JSON file
crawl get https://api.example.com --output=data.json
```

### Media Downloads

```bash
# Download all images from a page
crawl media https://gallery.example.com --type=image

# Download videos with 10 concurrent workers
crawl media https://videos.example.com --type=video --concurrency=10

# Download all media types to custom directory
crawl media https://example.com --type=all --output=./my-downloads
```

### Authentication

```bash
# Login and save session
crawl login https://github.com/login --session=github --wait="#dashboard"

# Use saved session for authenticated requests
crawl get https://github.com/settings/profile --session=github

# List all saved sessions
crawl sessions list

# View session details
crawl sessions view github

# Delete a session
crawl sessions delete github
```

---

## 📚 Usage

### `crawl get` - Retrieve Web Content

Extract text and structured data from web pages.

```bash
crawl get <url> [flags]
```

**Flags:**
- `--mode, -m`: Scraper mode - `auto`, `static`, or `spa` (default: `auto`)
- `--selector, -s`: CSS selector for content extraction
- `--output, -o`: Output file path (stdout if not specified)
- `--fields`: Custom field mapping in JSON format
- `--session`: Session name for authenticated requests
- `--headers`: Custom HTTP headers (can be used multiple times)
- `--timeout`: Request timeout (default: `30s`)
- `--user-agent`: Custom user agent string

**Examples:**

```bash
# Extract product information
crawl get https://shop.example.com/product \
  --selector=".product" \
  --fields='{"title": "h1", "price": ".price", "image": "img@src"}'

# Scrape with custom headers
crawl get https://api.example.com \
  --headers="Authorization: Bearer token123" \
  --headers="Accept: application/json"

# SPA with longer timeout
crawl get https://react-app.com --mode=spa --timeout=60s
```

### `crawl media` - Download Media Files

Extract and download images, videos, and audio files.

```bash
crawl media <url> [flags]
```

**Flags:**
- `--type, -t`: Media type - `image`, `video`, `audio`, or `all` (default: `all`)
- `--concurrency, -c`: Number of concurrent workers (1-50, default: `5`)
- `--output, -o`: Output directory (default: `./downloads`)
- `--mode, -m`: Scraper mode - `auto`, `static`, or `spa`
- `--session`: Session name for authenticated requests

**Examples:**

```bash
# Download profile pictures
crawl media https://example.com/gallery --type=image --output=./photos

# Download videos from authenticated page
crawl media https://site.com/private/videos \
  --type=video \
  --session=mylogin \
  --concurrency=10

# Download all media from SPA
crawl media https://spa-gallery.com --mode=spa --type=all
```

### `crawl login` - Interactive Authentication

Login to websites and save sessions for later use.

```bash
crawl login <url> [flags]
```

**Flags:**
- `--session, -s`: Session name to save (required)
- `--wait, -w`: CSS selector to wait for after login (e.g., `#dashboard`)
- `--login-timeout`: Timeout for login process (default: `5m`)
- `--remote-debug`: Enable Chrome remote debugging on specified port

**Examples:**

```bash
# Basic login
crawl login https://github.com/login --session=github

# Wait for specific element after login
crawl login https://app.example.com/login \
  --session=myapp \
  --wait="#user-dashboard"

# Login in dev container with remote debugging
crawl login https://example.com/login \
  --session=example \
  --remote-debug=9222
```

### `crawl sessions` - Manage Sessions

View and manage saved authentication sessions.

```bash
# List all sessions
crawl sessions list

# View session details
crawl sessions view <session-name>

# Delete a session
crawl sessions delete <session-name>
```

### `crawl import-cookies` - Import Browser Cookies

Import cookies from Chrome, Firefox, or JSON file.

```bash
crawl import-cookies --session=<name> --browser=<chrome|firefox> [--profile=<path>]
```

---

## ⚙️ Configuration

### Global Flags

These flags work with all commands:

- `--proxy`: HTTP/HTTPS proxy URL
- `--timeout`: Request timeout duration
- `--user-agent`: Custom user agent string
- `--verbose, -v`: Enable verbose logging
- `--quiet, -q`: Suppress all output except errors
- `--json`: Output in JSON format

### Environment Variables

```bash
# Set Chrome executable path
export CHROME_PATH=/path/to/chrome

# Configure cache directory
export CRAWL_CACHE_DIR=~/.cache/crawl

# Session storage directory
export CRAWL_SESSION_DIR=~/.crawl/sessions

# Disable cache
export CRAWL_NO_CACHE=1
```

### Output Formats

Crawl supports multiple output formats:

- **JSON** (default): Structured data, easy to parse
- **CSV**: Tabular data, spreadsheet-compatible
- **Markdown**: Human-readable formatted text
- **HTML**: Formatted HTML output

Specify format using the `--output` flag extension or `--format` flag:

```bash
crawl get URL --output=data.json    # JSON
crawl get URL --output=data.csv     # CSV
crawl get URL --output=data.md      # Markdown
crawl get URL --output=data.html    # HTML
```

---

## 🔧 Advanced Features

### Rate Limiting

Crawl includes built-in rate limiting to be respectful of target servers:

```bash
# Default rate limit: 5 requests/second per domain
crawl get URL

# Customize via code or configuration
```

### Caching

Intelligent caching reduces redundant requests:

- **TTL-based**: Cached responses expire after 5 minutes (configurable)
- **Memory + Disk**: In-memory cache with disk persistence
- **Cache Control**: Respects HTTP cache headers

```bash
# Cache is enabled by default
crawl get URL

# Disable cache for fresh data
CRAWL_NO_CACHE=1 crawl get URL
```

### Proxy Support

Route requests through HTTP/HTTPS proxies:

```bash
crawl get URL --proxy=http://proxy.example.com:8080

# With authentication
crawl get URL --proxy=http://user:pass@proxy.example.com:8080
```

### Custom Headers

Add custom HTTP headers for API authentication or special requirements:

```bash
crawl get URL \
  --headers="Authorization: Bearer token" \
  --headers="X-Custom-Header: value"
```

### Field Extraction

Extract structured data with custom field mapping:

```bash
crawl get https://shop.example.com/product \
  --fields='{
    "title": "h1.product-name",
    "price": ".price-value",
    "description": ".product-description",
    "image": "img.main-image@src",
    "availability": ".stock-status@data-available"
  }'
```

**Selector Syntax:**
- `selector`: Extract text content
- `selector@attr`: Extract attribute value
- `selector:html`: Extract inner HTML

---

## 🐛 Troubleshooting

### Chrome Not Found

**Error:** `Chrome executable not found`

**Solution:**
```bash
# Set custom Chrome path
export CHROME_PATH=/path/to/chrome

# Or install Chrome
# Linux
sudo apt install google-chrome-stable

# macOS
brew install --cask google-chrome
```

### Session Storage Issues

**Error:** `Failed to save session`

**Solutions:**
1. Check keyring access (requires OS keyring support)
2. Use file-based storage as fallback
3. Ensure proper permissions on `~/.crawl` directory

### Timeout Errors

**Error:** `Context deadline exceeded`

**Solutions:**
```bash
# Increase timeout
crawl get URL --timeout=60s

# Try SPA mode if page requires JavaScript
crawl get URL --mode=spa
```

### Connection Issues

**Error:** `Connection refused` or `Network error`

**Solutions:**
1. Check your internet connection
2. Verify the URL is accessible
3. Try using a proxy
4. Check if site blocks automated requests

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────┐
│                   CLI Interface                      │
│  (cobra-based commands: get, media, login, sessions) │
└────────────────┬────────────────────────────────────┘
                 │
    ┌────────────┴────────────┐
    │                         │
┌───▼────────┐        ┌───────▼──────┐
│ Fast Engine│        │ Deep Engine   │
│ (HTTP)     │        │ (Chrome)      │
│ - goquery  │        │ - chromedp    │
│ - net/http │        │ - JavaScript  │
└────┬───────┘        └───────┬───────┘
     │                        │
     └────────┬───────────────┘
              │
    ┌─────────▼──────────┐
    │   Core Services     │
    ├────────────────────┤
    │ • Rate Limiter     │
    │ • Cache Manager    │
    │ • Session Store    │
    │ • Retry Logic      │
    │ • Proxy Pool       │
    └────────────────────┘
```

### Project Structure

```
crawl/
├── cmd/crawl/           # CLI entry point
├── internal/
│   ├── app/            # Application context
│   ├── auth/           # Authentication & sessions
│   ├── cache/          # Response caching
│   ├── cli/            # CLI commands
│   ├── config/         # Configuration management
│   ├── downloader/     # Media download engine
│   ├── engine/         # Scraping engines (static/dynamic)
│   ├── proxy/          # Proxy pool management
│   ├── ratelimit/      # Rate limiting
│   ├── retry/          # Retry logic
│   ├── ui/             # UI components (progress bars)
│   └── utils/          # Utilities (headers, output, URL)
├── pkg/
│   └── models/         # Shared data models
└── go.mod
```

---

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

### Development Setup

```bash
# Clone repository
git clone https://github.com/law-makers/crawl.git
cd crawl

# Install dependencies
go mod download

# Run tests
go test ./...

# Build
go build -o crawl ./cmd/crawl

# Run
./crawl --help
```

### Code Standards

- Follow [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Write tests for new features
- Update documentation
- Use `gofmt` for formatting
- Run `go vet` and fix issues

### Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/engine/...

# Run with verbose output
go test -v ./...
```

---

## 📊 Performance & Limits

### Benchmarks

- **Static scraping**: ~100-500 requests/second
- **SPA scraping**: ~5-20 pages/second (Chrome overhead)
- **Media downloads**: Limited by network and concurrency settings

### Recommended Limits

- **Concurrency**: 5-10 workers for most use cases
- **Rate limiting**: 2-5 requests/second per domain
- **Timeout**: 30s for static, 60s+ for SPAs
- **Cache TTL**: 5 minutes (adjust based on content freshness needs)

---

## 🔒 Security & Privacy

### Session Storage

- Sessions stored securely in OS keyring (macOS Keychain, Windows Credential Manager, Linux Secret Service)
- Fallback to encrypted file storage if keyring unavailable
- Cookies never logged or transmitted

### Best Practices

- **Respect robots.txt**: Always check site's robots.txt before scraping
- **Rate limiting**: Use appropriate rate limits to avoid overloading servers
- **User agent**: Use descriptive user agent identifying your tool
- **Terms of Service**: Review and comply with website ToS
- **Personal data**: Handle scraped data responsibly and legally

---

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## 🙏 Acknowledgments

Built with amazing open-source libraries:

- [chromedp](https://github.com/chromedp/chromedp) - Chrome DevTools Protocol in Go
- [goquery](https://github.com/PuerkitoBio/goquery) - jQuery-like HTML parsing
- [cobra](https://github.com/spf13/cobra) - CLI framework
- [zerolog](https://github.com/rs/zerolog) - Fast structured logging
- [html-to-markdown](https://github.com/JohannesKaufmann/html-to-markdown) - HTML to Markdown conversion
- [go-keyring](https://github.com/zalando/go-keyring) - Secure credential storage

---

## 📞 Support

- **Issues**: [GitHub Issues](https://github.com/law-makers/crawl/issues)
- **Discussions**: [GitHub Discussions](https://github.com/law-makers/crawl/discussions)
- **Documentation**: [Wiki](https://github.com/law-makers/crawl/wiki)

---

<div align="center">

**[⬆ Back to Top](#-crawl)**

Made with ❤️ by the Law Makers team

</div>