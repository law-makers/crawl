package engine

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dop251/goja"
	"github.com/law-makers/crawl/pkg/models"
	"github.com/rs/zerolog/log"
)

// HybridScraper combines static scraping with lightweight JS execution
type HybridScraper struct {
	static  *StaticScraper
	dynamic *DynamicScraper
}

// NewHybridScraper creates a new HybridScraper with the provided scrapers
func NewHybridScraper(staticScraper *StaticScraper, dynamicScraper *DynamicScraper) *HybridScraper {
	return &HybridScraper{
		static:  staticScraper,
		dynamic: dynamicScraper,
	}
}

// Name returns the name of the scraper
func (s *HybridScraper) Name() string {
	return "HybridScraper"
}

// Fetch retrieves data using static scraper and then executes inline scripts
func (s *HybridScraper) Fetch(opts models.RequestOptions) (*models.PageData, error) {
	// 1. Fetch with static scraper
	data, doc, err := s.static.FetchWithDoc(opts)
	if err != nil {
		return nil, err
	}

	// 2. Execute JS if needed
	// We only execute if we found scripts and the user didn't explicitly ask for static only
	// (Though HybridScraper implies we want JS)
	if len(data.Scripts) > 0 || strings.Contains(data.HTML, "<script") {
		s.executeScripts(data, doc)
	}

	return data, nil
}

func (s *HybridScraper) executeScripts(data *models.PageData, doc *goquery.Document) {
	vm := goja.New()

	// Mock basic browser environment
	// This is very limited, just enough to capture data assignments
	vm.Set("window", vm.GlobalObject())
	vm.Set("self", vm.GlobalObject())
	vm.Set("document", map[string]interface{}{
		"location": map[string]interface{}{
			"href": data.URL,
		},
	})
	vm.Set("location", map[string]interface{}{
		"href": data.URL,
	})
	vm.Set("console", map[string]interface{}{
		"log": func(call goja.FunctionCall) goja.Value {
			return nil
		},
		"error": func(call goja.FunctionCall) goja.Value {
			return nil
		},
	})

	// Use the existing document to find inline scripts
	if doc == nil {
		// Fallback if doc is missing (shouldn't happen with FetchWithDoc)
		var err error
		doc, err = goquery.NewDocumentFromReader(strings.NewReader(data.HTML))
		if err != nil {
			log.Warn().Err(err).Msg("Failed to parse HTML for JS execution")
			return
		}
	}

	doc.Find("script").Each(func(i int, sel *goquery.Selection) {
		// Skip external scripts
		if _, exists := sel.Attr("src"); exists {
			return
		}

		// Execute inline script
		scriptContent := sel.Text()
		if scriptContent != "" {
			_, err := vm.RunString(scriptContent)
			if err != nil {
				// Ignore errors, most scripts will fail due to missing DOM
				// log.Debug().Err(err).Msg("Script execution failed (expected)")
			}
		}
	})

	// Extract global variables that might contain data
	// We look for common data patterns
	keys := vm.GlobalObject().Keys()
	for _, key := range keys {
		// Filter out standard globals
		if isStandardGlobal(key) {
			continue
		}

		val := vm.Get(key)
		if val != nil {
			// Store in metadata or a new field?
			// For now, let's put it in Metadata with a prefix
			exportVal := val.Export()
			if exportVal != nil {
				data.Metadata["js:"+key] = fmt.Sprintf("%v", exportVal)
			}
		}
	}
}

func isStandardGlobal(key string) bool {
	standards := map[string]bool{
		"window": true, "self": true, "document": true, "location": true, "console": true,
		"Object": true, "Array": true, "String": true, "Number": true, "Boolean": true,
		"Date": true, "Math": true, "JSON": true, "RegExp": true, "Error": true,
		"Function": true, "parseInt": true, "parseFloat": true, "isNaN": true,
		"isFinite": true, "encodeURI": true, "decodeURI": true, "encodeURIComponent": true,
		"decodeURIComponent": true, "undefined": true, "NaN": true, "Infinity": true,
	}
	return standards[key]
}
