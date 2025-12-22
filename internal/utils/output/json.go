package output

import (
	"encoding/json"
	"os"

	urlutil "github.com/law-makers/crawl/internal/utils/url"
	"github.com/law-makers/crawl/pkg/models"
)

// SaveJSON writes a compacted JSON export of the PageData (HTML removed) to filepath.
func SaveJSON(data *models.PageData, filepath string) error {
	// Create a copy to avoid modifying the original data
	exportData := *data
	exportData.HTML = "" // Remove HTML from JSON export
	urlutil.ResolveRelativeLinks(&exportData)

	content, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath, content, 0644)
}
