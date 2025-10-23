package routes

import (
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/xenonnn4w/orangeurl/internal/database"
	"github.com/xenonnn4w/orangeurl/internal/handlers/tracking"
	"github.com/xenonnn4w/orangeurl/internal/utils"
)

func ResolveURL(c *fiber.Ctx) error {
	shortID := c.Params("url")

	// Get URL from PostgreSQL first
	queries := database.GetQueries()
	urlRecord, err := queries.GetURLByShortID(c.Context(), shortID)
	
	if err != nil {
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
	}

	// Check if URL is expired
	if urlRecord.Expiry.Valid && urlRecord.Expiry.Time.Before(time.Now()) {
		c.Status(410)
		return c.Type("html").SendString(`
			<!DOCTYPE html>
			<html>
			<head>
				<meta http-equiv="refresh" content="0;url=https://app.orangeurl.live/broken-link">
				<title>Link Expired</title>
			</head>
			<body>
				<p>This link has expired. Redirecting...</p>
				<script>window.location.href='https://app.orangeurl.live/broken-link';</script>
			</body>
			</html>
		`)
	}

	// Get the original URL
	originalURL := urlRecord.OriginalUrl

	// Also check Redis cache as fallback/sync
	r := database.CreateClient(0)
	defer r.Close()

	cachedURL, err := r.Get(database.Ctx, shortID).Result()
	if err == redis.Nil {
		// Cache miss - store in Redis for future requests
		if err := database.CacheURL(shortID, originalURL, 0); err != nil {
			log.Printf("Error caching URL in Redis: %v", err)
		}
	} else if err == nil && cachedURL != originalURL {
		// Sync issue - update Redis
		if err := database.CacheURL(shortID, originalURL, 0); err != nil {
			log.Printf("Error syncing URL to Redis: %v", err)
		}
	}

	// Track the click asynchronously
	go func() {
		// Extract request information
		ipAddress := utils.GetIPAddress(
			c.Get("X-Forwarded-For"),
			c.Get("X-Real-IP"),
			c.IP(),
		)
		userAgent := c.Get("User-Agent")
		referer := c.Get("Referer")

		// Track the click
		if err := tracking.TrackClick(urlRecord.ID, ipAddress, userAgent, referer); err != nil {
			log.Printf("Error tracking click: %v", err)
		}
	}()

	// Redirect to original URL
	return c.Redirect(originalURL, 301)
}

