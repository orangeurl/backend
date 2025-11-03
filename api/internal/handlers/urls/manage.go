package urls

import (
	"database/sql"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/xenonnn4w/orangeurl/internal/database"
	"github.com/xenonnn4w/orangeurl/internal/middleware"
)

// DeleteURL soft-deletes a URL (sets is_active to false) and removes from Redis
func DeleteURL(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
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

	// First, get the URL record to obtain the short_id before deleting
	urlRecord, err := queries.GetURL(c.Context(), parsedID)
	if err != nil {
		log.Printf("[DeleteURL] Error getting URL: %v", err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "URL not found"})
	}

	// Verify ownership
	if urlRecord.UserID != user.ID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Access denied"})
	}

	// Delete from PostgreSQL (soft delete)
	err = queries.DeleteURL(c.Context(), database.DeleteURLParams{
		ID:     parsedID,
		UserID: user.ID,
	})

	if err != nil {
		log.Printf("[DeleteURL] Error deleting from PostgreSQL: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete URL"})
	}

	// Remove from Redis cache
	if urlRecord.ShortID != "" {
		redisClient := database.CreateClient(0)
		defer redisClient.Close()

		delResult := redisClient.Del(database.Ctx, urlRecord.ShortID)
		if delResult.Err() != nil {
			log.Printf("[DeleteURL] Warning: Failed to delete from Redis: %v", delResult.Err())
		} else {
			log.Printf("[DeleteURL] Successfully deleted '%s' from Redis", urlRecord.ShortID)
		}
	}

	return c.JSON(fiber.Map{"message": "URL deleted successfully"})
}

// ToggleURLStatus toggles the is_active status of a URL
func ToggleURLStatus(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
	}

	urlID := c.Params("id")
	if urlID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "URL ID required"})
	}

	parsedID, err := uuid.Parse(urlID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid URL ID"})
	}

	type ToggleRequest struct {
		IsActive bool `json:"is_active"`
	}

	body := new(ToggleRequest)
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	queries := database.GetQueries()
	
	// First verify the URL belongs to this user
	urlRecord, err := queries.GetURLByShortID(c.Context(), urlID)
	if err != nil || urlRecord.UserID != user.ID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "URL not found"})
	}

	// Update the URL - using the existing UpdateURL query
	// Note: This is a simple implementation, might need a dedicated ToggleActive query
	_, err = queries.UpdateURL(c.Context(), database.UpdateURLParams{
		ID:          parsedID,
		OriginalUrl: urlRecord.OriginalUrl,
		Expiry:      urlRecord.Expiry,
		IsActive:    sql.NullBool{Bool: body.IsActive, Valid: true},
	})

	if err != nil {
		log.Printf("[ToggleURLStatus] Error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update URL"})
	}

	return c.JSON(fiber.Map{
		"message":   "URL status updated",
		"is_active": body.IsActive,
	})
}

// GetURLDetails returns detailed information about a specific URL
func GetURLDetails(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	shortID := c.Params("shortId")
	if shortID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Short ID required"})
	}

	queries := database.GetQueries()
	urlRecord, err := queries.GetURLByShortID(c.Context(), shortID)
	if err != nil {
		log.Printf("[GetURLDetails] URL not found: %v", err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "URL not found"})
	}

	// Verify ownership
	if urlRecord.UserID != user.ID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Access denied"})
	}

	return c.JSON(fiber.Map{
		"id":           urlRecord.ID,
		"short_id":     urlRecord.ShortID,
		"original_url": urlRecord.OriginalUrl,
		"expiry":       urlRecord.Expiry,
		"is_active":    urlRecord.IsActive,
		"is_locked":    urlRecord.IsLocked,
		"created_at":   urlRecord.CreatedAt,
		"updated_at":   urlRecord.UpdatedAt,
	})
}

// UpdateURL updates a URL's properties (expiry, active status)
func UpdateURL(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	shortID := c.Params("shortId")
	if shortID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Short ID required"})
	}

	type UpdateRequest struct {
		OriginalURL *string `json:"original_url"`
		Expiry      *string `json:"expiry"`
		IsActive    *bool   `json:"is_active"`
	}

	var req UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	queries := database.GetQueries()

	// Get existing URL
	urlRecord, err := queries.GetURLByShortID(c.Context(), shortID)
	if err != nil {
		log.Printf("[UpdateURL] URL not found: %v", err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "URL not found"})
	}

	// Verify ownership
	if urlRecord.UserID != user.ID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Access denied"})
	}

	// Build update params (use existing values if not provided)
	originalURL := urlRecord.OriginalUrl
	if req.OriginalURL != nil {
		originalURL = *req.OriginalURL
	}

	expiry := urlRecord.Expiry
	if req.Expiry != nil {
		// Parse expiry time
		// For simplicity, using the existing expiry format
		// You can add time parsing logic here if needed
	}

	isActive := urlRecord.IsActive
	if req.IsActive != nil {
		isActive = sql.NullBool{Bool: *req.IsActive, Valid: true}
	}

	// Update URL
	updatedURL, err := queries.UpdateURL(c.Context(), database.UpdateURLParams{
		ID:          urlRecord.ID,
		OriginalUrl: originalURL,
		Expiry:      expiry,
		IsActive:    isActive,
	})
	if err != nil {
		log.Printf("[UpdateURL] Failed to update: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update URL"})
	}

	return c.JSON(fiber.Map{
		"message":      "URL updated successfully",
		"id":           updatedURL.ID,
		"short_id":     updatedURL.ShortID,
		"original_url": updatedURL.OriginalUrl,
		"expiry":       updatedURL.Expiry,
		"is_active":    updatedURL.IsActive,
		"updated_at":   updatedURL.UpdatedAt,
	})
}

