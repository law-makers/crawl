package urlutil

import "testing"

func TestValidate(t *testing.T) {
	valid := []string{
		"http://example.com",
		"https://example.com/path",
	}
	for _, u := range valid {
		if err := ValidateURL(u); err != nil {
			t.Fatalf("expected valid, got error: %v", err)
		}
	}

	invalid := []string{"ftp://example.com", "//example.com", "http:///"}
	for _, u := range invalid {
		if err := ValidateURL(u); err == nil {
			t.Fatalf("expected invalid for %s", u)
		}
	}
}
