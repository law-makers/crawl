package batch

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/law-makers/crawl/pkg/models"
)

type mockScraper struct{}

func (m *mockScraper) Name() string { return "mock" }
func (m *mockScraper) Fetch(opts models.RequestOptions) (*models.PageData, error) {
	time.Sleep(10 * time.Millisecond)
	if opts.URL == "error" {
		return nil, errors.New("fetch error")
	}
	return &models.PageData{URL: opts.URL}, nil
}

func TestBatchScraper(t *testing.T) {
	scraper := &mockScraper{}
	batch := New(scraper, 2)

	requests := []models.RequestOptions{
		{URL: "url1"},
		{URL: "url2"},
		{URL: "url3"},
		{URL: "error"},
	}

	results := batch.ScrapeBatch(context.Background(), requests)

	count := 0
	errors := 0
	for res := range results {
		count++
		if res.Error != nil {
			errors++
		}
	}

	if count != 4 {
		t.Errorf("Expected 4 results, got %d", count)
	}
	if errors != 1 {
		t.Errorf("Expected 1 error, got %d", errors)
	}
}
