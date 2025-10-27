package analytics

import (
	"github.com/gofiber/fiber/v2"
	"github.com/xenonnn4w/orangeurl/internal/database"
	"github.com/xenonnn4w/orangeurl/internal/middleware"
)

// GetUserAnalytics returns all analytics data for the authenticated user
func GetUserAnalytics(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
	}

	queries := database.GetQueries()

	// Get all user analytics
	analytics, err := queries.GetUserAnalytics(c.Context(), user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch analytics"})
	}

	// Get aggregated stats
	stats, err := queries.GetUserClickStats(c.Context(), user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch stats"})
	}

	// Get top countries
	countries, _ := queries.GetUserClicksByCountry(c.Context(), user.ID)

	response := fiber.Map{
		"total_clicks":      stats.TotalClicks,
		"unique_urls":       stats.UniqueUrlsClicked,
		"unique_countries":  stats.UniqueCountries,
		"unique_cities":     stats.UniqueCities,
		"top_countries":     countries,
		"recent_clicks":     analytics[:min(len(analytics), 100)], // Last 100 clicks
	}

	return c.JSON(response)
}

// GetAllUserURLs returns all URLs for the authenticated user with click counts
func GetAllUserURLs(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
	}

	queries := database.GetQueries()

	// Get all user URLs
	urls, err := queries.GetUserURLs(c.Context(), user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch URLs"})
	}

	// Enhance with click counts
	urlsWithStats := make([]map[string]interface{}, 0)
	for _, url := range urls {
		clicks, _ := queries.GetURLAnalytics(c.Context(), url.ID)
		urlsWithStats = append(urlsWithStats, map[string]interface{}{
			"id":           url.ID,
			"short_id":     url.ShortID,
			"original_url": url.OriginalUrl,
			"expiry":       url.Expiry,
			"is_active":    url.IsActive.Bool,
			"created_at":   url.CreatedAt,
			"updated_at":   url.UpdatedAt,
			"click_count":  len(clicks),
		})
	}

	return c.JSON(fiber.Map{
		"urls": urlsWithStats,
		"total": len(urls),
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}



