package routes

import (
	"github.com/gofiber/fiber/v2"
	"github.com/xenonnn4w/short-url/database"
)

type URLMapping struct {
	ShortID     string `json:"short_id"`
	OriginalURL string `json:"original_url"`
}

type AdminStats struct {
	TotalURLs int64        `json:"total_urls"`
	URLs      []URLMapping `json:"urls"`
}

// GetAllURLs - Admin endpoint to view all stored URLs
func GetAllURLs(c *fiber.Ctx) error {
	urlMap, err := database.GetAllURLs()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch URLs",
		})
	}

	// Convert map to array for better JSON response
	urls := make([]URLMapping, 0, len(urlMap))
	for shortID, originalURL := range urlMap {
		urls = append(urls, URLMapping{
			ShortID:     shortID,
			OriginalURL: originalURL,
		})
	}

	totalCount, _ := database.GetTotalURLCount()

	stats := AdminStats{
		TotalURLs: totalCount,
		URLs:      urls,
	}

	return c.Status(fiber.StatusOK).JSON(stats)
}

// GetURLStats - Admin endpoint for statistics only
func GetURLStats(c *fiber.Ctx) error {
	totalCount, err := database.GetTotalURLCount()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch stats",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"total_urls": totalCount,
	})
}
