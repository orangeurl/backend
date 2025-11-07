package dashboard

import (
	"log"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/xenonnn4w/orangeurl/internal/database"
	"github.com/xenonnn4w/orangeurl/internal/middleware"
)

// GetRecentClicks returns recent URL clicks for Zapier polling
func GetRecentClicks(c *fiber.Ctx) error {
	// Get API key from context
	apiKey, err := middleware.GetAPIKeyFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "API key required"})
	}

	// Get user from API key
	queries := database.GetQueries()
	if queries == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "database not initialized"})
	}

	user, err := queries.GetUserByID(c.Context(), apiKey.UserID)
	if err != nil {
		log.Printf("[GetRecentClicks] Failed to get user: %v", err)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "user not found"})
	}

	// Fetch recent clicks (limit to 3 for testing purposes)
	clicks, err := queries.GetRecentClicksWithURL(c.Context(), database.GetRecentClicksWithURLParams{
		UserID: user.ID,
		Limit:  3,
	})
	if err != nil {
		log.Printf("[GetRecentClicks] Failed to fetch clicks: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch clicks"})
	}

	// Get domain for constructing short URLs
	host := os.Getenv("DOMAIN")
	if host == "" {
		host = os.Getenv("PUBLIC_HOST")
	}
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		host = "https://" + host
	}

	// Transform clicks to match Zapier webhook payload structure EXACTLY
	result := make([]fiber.Map, 0)
	for _, click := range clicks {
		// Format clicked_at as RFC3339 string
		clickedAtStr := ""
		if click.ClickedAt.Valid {
			clickedAtStr = click.ClickedAt.Time.Format("2006-01-02T15:04:05Z07:00")
		}

		clickData := fiber.Map{
			"short_url":      host + "/" + click.ShortID,
			"short_id":       click.ShortID,
			"long_url":       click.OriginalUrl,
			"clicked_at":     clickedAtStr,
			"referer":        click.Referer.String,
			"referer_source": extractRefererSource(click.Referer.String),
			"country":        click.Country.String,
			"city":           click.City.String,
			"device_type":    click.DeviceType.String,
			"browser":        click.Browser.String,
			"os":             click.Os.String,
			"is_bot":         click.IsBot.Bool,
		}
		result = append(result, clickData)
	}

	return c.JSON(fiber.Map{
		"clicks": result,
	})
}

// extractRefererSource extracts the source name from a referer URL
func extractRefererSource(referer string) string {
	if referer == "" {
		return "Direct"
	}

	referer = strings.ToLower(referer)

	// Check for common sources
	if strings.Contains(referer, "google") {
		return "Google"
	} else if strings.Contains(referer, "facebook") || strings.Contains(referer, "fb.") {
		return "Facebook"
	} else if strings.Contains(referer, "twitter") || strings.Contains(referer, "t.co") {
		return "X"
	} else if strings.Contains(referer, "linkedin") {
		return "LinkedIn"
	} else if strings.Contains(referer, "instagram") {
		return "Instagram"
	} else if strings.Contains(referer, "youtube") {
		return "YouTube"
	} else if strings.Contains(referer, "reddit") {
		return "Reddit"
	} else if strings.Contains(referer, "tiktok") {
		return "TikTok"
	}

	return "Other"
}
