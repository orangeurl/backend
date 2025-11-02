package url

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// ValidateURLProtocol checks if URL uses only http or https protocols
// Blocks dangerous protocols like javascript:, data:, file:, etc.
func ValidateURLProtocol(rawURL string) error {
	// Parse the URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// If no scheme, it will be added by EnforceHTTP, so allow it
	if parsedURL.Scheme == "" {
		return nil
	}

	// Only allow http and https
	scheme := strings.ToLower(parsedURL.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("invalid protocol '%s': only http and https are allowed", parsedURL.Scheme)
	}

	return nil
}

func EnforceHTTP(urlStr string) string {
	// Check if URL is too short or doesn't start with http/https
	if len(urlStr) < 4 {
		return "https://" + urlStr
	}

	// Normalize to lowercase for comparison
	lowerURL := strings.ToLower(urlStr)

	// Block dangerous protocols
	dangerousProtocols := []string{
		"javascript:",
		"data:",
		"vbscript:",
		"file:",
		"about:",
		"blob:",
	}

	for _, protocol := range dangerousProtocols {
		if strings.HasPrefix(lowerURL, protocol) {
			// Return empty string for dangerous protocols
			// This will be caught by validation later
			return ""
		}
	}

	// If already has http:// or https://, return as is
	if strings.HasPrefix(urlStr, "http://") || strings.HasPrefix(urlStr, "https://") {
		return urlStr
	}

	// Add https:// prefix for URLs without protocol
	return "https://" + urlStr
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
