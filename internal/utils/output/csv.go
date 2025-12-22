package output

import (
	"encoding/csv"
	"os"
	"sort"

	"github.com/law-makers/crawl/pkg/models"
)

// SaveCSV writes page data to a CSV file. Returns an error on failure.
func SaveCSV(data *models.PageData, filepath string) error {
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// If we have structured data (from --fields), use that
	if len(data.Structured) > 0 {
		// Get headers from the first item
		var headers []string
		firstItem := data.Structured[0]
		for k := range firstItem {
			headers = append(headers, k)
		}
		sort.Strings(headers)

		if err := writer.Write(headers); err != nil {
			return err
		}

		for _, item := range data.Structured {
			var row []string
			for _, h := range headers {
				row = append(row, item[h])
			}
			if err := writer.Write(row); err != nil {
				return err
			}
		}
	} else if len(data.Data) > 0 {
		// If we have list data but no fields, just dump Text and HTML
		if err := writer.Write([]string{"Text", "HTML"}); err != nil {
			return err
		}
		for _, item := range data.Data {
			if err := writer.Write([]string{item.Text, item.HTML}); err != nil {
				return err
			}
		}
	} else {
		// Fallback for single page content
		if err := writer.Write([]string{"Content", "HTML"}); err != nil {
			return err
		}
		if err := writer.Write([]string{data.Content, data.HTML}); err != nil {
			return err
		}
	}

	return nil
}
