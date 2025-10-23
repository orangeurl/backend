package utils

import (
	"net"
	"strings"
)

// ParseUserAgent extracts browser and OS from user agent string
func ParseUserAgent(userAgent string) (browser, os, deviceType string) {
	ua := strings.ToLower(userAgent)
	
	// Detect Browser
	switch {
	case strings.Contains(ua, "edg"):
		browser = "Edge"
	case strings.Contains(ua, "chrome") && !strings.Contains(ua, "edg"):
		browser = "Chrome"
	case strings.Contains(ua, "safari") && !strings.Contains(ua, "chrome"):
		browser = "Safari"
	case strings.Contains(ua, "firefox"):
		browser = "Firefox"
	case strings.Contains(ua, "opera") || strings.Contains(ua, "opr"):
		browser = "Opera"
	case strings.Contains(ua, "msie") || strings.Contains(ua, "trident"):
		browser = "Internet Explorer"
	default:
		browser = "Unknown"
	}

	// Detect OS
	switch {
	case strings.Contains(ua, "windows"):
		os = "Windows"
	case strings.Contains(ua, "mac"):
		os = "macOS"
	case strings.Contains(ua, "linux"):
		os = "Linux"
	case strings.Contains(ua, "android"):
		os = "Android"
	case strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad"):
		os = "iOS"
	default:
		os = "Unknown"
	}

	// Detect Device Type
	switch {
	case strings.Contains(ua, "mobile") || strings.Contains(ua, "android") || strings.Contains(ua, "iphone"):
		deviceType = "Mobile"
	case strings.Contains(ua, "tablet") || strings.Contains(ua, "ipad"):
		deviceType = "Tablet"
	default:
		deviceType = "Desktop"
	}

	return browser, os, deviceType
}

// IsBot checks if the user agent is a bot
func IsBot(userAgent string) bool {
	ua := strings.ToLower(userAgent)
	botKeywords := []string{
		"bot", "crawler", "spider", "scraper", "googlebot", "bingbot",
		"slurp", "duckduckbot", "baiduspider", "yandexbot", "facebookexternalhit",
		"linkedinbot", "twitterbot", "whatsapp", "telegram",
	}

	for _, keyword := range botKeywords {
		if strings.Contains(ua, keyword) {
			return true
		}
	}
	return false
}

// GetIPAddress extracts the real IP address from request headers
func GetIPAddress(xForwardedFor, xRealIP, remoteAddr string) string {
	// Check X-Forwarded-For header first (used by proxies)
	if xForwardedFor != "" {
		ips := strings.Split(xForwardedFor, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	if xRealIP != "" {
		return xRealIP
	}

	// Fall back to remote address
	if remoteAddr != "" {
		// Remove port if present
		host, _, err := net.SplitHostPort(remoteAddr)
		if err == nil {
			return host
		}
		return remoteAddr
	}

	return ""
}


