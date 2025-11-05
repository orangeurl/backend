package webhooks

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ValidateWebhookURL validates a webhook URL to prevent SSRF attacks
func ValidateWebhookURL(rawURL string) error {
	// Parse URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Only allow HTTP and HTTPS schemes
	scheme := strings.ToLower(parsedURL.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("only HTTP and HTTPS protocols are allowed")
	}

	// Require HTTPS in production (optional: can be enforced based on env variable)
	// For now, we allow HTTP for local development but recommend HTTPS

	// Extract hostname
	hostname := parsedURL.Hostname()
	if hostname == "" {
		return fmt.Errorf("URL must contain a hostname")
	}

	// Resolve hostname to IP addresses
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return fmt.Errorf("failed to resolve hostname: %w", err)
	}

	if len(ips) == 0 {
		return fmt.Errorf("hostname does not resolve to any IP address")
	}

	// Check each resolved IP for private/internal addresses
	for _, ip := range ips {
		if err := validateIP(ip); err != nil {
			return err
		}
	}

	// Additional checks for well-known metadata endpoints
	if isMetadataEndpoint(hostname) {
		return fmt.Errorf("requests to cloud metadata endpoints are not allowed")
	}

	return nil
}

// validateIP checks if an IP address is private, loopback, or otherwise restricted
func validateIP(ip net.IP) error {
	// Reject loopback addresses (127.0.0.0/8, ::1)
	if ip.IsLoopback() {
		return fmt.Errorf("loopback addresses are not allowed")
	}

	// Reject private IP ranges (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, fc00::/7)
	if ip.IsPrivate() {
		return fmt.Errorf("private IP addresses are not allowed")
	}

	// Reject link-local addresses (169.254.0.0/16, fe80::/10)
	if ip.IsLinkLocalUnicast() {
		return fmt.Errorf("link-local addresses are not allowed")
	}

	// Reject multicast addresses
	if ip.IsMulticast() {
		return fmt.Errorf("multicast addresses are not allowed")
	}

	// Reject unspecified addresses (0.0.0.0, ::)
	if ip.IsUnspecified() {
		return fmt.Errorf("unspecified addresses are not allowed")
	}

	// Additional IPv4-specific checks
	if ipv4 := ip.To4(); ipv4 != nil {
		// Reject 0.0.0.0/8 (current network)
		if ipv4[0] == 0 {
			return fmt.Errorf("addresses in 0.0.0.0/8 are not allowed")
		}

		// Reject 192.0.0.0/24 (IETF Protocol Assignments)
		if ipv4[0] == 192 && ipv4[1] == 0 && ipv4[2] == 0 {
			return fmt.Errorf("addresses in 192.0.0.0/24 are not allowed")
		}

		// Reject 192.0.2.0/24 (TEST-NET-1)
		if ipv4[0] == 192 && ipv4[1] == 0 && ipv4[2] == 2 {
			return fmt.Errorf("addresses in TEST-NET ranges are not allowed")
		}

		// Reject 198.51.100.0/24 (TEST-NET-2)
		if ipv4[0] == 198 && ipv4[1] == 51 && ipv4[2] == 100 {
			return fmt.Errorf("addresses in TEST-NET ranges are not allowed")
		}

		// Reject 203.0.113.0/24 (TEST-NET-3)
		if ipv4[0] == 203 && ipv4[1] == 0 && ipv4[2] == 113 {
			return fmt.Errorf("addresses in TEST-NET ranges are not allowed")
		}

		// Reject 224.0.0.0/4 (Multicast - already covered by IsMulticast)
		// Reject 240.0.0.0/4 (Reserved for future use)
		if ipv4[0] >= 240 {
			return fmt.Errorf("addresses in 240.0.0.0/4 are reserved")
		}
	}

	return nil
}

// isMetadataEndpoint checks for well-known cloud metadata endpoints
func isMetadataEndpoint(hostname string) bool {
	hostname = strings.ToLower(hostname)

	metadataEndpoints := []string{
		"169.254.169.254",           // AWS, Azure, GCP, DigitalOcean
		"metadata.google.internal",  // GCP
		"metadata",                  // Generic
		"instance-data",             // AWS alternative
		"fd00:ec2::254",            // AWS IPv6
	}

	for _, endpoint := range metadataEndpoints {
		if hostname == endpoint {
			return true
		}
	}

	return false
}
