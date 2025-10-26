package urls

import (
	"database/sql"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/xenonnn4w/orangeurl/internal/database"
	"github.com/xenonnn4w/orangeurl/internal/middleware"
)

// DeleteURL soft-deletes a URL (sets is_active to false)
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
	err = queries.DeleteURL(c.Context(), database.DeleteURLParams{
		ID:     parsedID,
		UserID: user.ID,
	})

	if err != nil {
		log.Printf("[DeleteURL] Error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete URL"})
	}

	// Also remove from Redis if exists
	urlRecord, _ := queries.GetURLByShortID(c.Context(), urlID)
	if urlRecord.ShortID != "" {
		redisClient := database.CreateClient(0)
		defer redisClient.Close()
		redisClient.Del(database.Ctx, urlRecord.ShortID)
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

