package admin

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/xenonnn4w/orangeurl/internal/database"
	"github.com/xenonnn4w/orangeurl/internal/middleware"
)

// AdminListAllURLs returns all URLs (including inactive) for admin management
func AdminListAllURLs(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	if !user.IsAdmin.Valid || !user.IsAdmin.Bool {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin access required"})
	}

	queries := database.GetQueries()
	urls, err := queries.AdminGetAllURLs(c.Context())
	if err != nil {
		log.Printf("[AdminListAllURLs] Error fetching URLs: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch URLs"})
	}

	result := make([]fiber.Map, 0, len(urls))
	for _, u := range urls {
		item := fiber.Map{
			"id":           u.ID,
			"short_id":     u.ShortID,
			"original_url": u.OriginalUrl,
			"is_active":    u.IsActive.Valid && u.IsActive.Bool,
			"is_locked":    u.IsLocked.Valid && u.IsLocked.Bool,
			"created_at":   u.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
			"owner_email":  u.OwnerEmail,
		}
		if u.Expiry.Valid {
			item["expiry"] = u.Expiry.Time.Format("2006-01-02T15:04:05Z07:00")
		}
		result = append(result, item)
	}

	return c.JSON(fiber.Map{
		"urls":  result,
		"count": len(result),
	})
}

// AdminHardDeleteURL permanently deletes a URL from PostgreSQL and Redis
func AdminHardDeleteURL(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	if !user.IsAdmin.Valid || !user.IsAdmin.Bool {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin access required"})
	}

	urlID := c.Params("id")
	if urlID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "URL ID required"})
	}

	parsedID, err := uuid.Parse(urlID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid URL ID"})
	}

	queries := database.GetQueries()

	// Get the URL record first to obtain short_id for Redis cleanup
	urlRecord, err := queries.AdminGetURLByID(c.Context(), parsedID)
	if err != nil {
		log.Printf("[AdminHardDeleteURL] URL not found: %v", err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "URL not found"})
	}

	// 1. Delete click analytics
	if err := queries.AdminDeleteURLClicks(c.Context(), parsedID); err != nil {
		log.Printf("[AdminHardDeleteURL] Warning: Failed to delete clicks: %v", err)
	}

	// 2. Hard delete from PostgreSQL
	if err := queries.AdminHardDeleteURL(c.Context(), parsedID); err != nil {
		log.Printf("[AdminHardDeleteURL] Error deleting from PostgreSQL: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete URL from database"})
	}

	// 3. Remove from Redis cache
	if urlRecord.ShortID != "" {
		redisClient := database.CreateClient(0)
		defer redisClient.Close()

		delResult := redisClient.Del(database.Ctx, urlRecord.ShortID)
		if delResult.Err() != nil {
			log.Printf("[AdminHardDeleteURL] Warning: Failed to delete from Redis: %v", delResult.Err())
		} else {
			log.Printf("[AdminHardDeleteURL] Deleted '%s' from Redis", urlRecord.ShortID)
		}
	}

	log.Printf("[AdminHardDeleteURL] Admin %s permanently deleted URL '%s' (ID: %s)", user.Email, urlRecord.ShortID, urlID)

	return c.JSON(fiber.Map{
		"message":  "URL permanently deleted from PostgreSQL and Redis",
		"short_id": urlRecord.ShortID,
		"id":       urlID,
	})
}
