package webhooks

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/xenonnn4w/orangeurl/internal/database"
	"github.com/xenonnn4w/orangeurl/internal/middleware"
)

type CreateWebhookRequest struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

// CreateWebhook creates a new webhook endpoint
func CreateWebhook(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	var req CreateWebhookRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Validate URL
	if req.URL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Webhook URL is required"})
	}

	// Define event access based on tier
	// Pro tier: Basic events only (url.created, url.clicked)
	// Premium tier: All events
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

	if len(req.Events) == 0 {
		req.Events = []string{"url.created", "url.clicked"}
	}

	// Check tier-based event restrictions
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
				"error":   "Event not available for Pro tier",
				"message": "The event '" + event + "' is only available for Premium subscribers. Pro tier supports: url.created, url.clicked",
				"upgrade_url": "https://orangeurl.live/pricing",
			})
		}
	}

	// Check webhook limit based on subscription tier
	queries := database.GetQueries()
	count, err := queries.CountUserActiveWebhooks(c.Context(), user.ID)
	if err != nil {
		log.Printf("[CreateWebhook] Failed to count webhooks: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create webhook"})
	}

	var maxWebhooks int64
	switch user.SubscriptionTier {
	case "free":
		maxWebhooks = 1
	case "pro":
		maxWebhooks = 3
	case "premium":
		maxWebhooks = 10
	default:
		maxWebhooks = 1
	}

	if count >= maxWebhooks {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Webhook limit reached for your subscription tier",
			"limit": maxWebhooks,
		})
	}

	// Generate webhook secret
	secret, err := generateWebhookSecret()
	if err != nil {
		log.Printf("[CreateWebhook] Failed to generate secret: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create webhook"})
	}

	// Create webhook
	webhook, err := queries.CreateWebhook(c.Context(), database.CreateWebhookParams{
		UserID:   user.ID,
		Url:      req.URL,
		Events:   req.Events,
		Secret:   secret,
		IsActive: sql.NullBool{Bool: true, Valid: true},
	})
	if err != nil {
		log.Printf("[CreateWebhook] Failed to create webhook: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create webhook"})
	}

	log.Printf("[CreateWebhook] Webhook created for user %s: %s", user.Email, webhook.ID)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":         webhook.ID,
		"url":        webhook.Url,
		"events":     webhook.Events,
		"secret":     secret, // Only returned once!
		"is_active":  webhook.IsActive,
		"created_at": webhook.CreatedAt,
	})
}

// generateWebhookSecret generates a cryptographically secure webhook secret
func generateWebhookSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "whsec_" + hex.EncodeToString(bytes), nil
}
