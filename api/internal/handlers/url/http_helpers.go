package url

import (
	"os"
	"strings"
)

func EnforceHTTP(url string) string {
	// Check if URL is too short or doesn't start with http/https
	if len(url) < 4 {
		return "https://" + url
	}

	// If already has http:// or https://, return as is
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return url
	}

	// Add https:// prefix for URLs without protocol
	return "https://" + url
}

func RemoveDomainError(url string) bool {
	if url == os.Getenv("DOMAIN") {
		return false
	}
	newURL := strings.Replace(url, "http://", "", 1)
	newURL = strings.Replace(newURL, "https://", "", 1)
	newURL = strings.Replace(newURL, "www.", "", 1)
	newURL = strings.Split(newURL, "/")[0]

	if newURL == os.Getenv("DOMAIN") {
		return false
	}
	return true
}
