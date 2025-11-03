package api_keys

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/xenonnn4w/orangeurl/internal/database"
	"github.com/xenonnn4w/orangeurl/internal/middleware"
	"golang.org/x/crypto/bcrypt"
)

type CreateAPIKeyRequest struct {
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
	ExpiresIn   *int     `json:"expires_in"` // Days until expiration (null = never expires)
}

type CreateAPIKeyResponse struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	APIKey      string     `json:"api_key"` // Only returned once!
	Prefix      string     `json:"prefix"`
	Permissions []string   `json:"permissions"`
	ExpiresAt   *time.Time `json:"expires_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// CreateAPIKey creates a new API key for the authenticated user
func CreateAPIKey(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	var req CreateAPIKeyRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Validate key name
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Key name is required"})
	}

	// Set default permissions if not provided
	if len(req.Permissions) == 0 {
		req.Permissions = []string{"urls:read", "urls:write", "analytics:read"}
	}

	// Validate permissions
	validPermissions := map[string]bool{
		"urls:read":      true,
		"urls:write":     true,
		"analytics:read": true,
		"webhooks:read":  true,
		"webhooks:write": true,
	}
	for _, perm := range req.Permissions {
		if !validPermissions[perm] {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid permission: " + perm,
			})
		}
	}

	// Check user's API key limit based on tier
	queries := database.GetQueries()
	count, err := queries.CountUserActiveAPIKeys(c.Context(), user.ID)
	if err != nil {
		log.Printf("[CreateAPIKey] Failed to count API keys: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create API key"})
	}

	// Set limits based on subscription tier
	var maxKeys int64
	switch user.SubscriptionTier {
	case "free":
		maxKeys = 1
	case "pro":
		maxKeys = 5
	case "premium":
		maxKeys = 20
	default:
		maxKeys = 1
	}

	if count >= maxKeys {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "API key limit reached for your subscription tier",
			"limit": maxKeys,
		})
	}

	// Generate API key
	apiKey, err := generateAPIKey("live")
	if err != nil {
		log.Printf("[CreateAPIKey] Failed to generate API key: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate API key"})
	}

	// Extract prefix (first 20 characters)
	prefix := apiKey[:20]

	// Hash the API key
	hashedKey, err := bcrypt.GenerateFromPassword([]byte(apiKey), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("[CreateAPIKey] Failed to hash API key: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create API key"})
	}

	// Calculate expiration
	var expiresAt sql.NullTime
	if req.ExpiresIn != nil {
		expiry := time.Now().AddDate(0, 0, *req.ExpiresIn)
		expiresAt = sql.NullTime{Time: expiry, Valid: true}
	}

	// Convert permissions to JSONB format
	permissionsJSON := `["` + req.Permissions[0]
	for i := 1; i < len(req.Permissions); i++ {
		permissionsJSON += `", "` + req.Permissions[i]
	}
	permissionsJSON += `"]`

	// Create API key in database
	apiKeyRecord, err := queries.CreateAPIKey(c.Context(), database.CreateAPIKeyParams{
		UserID:        user.ID,
		KeyName:       req.Name,
		ApiKeyHash:    string(hashedKey),
		ApiKeyPrefix:  prefix,
		Permissions:   database.NewNullRawMessage([]byte(permissionsJSON)),
		RateLimitTier: user.SubscriptionTier,
		ExpiresAt:     expiresAt,
	})
	if err != nil {
		log.Printf("[CreateAPIKey] Failed to save API key: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create API key"})
	}

	log.Printf("[CreateAPIKey] API key created for user %s: %s (prefix: %s)", user.Email, apiKeyRecord.ID, prefix)

	// Return response with the plaintext API key (only time it's visible!)
	var expiresAtPtr *time.Time
	if apiKeyRecord.ExpiresAt.Valid {
		expiresAtPtr = &apiKeyRecord.ExpiresAt.Time
	}

	return c.Status(fiber.StatusCreated).JSON(CreateAPIKeyResponse{
		ID:          apiKeyRecord.ID,
		Name:        apiKeyRecord.KeyName,
		APIKey:      apiKey,
		Prefix:      prefix,
		Permissions: req.Permissions,
		ExpiresAt:   expiresAtPtr,
		CreatedAt:   apiKeyRecord.CreatedAt.Time,
	})
}

// generateAPIKey generates a cryptographically secure API key
func generateAPIKey(env string) (string, error) {
	// Generate 32 random bytes
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}

	// Encode to base64 and make URL-safe
	encoded := base64.RawURLEncoding.EncodeToString(randomBytes)

	// Format: oran_live_<random> or oran_test_<random>
	return "oran_" + env + "_" + encoded, nil
}
