package routes

import (
	"database/sql"
	"net"
	"strings"

	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/sqlc-dev/pqtype"
	"github.com/xenonnn4w/orangeurl/internal/database"
)

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

func ResolveURL(c *fiber.Ctx) error {
	url := c.Params("url")

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
	queries := database.GetQueries()
	urlRecord, pgErr := queries.GetURLByShortID(c.Context(), url)
	if pgErr == nil {
		// URL found in Postgres, track the click
		userAgent := c.Get("User-Agent")
		referer := c.Get("Referer")
		ipAddress := c.IP()

		// Parse user agent data
		deviceType := parseDeviceType(userAgent)
		browser := parseBrowser(userAgent)
		os := parseOS(userAgent)
		botDetected := isBot(userAgent)

		// Create click record
		// Note: country and city would need a GeoIP service, using null for now
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
			Country:    sql.NullString{Valid: false}, // TODO: Implement GeoIP lookup
			City:       sql.NullString{Valid: false}, // TODO: Implement GeoIP lookup
			DeviceType: sql.NullString{String: deviceType, Valid: true},
			Browser:    sql.NullString{String: browser, Valid: true},
			Os:         sql.NullString{String: os, Valid: true},
			IsBot:      sql.NullBool{Bool: botDetected, Valid: true},
		})
	}

	return c.Redirect(value, 301)
}

