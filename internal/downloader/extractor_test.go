package downloader

import (
	"strings"
	"testing"
)

func TestExtractMedia_Images(t *testing.T) {
	html := `
	<html>
		<body>
			<img src="/images/photo1.jpg">
			<img src="https://cdn.example.com/photo2.png">
		</body>
	</html>
	`

	urls, err := ExtractMedia(html, "https://example.com", MediaTypeImage)
	if err != nil {
		t.Fatalf("ExtractMedia failed: %v", err)
	}

	if len(urls) == 0 {
		t.Fatal("No images extracted")
	}

	found := false
	for _, url := range urls {
		if strings.Contains(url, "photo1.jpg") || strings.Contains(url, "photo2.png") {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected images not found in extracted URLs")
	}
}

func TestExtractMedia_Videos(t *testing.T) {
	html := `
	<html>
		<body>
			<video src="/videos/clip.mp4"></video>
		</body>
	</html>
	`

	urls, err := ExtractMedia(html, "https://example.com", MediaTypeVideo)
	if err != nil {
		t.Fatalf("ExtractMedia failed: %v", err)
	}

	if len(urls) == 0 {
		t.Fatal("No videos extracted")
	}

	found := false
	for _, url := range urls {
		if strings.Contains(url, "clip.mp4") {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected video not found")
	}
}

func TestIsValidMediaURL(t *testing.T) {
	tests := []struct {
		url   string
		valid bool
	}{
		{"https://example.com/image.jpg", true},
		{"https://cdn.example.com/video.mp4", true},
		{"ftp://example.com/file.jpg", false},
		{"data:image/png;base64,abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			result := isValidMediaURL(tt.url)
			if result != tt.valid {
				t.Errorf("isValidMediaURL(%q) = %v, want %v", tt.url, result, tt.valid)
			}
		})
	}
}
