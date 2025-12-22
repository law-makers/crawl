// internal/engine/batch/grouping.go
package batch

import (
	"net/url"

	"github.com/law-makers/crawl/pkg/models"
)

// GroupByDomain groups requests by their domain for better HTTP/2 multiplexing
func GroupByDomain(requests []models.RequestOptions) map[string][]models.RequestOptions {
	groups := make(map[string][]models.RequestOptions)

	for _, req := range requests {
		u, err := url.Parse(req.URL)
		if err != nil {
			// If URL parsing fails, use a default group
			groups["default"] = append(groups["default"], req)
			continue
		}

		domain := u.Host
		groups[domain] = append(groups[domain], req)
	}

	return groups
}
