package api_keys

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/xenonnn4w/orangeurl/internal/database"
	"github.com/xenonnn4w/orangeurl/internal/middleware"
)

// DeleteAPIKey revokes an API key
func DeleteAPIKey(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Parse API key ID from URL
	keyIDStr := c.Params("id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid API key ID"})
	}

	// Verify ownership before deleting
	queries := database.GetQueries()
	_, err = queries.GetAPIKeyByID(c.Context(), database.GetAPIKeyByIDParams{
		ID:     keyID,
		UserID: user.ID,
	})
	if err != nil {
		log.Printf("[DeleteAPIKey] API key not found or unauthorized: %v", err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "API key not found"})
	}

	// Delete the API key
	if err := queries.DeleteAPIKey(c.Context(), database.DeleteAPIKeyParams{
		ID:     keyID,
		UserID: user.ID,
	}); err != nil {
		log.Printf("[DeleteAPIKey] Failed to delete API key: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete API key"})
	}

	log.Printf("[DeleteAPIKey] API key deleted: %s (user: %s)", keyID, user.Email)

	return c.JSON(fiber.Map{
		"message": "API key revoked successfully",
	})
}

// UpdateAPIKey updates an API key's name or permissions
func UpdateAPIKey(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Parse API key ID from URL
	keyIDStr := c.Params("id")
	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid API key ID"})
	}

	var req struct {
		Name        *string  `json:"name"`
		Permissions []string `json:"permissions"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	queries := database.GetQueries()

	// Update name if provided
	if req.Name != nil && *req.Name != "" {
		_, err = queries.UpdateAPIKeyName(c.Context(), database.UpdateAPIKeyNameParams{
			ID:      keyID,
			KeyName: *req.Name,
			UserID:  user.ID,
		})
		if err != nil {
			log.Printf("[UpdateAPIKey] Failed to update API key name: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update API key"})
		}
	}

	// Update permissions if provided
	if len(req.Permissions) > 0 {
		// Convert permissions to JSONB format
		permissionsJSON := `["` + req.Permissions[0]
		for i := 1; i < len(req.Permissions); i++ {
			permissionsJSON += `", "` + req.Permissions[i]
		}
		permissionsJSON += `"]`

		_, err = queries.UpdateAPIKeyPermissions(c.Context(), database.UpdateAPIKeyPermissionsParams{
			ID:          keyID,
			Permissions: database.NewNullRawMessage([]byte(permissionsJSON)),
			UserID:      user.ID,
		})
		if err != nil {
			log.Printf("[UpdateAPIKey] Failed to update API key permissions: %v", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update API key"})
		}
	}

	log.Printf("[UpdateAPIKey] API key updated: %s (user: %s)", keyID, user.Email)

	return c.JSON(fiber.Map{
		"message": "API key updated successfully",
	})
}
