package webhooks

import (
	"database/sql"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/xenonnn4w/orangeurl/internal/database"
	"github.com/xenonnn4w/orangeurl/internal/middleware"
)

// ListWebhooks returns all webhooks for the authenticated user
func ListWebhooks(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	queries := database.GetQueries()
	webhooks, err := queries.ListUserWebhooks(c.Context(), user.ID)
	if err != nil {
		log.Printf("[ListWebhooks] Failed to fetch webhooks: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch webhooks"})
	}

	// Transform to response format (hide secrets)
	response := make([]fiber.Map, len(webhooks))
	for i, webhook := range webhooks {
		response[i] = fiber.Map{
			"id":         webhook.ID,
			"url":        webhook.Url,
			"events":     webhook.Events,
			"is_active":  webhook.IsActive,
			"created_at": webhook.CreatedAt,
			"updated_at": webhook.UpdatedAt,
		}
	}

	return c.JSON(fiber.Map{
		"webhooks": response,
		"count":    len(response),
	})
}

// DeleteWebhook deletes a webhook
func DeleteWebhook(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	webhookIDStr := c.Params("id")
	webhookID, err := uuid.Parse(webhookIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid webhook ID"})
	}

	queries := database.GetQueries()

	// Verify ownership
	_, err = queries.GetWebhookByID(c.Context(), database.GetWebhookByIDParams{
		ID:     webhookID,
		UserID: user.ID,
	})
	if err != nil {
		log.Printf("[DeleteWebhook] Webhook not found: %v", err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Webhook not found"})
	}

	// Delete webhook
	if err := queries.DeleteWebhook(c.Context(), database.DeleteWebhookParams{
		ID:     webhookID,
		UserID: user.ID,
	}); err != nil {
		log.Printf("[DeleteWebhook] Failed to delete webhook: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete webhook"})
	}

	log.Printf("[DeleteWebhook] Webhook deleted: %s (user: %s)", webhookID, user.Email)

	return c.JSON(fiber.Map{
		"message": "Webhook deleted successfully",
	})
}

// UpdateWebhook updates a webhook's URL or events
func UpdateWebhook(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	webhookIDStr := c.Params("id")
	webhookID, err := uuid.Parse(webhookIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid webhook ID"})
	}

	var req struct {
		URL    string   `json:"url"`
		Events []string `json:"events"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.URL == "" || len(req.Events) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "URL and events are required"})
	}

	// Validate URL format and prevent SSRF attacks
	if err := ValidateWebhookURL(req.URL); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid webhook URL: " + err.Error(),
		})
	}

	// Define event access based on tier (same as CreateWebhook)
	basicEvents := map[string]bool{
		"url.created": true,
		"url.clicked": true,
	}

	allEvents := map[string]bool{
		"url.created": true,
		"url.clicked": true,
		"url.deleted": true,
		"url.expired": true,
	}

	// Validate events
	for _, event := range req.Events {
		// First check if event is valid at all
		if !allEvents[event] {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid event type: " + event,
			})
		}

		// Pro tier can only use basic events
		if user.SubscriptionTier == "pro" && !basicEvents[event] {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error":       "Event not available for Pro tier",
				"message":     "The event '" + event + "' is only available for Premium subscribers. Pro tier supports: url.created, url.clicked",
				"upgrade_url": "https://orangeurl.live/pricing",
			})
		}

		// Free tier can only use basic events
		if user.SubscriptionTier != "premium" && !basicEvents[event] {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error":       "Event requires Premium subscription",
				"message":     "The event '" + event + "' is only available for Premium subscribers",
				"upgrade_url": "https://orangeurl.live/pricing",
			})
		}
	}

	queries := database.GetQueries()

	// Update webhook
	webhook, err := queries.UpdateWebhook(c.Context(), database.UpdateWebhookParams{
		ID:     webhookID,
		Url:    req.URL,
		Events: req.Events,
		UserID: user.ID,
	})
	if err != nil {
		log.Printf("[UpdateWebhook] Failed to update webhook: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update webhook"})
	}

	log.Printf("[UpdateWebhook] Webhook updated: %s (user: %s)", webhookID, user.Email)

	return c.JSON(fiber.Map{
		"id":         webhook.ID,
		"url":        webhook.Url,
		"events":     webhook.Events,
		"is_active":  webhook.IsActive,
		"updated_at": webhook.UpdatedAt,
	})
}

// ToggleWebhook enables or disables a webhook
func ToggleWebhook(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	webhookIDStr := c.Params("id")
	webhookID, err := uuid.Parse(webhookIDStr)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid webhook ID"})
	}

	var req struct {
		IsActive bool `json:"is_active"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	queries := database.GetQueries()

	webhook, err := queries.ToggleWebhookStatus(c.Context(), database.ToggleWebhookStatusParams{
		ID:       webhookID,
		IsActive: sql.NullBool{Bool: req.IsActive, Valid: true},
		UserID:   user.ID,
	})
	if err != nil {
		log.Printf("[ToggleWebhook] Failed to toggle webhook: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to toggle webhook"})
	}

	log.Printf("[ToggleWebhook] Webhook toggled: %s (active: %v, user: %s)", webhookID, req.IsActive, user.Email)

	return c.JSON(fiber.Map{
		"id":        webhook.ID,
		"is_active": webhook.IsActive,
	})
}
