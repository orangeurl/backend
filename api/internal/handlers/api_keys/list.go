package api_keys

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/xenonnn4w/orangeurl/internal/database"
	"github.com/xenonnn4w/orangeurl/internal/middleware"
)

type APIKeyListItem struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Prefix      string     `json:"prefix"`
	Permissions []string   `json:"permissions"`
	IsActive    bool       `json:"is_active"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	ExpiresAt   *time.Time `json:"expires_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// ListAPIKeys returns all API keys for the authenticated user
func ListAPIKeys(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	queries := database.GetQueries()
	apiKeys, err := queries.ListUserAPIKeys(c.Context(), user.ID)
	if err != nil {
		log.Printf("[ListAPIKeys] Failed to fetch API keys: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch API keys"})
	}

	// Transform to response format
	response := make([]APIKeyListItem, len(apiKeys))
	for i, key := range apiKeys {
		var permissions []string
		if key.Permissions.Valid {
			if err := json.Unmarshal(key.Permissions.RawMessage, &permissions); err != nil {
				log.Printf("[ListAPIKeys] Failed to parse permissions: %v", err)
				permissions = []string{}
			}
		}

		var lastUsedAt *time.Time
		if key.LastUsedAt.Valid {
			lastUsedAt = &key.LastUsedAt.Time
		}

		var expiresAt *time.Time
		if key.ExpiresAt.Valid {
			expiresAt = &key.ExpiresAt.Time
		}

		response[i] = APIKeyListItem{
			ID:          key.ID,
			Name:        key.KeyName,
			Prefix:      key.ApiKeyPrefix,
			Permissions: permissions,
			IsActive:    key.IsActive.Bool,
			LastUsedAt:  lastUsedAt,
			ExpiresAt:   expiresAt,
			CreatedAt:   key.CreatedAt.Time,
		}
	}

	return c.JSON(fiber.Map{
		"api_keys": response,
		"count":    len(response),
	})
}
