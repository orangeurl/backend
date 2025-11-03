package middleware

import (
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/xenonnn4w/orangeurl/internal/database"
	"golang.org/x/crypto/bcrypt"
)

const (
	APIKeyContextKey = "api_key"
	APIKeyUserKey    = "api_key_user"
)

// RequireAPIKey validates API key from Authorization header or X-API-Key header
func RequireAPIKey() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Extract API key from headers
		apiKey := extractAPIKey(c)
		if apiKey == "" {
			log.Printf("[API Auth] No API key provided")
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "API key required. Provide via Authorization: Bearer <key> or X-API-Key: <key>",
			})
		}

		// Validate API key format (should start with oran_live_ or oran_test_)
		if !strings.HasPrefix(apiKey, "oran_live_") && !strings.HasPrefix(apiKey, "oran_test_") {
			log.Printf("[API Auth] Invalid API key format")
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid API key format",
			})
		}

		// Extract prefix (first 20 characters: "oran_live_" or "oran_test_" + first 8 chars of key)
		prefix := apiKey
		if len(apiKey) > 20 {
			prefix = apiKey[:20]
		}

		// Lookup API key by prefix
		queries := database.GetQueries()
		apiKeyRecord, err := queries.GetAPIKeyByPrefix(c.Context(), prefix)
		if err != nil {
			log.Printf("[API Auth] API key not found: %v", err)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid API key",
			})
		}

		// Verify the full API key hash
		if err := bcrypt.CompareHashAndPassword([]byte(apiKeyRecord.ApiKeyHash), []byte(apiKey)); err != nil {
			log.Printf("[API Auth] API key hash mismatch")
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid API key",
			})
		}

		// Check if API key is active
		if !apiKeyRecord.IsActive.Bool {
			log.Printf("[API Auth] API key is inactive")
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "API key is inactive",
			})
		}

		// Check expiration
		if apiKeyRecord.ExpiresAt.Valid && apiKeyRecord.ExpiresAt.Time.Before(time.Now()) {
			log.Printf("[API Auth] API key expired")
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "API key expired",
			})
		}

		// Get user associated with API key
		user, err := queries.GetUserByID(c.Context(), apiKeyRecord.UserID)
		if err != nil {
			log.Printf("[API Auth] User not found for API key: %v", err)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "User not found",
			})
		}

		// Update last used timestamp (async, non-blocking)
		go func() {
			if err := queries.UpdateAPIKeyLastUsed(c.Context(), apiKeyRecord.ID); err != nil {
				log.Printf("[API Auth] Failed to update last_used_at: %v", err)
			}
		}()

		// Store API key and user in context
		c.Locals(APIKeyContextKey, apiKeyRecord)
		c.Locals("user", &user)

		log.Printf("[API Auth] API key validated for user: %s (tier: %s)", user.Email, apiKeyRecord.RateLimitTier)

		return c.Next()
	}
}

// extractAPIKey extracts API key from Authorization header or X-API-Key header
func extractAPIKey(c *fiber.Ctx) string {
	// Check Authorization header (Bearer token)
	authHeader := c.Get("Authorization")
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			return parts[1]
		}
	}

	// Check X-API-Key header
	apiKey := c.Get("X-API-Key")
	if apiKey != "" {
		return apiKey
	}

	return ""
}

// GetAPIKeyFromContext retrieves the API key from the Fiber context
func GetAPIKeyFromContext(c *fiber.Ctx) (database.ApiKey, error) {
	apiKey, ok := c.Locals(APIKeyContextKey).(database.ApiKey)
	if !ok {
		return database.ApiKey{}, fiber.NewError(fiber.StatusUnauthorized, "API key not found in context")
	}
	return apiKey, nil
}

// RequirePermission checks if the API key has a specific permission
func RequirePermission(permission string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiKey, err := GetAPIKeyFromContext(c)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "API key required",
			})
		}

		// Parse permissions from JSONB
		var permissions []string
		if apiKey.Permissions.Valid {
			// Try to parse the JSON array
			permStr := string(apiKey.Permissions.RawMessage)
			// Remove brackets and quotes, split by comma
			permStr = strings.Trim(permStr, "[]")
			permStr = strings.ReplaceAll(permStr, "\"", "")
			permStr = strings.ReplaceAll(permStr, " ", "")
			if permStr != "" {
				permissions = strings.Split(permStr, ",")
			}
		}

		// Check if the required permission exists
		hasPermission := false
		for _, perm := range permissions {
			if perm == permission {
				hasPermission = true
				break
			}
		}

		if !hasPermission {
			log.Printf("[API Auth] Permission denied: required '%s', API key has %v", permission, permissions)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Insufficient permissions. Required: " + permission,
			})
		}

		log.Printf("[API Auth] Permission granted: %s", permission)
		return c.Next()
	}
}
