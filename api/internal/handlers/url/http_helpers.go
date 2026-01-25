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

// SuspiciousDomains contains domains commonly used for phishing and abuse
// These are free DNS services and tunneling services often abused
var SuspiciousDomains = []string{
	// Free dynamic DNS services (commonly abused)
	"dpdns.org",
	"duckdns.org",
	"freedns.afraid.org",
	"no-ip.org",
	"no-ip.com",
	"noip.com",
	"dynu.com",
	"dynv6.com",
	"freeddns.org",
	"chickenkiller.com",
	"ddns.net",
	"hopto.org",
	"myftp.org",
	"sytes.net",
	"zapto.org",
	
	// Tunneling services (can hide origin)
	"ngrok.io",
	"ngrok.app",
	"ngrok-free.app",
	"serveo.net",
	"localhost.run",
	"localtunnel.me",
	"loca.lt",
	"telebit.io",
	
	// URL shorteners (prevent redirect chains)
	"bit.ly",
	"tinyurl.com",
	"t.co",
	"goo.gl",
	"ow.ly",
	"is.gd",
	"buff.ly",
	"adf.ly",
	"bit.do",
	
	// Known phishing TLDs/domains
	"tk", // .tk free domain
	"ml", // .ml free domain
	"ga", // .ga free domain
	"cf", // .cf free domain
	"gq", // .gq free domain
}

// IsSuspiciousDomain checks if the URL uses a known suspicious/phishing domain
func IsSuspiciousDomain(rawURL string) (bool, string) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return true, "invalid URL format"
	}

	hostname := strings.ToLower(parsedURL.Hostname())
	
	// Check against suspicious domain list
	for _, domain := range SuspiciousDomains {
		// Check if hostname ends with the suspicious domain
		if strings.HasSuffix(hostname, domain) || hostname == domain {
			return true, fmt.Sprintf("domain '%s' is not allowed", domain)
		}
		
		// Check for free TLDs (just the extension)
		if strings.HasPrefix(domain, ".") == false && len(domain) <= 3 {
			// This is a TLD check (like tk, ml, ga)
			if strings.HasSuffix(hostname, "."+domain) {
				return true, fmt.Sprintf("TLD '.%s' is not allowed due to high abuse rate", domain)
			}
		}
	}

	return false, ""
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
