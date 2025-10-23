package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type GeoLocation struct {
	Country string `json:"country"`
	City    string `json:"city"`
}

// GetLocationFromIP fetches geolocation data from IP address using ip-api.com
func GetLocationFromIP(ip string) (country, city string) {
	// Default values
	country = "Unknown"
	city = "Unknown"

	// Skip private IPs
	if ip == "" || ip == "127.0.0.1" || ip == "::1" {
		return
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	// Use ip-api.com free API (no key required, 45 req/min limit)
	url := fmt.Sprintf("http://ip-api.com/json/%s?fields=country,city", ip)
	
	resp, err := client.Get(url)
	if err != nil {
		// If API fails, return defaults
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var location GeoLocation
	if err := json.Unmarshal(body, &location); err != nil {
		return
	}

	if location.Country != "" {
		country = location.Country
	}
	if location.City != "" {
		city = location.City
	}

	return country, city
}

// GetLocationFromIPCached is a wrapper that could include caching logic
func GetLocationFromIPCached(ip string) (country, city string) {
	// For now, directly call the API
	// In production, you might want to add Redis caching here
	return GetLocationFromIP(ip)
}


