package routes

import (
	"database/sql"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/oschwald/geoip2-golang"
	"github.com/sqlc-dev/pqtype"
	"github.com/xenonnn4w/orangeurl/internal/database"
	webhookService "github.com/xenonnn4w/orangeurl/internal/services/webhooks"
)

var (
	geoIPReader *geoip2.Reader
	geoIPOnce   sync.Once
)

// initGeoIP initializes the GeoIP2 reader (called once)
func initGeoIP() {
	geoIPOnce.Do(func() {
		dbPath := os.Getenv("GEOIP_DB_PATH")
		if dbPath == "" {
			dbPath = "/usr/share/GeoIP/GeoLite2-City.mmdb" // Default path
		}

		reader, err := geoip2.Open(dbPath)
		if err != nil {
			log.Printf("[GeoIP] Failed to open GeoIP database at %s: %v (city lookup will be disabled)", dbPath, err)
			return
		}

		geoIPReader = reader
		log.Printf("[GeoIP] Successfully loaded GeoIP database from %s", dbPath)
	})
}

// GeoData holds geographic information from IP lookup
type GeoData struct {
	Country string
	City    string
}

// getGeoFromIP looks up geographic information from IP address using GeoIP2
func getGeoFromIP(ipAddress string) GeoData {
	result := GeoData{}

	// Initialize GeoIP reader if not already done
	initGeoIP()

	if geoIPReader == nil {
		return result // GeoIP database not available
	}

	ip := net.ParseIP(ipAddress)
	if ip == nil {
		return result
	}

	record, err := geoIPReader.City(ip)
	if err != nil {
		return result
	}

	// Extract country code (ISO format like "US", "GB", "CA")
	if record.Country.IsoCode != "" {
		result.Country = record.Country.IsoCode
	}

	// Extract city name in English
	if record.City.Names != nil {
		if cityName, ok := record.City.Names["en"]; ok {
			result.City = cityName
		}
	}

	return result
}

// Simple user-agent parser helpers
func parseDeviceType(ua string) string {
	ua = strings.ToLower(ua)
	if strings.Contains(ua, "mobile") || strings.Contains(ua, "android") || strings.Contains(ua, "iphone") {
		return "mobile"
	} else if strings.Contains(ua, "tablet") || strings.Contains(ua, "ipad") {
		return "tablet"
	}
	return "desktop"
}

func parseBrowser(ua string) string {
	ua = strings.ToLower(ua)
	if strings.Contains(ua, "edg") {
		return "Edge"
	} else if strings.Contains(ua, "chrome") {
		return "Chrome"
	} else if strings.Contains(ua, "firefox") {
		return "Firefox"
	} else if strings.Contains(ua, "safari") {
		return "Safari"
	}
	return "Other"
}

func parseOS(ua string) string {
	ua = strings.ToLower(ua)
	if strings.Contains(ua, "windows") {
		return "Windows"
	} else if strings.Contains(ua, "mac") || strings.Contains(ua, "darwin") {
		return "macOS"
	} else if strings.Contains(ua, "linux") {
		return "Linux"
	} else if strings.Contains(ua, "android") {
		return "Android"
	} else if strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad") {
		return "iOS"
	}
	return "Other"
}

func isBot(ua string) bool {
	ua = strings.ToLower(ua)
	botKeywords := []string{"bot", "crawler", "spider", "scraper", "curl", "wget"}
	for _, keyword := range botKeywords {
		if strings.Contains(ua, keyword) {
			return true
		}
	}
	return false
}

func parseReferrerSource(referer string) string {
	if referer == "" {
		return "Direct"
	}

	ref := strings.ToLower(referer)

	if strings.Contains(ref, "youtube.com") || strings.Contains(ref, "youtu.be") {
		return "YouTube"
	} else if strings.Contains(ref, "twitter.com") || strings.Contains(ref, "x.com") {
		return "X"
	} else if strings.Contains(ref, "instagram.com") {
		return "Instagram"
	} else if strings.Contains(ref, "linkedin.com") {
		return "LinkedIn"
	} else if strings.Contains(ref, "medium.com") {
		return "Medium"
	} else if strings.Contains(ref, "facebook.com") || strings.Contains(ref, "fb.com") {
		return "Facebook"
	} else if strings.Contains(ref, "reddit.com") {
		return "Reddit"
	} else if strings.Contains(ref, "tiktok.com") {
		return "TikTok"
	} else if strings.Contains(ref, "t.me") || strings.Contains(ref, "telegram.org") {
		return "Telegram"
	} else if strings.Contains(ref, "discord.com") || strings.Contains(ref, "discord.gg") {
		return "Discord"
	} else if strings.Contains(ref, "google.com") || strings.Contains(ref, "google.") {
		return "Google"
	}

	return "Other"
}

// extractCountryFromHeaders tries to extract country code from common CDN/proxy headers
func extractCountryFromHeaders(c *fiber.Ctx) string {
	// Check for Cloudflare country header
	if country := c.Get("CF-IPCountry"); country != "" {
		return country
	}
	
	// Check for other common geo headers
	if country := c.Get("X-Country-Code"); country != "" {
		return country
	}
	
	if country := c.Get("x-rg-country"); country != "" {
		return country
	}
	
	if country := c.Get("x-forwarded-country"); country != "" {
		return country
	}
	
	// Fastly geo header
	if country := c.Get("Fastly-Geo-Country"); country != "" {
		return country
	}
	
	return ""
}

func ResolveURL(c *fiber.Ctx) error {
	url := c.Params("url")

	// First, check if this URL is blocked (abuse reports, etc.)
	queries := database.GetQueries()
	isBlocked, _ := queries.IsURLBlocked(c.Context(), url)
	if isBlocked {
		log.Printf("[ResolveURL] ⛔ Blocked URL access attempt: %s", url)
		c.Status(403)
		return c.Type("html").SendString(`
			<!DOCTYPE html>
			<html>
			<head>
				<meta http-equiv="refresh" content="0;url=https://app.orangeurl.live/broken-link">
				<title>Link Blocked</title>
			</head>
			<body>
				<p>This link has been blocked due to abuse. Redirecting...</p>
				<script>window.location.href='https://app.orangeurl.live/broken-link';</script>
			</body>
			</html>
		`)
	}

	r := database.CreateClient(0)
	defer r.Close()

	value, err := r.Get(database.Ctx, url).Result()
	if err == redis.Nil {
		// Return 404 with HTML that redirects to broken-link page
		c.Status(404)
		return c.Type("html").SendString(`
			<!DOCTYPE html>
			<html>
			<head>
				<meta http-equiv="refresh" content="0;url=https://app.orangeurl.live/broken-link">
				<title>Link Not Found</title>
			</head>
			<body>
				<p>Redirecting to error page...</p>
				<script>window.location.href='https://app.orangeurl.live/broken-link';</script>
			</body>
			</html>
		`)
	} else if err != nil {
		// Return 500 with HTML that redirects for database errors
		c.Status(500)
		return c.Type("html").SendString(`
			<!DOCTYPE html>
			<html>
			<head>
				<meta http-equiv="refresh" content="0;url=https://app.orangeurl.live/broken-link">
				<title>Server Error</title>
			</head>
			<body>
				<p>Redirecting to error page...</p>
				<script>window.location.href='https://app.orangeurl.live/broken-link';</script>
			</body>
			</html>
		`)
	}

	rInr := database.CreateClient(1)
	defer rInr.Close()

	_ = rInr.Incr(database.Ctx, "counter")

	// Track analytics in Postgres if URL exists in database
	urlRecord, pgErr := queries.GetURLByShortID(c.Context(), url)
	if pgErr == nil {
		// URL found in Postgres, track the click
		userAgent := c.Get("User-Agent")
		referer := c.Get("Referer")
		ipAddress := c.IP()

		// Extract the first IP if multiple IPs are present (X-Forwarded-For can have multiple IPs)
		// Format: "client_ip, proxy1_ip, proxy2_ip"
		if idx := strings.Index(ipAddress, ","); idx != -1 {
			ipAddress = strings.TrimSpace(ipAddress[:idx])
		}

		// Parse user agent data
		deviceType := parseDeviceType(userAgent)
		browser := parseBrowser(userAgent)
		osName := parseOS(userAgent)
		botDetected := isBot(userAgent)

		// Try to get country from common headers set by CDNs/proxies
		country := extractCountryFromHeaders(c)
		city := ""

		// If no country from headers, or to get city, use GeoIP lookup
		if country == "" {
			geoData := getGeoFromIP(ipAddress)
			country = geoData.Country
			city = geoData.City
		} else {
			// We have country from headers, but still want city from GeoIP
			geoData := getGeoFromIP(ipAddress)
			city = geoData.City
		}

		// Create click record
		// Parse IP address using net package
		ipAddr := pqtype.Inet{Valid: false}
		if parsedIP := net.ParseIP(ipAddress); parsedIP != nil {
			// Create IPNet with the parsed IP and appropriate mask
			var ipNet net.IPNet
			if parsedIP.To4() != nil {
				// IPv4
				ipNet = net.IPNet{IP: parsedIP, Mask: net.CIDRMask(32, 32)}
			} else {
				// IPv6
				ipNet = net.IPNet{IP: parsedIP, Mask: net.CIDRMask(128, 128)}
			}
			ipAddr = pqtype.Inet{IPNet: ipNet, Valid: true}
		}

		_, _ = queries.CreateClick(c.Context(), database.CreateClickParams{
			UrlID:      urlRecord.ID,
			IpAddress:  ipAddr,
			UserAgent:  sql.NullString{String: userAgent, Valid: userAgent != ""},
			Referer:    sql.NullString{String: referer, Valid: referer != ""},
			Country:    sql.NullString{String: country, Valid: country != ""},
			City:       sql.NullString{String: city, Valid: city != ""},
			DeviceType: sql.NullString{String: deviceType, Valid: true},
			Browser:    sql.NullString{String: browser, Valid: true},
			Os:         sql.NullString{String: osName, Valid: true},
			IsBot:      sql.NullBool{Bool: botDetected, Valid: true},
		})

		// Trigger webhook for url.clicked event
		host := os.Getenv("DOMAIN")
		if host == "" {
			host = os.Getenv("PUBLIC_HOST")
		}
		if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
			host = "https://" + host
		}
		shortURL := host + "/" + url

		referrerSource := parseReferrerSource(referer)

		webhookService.TriggerWebhook("url.clicked", urlRecord.UserID, map[string]interface{}{
			"short_url":      shortURL,
			"short_id":       url,
			"long_url":       value,
			"clicked_at":     time.Now().UTC().Format(time.RFC3339),
			"referer":        referer,
			"referer_source": referrerSource,
			"country":        country,
			"city":           city,
			"device_type":    deviceType,
			"browser":        browser,
			"os":             osName,
			"is_bot":         botDetected,
		})
		log.Printf("[ResolveURL] Webhook triggered for url.clicked: %s", shortURL)
	}

	return c.Redirect(value, 301)
}

