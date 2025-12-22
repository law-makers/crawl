package headers

import (
	"strings"
)

// ParseHeaders converts an array of header strings ("Key: Value") into a map
func ParseHeaders(h []string) map[string]string {
	m := make(map[string]string)
	for _, hdr := range h {
		parts := strings.SplitN(hdr, ":", 2)
		if len(parts) == 2 {
			m[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return m
}
