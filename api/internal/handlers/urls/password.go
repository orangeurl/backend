package urls

import (
	"database/sql"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/xenonnn4w/orangeurl/internal/database"
	"github.com/xenonnn4w/orangeurl/internal/middleware"
)

// SetURLPassword sets or updates password protection for a URL (Premium only)
func SetURLPassword(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		log.Printf("[SetURLPassword] User not found in context: %v", err)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
	}

	log.Printf("[SetURLPassword] User ID: %s", user.ID)

	// Check if user has Premium subscription
	queries := database.GetQueries()

	// Check subscription tier - try subscriptions table first, fallback to user.subscription_tier
	planTier := user.SubscriptionTier
	subscription, err := queries.GetUserSubscription(c.Context(), user.ID)
	if err == nil && subscription.PlanID != "" {
		planTier = subscription.PlanID
		log.Printf("[SetURLPassword] Using subscription table plan: %s", planTier)
	} else {
		log.Printf("[SetURLPassword] No subscription found, using user.subscription_tier: %s", planTier)
	}

	if strings.ToLower(planTier) != "premium" {
		log.Printf("[SetURLPassword] User plan '%s' is not premium, denying access", planTier)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Link locking is a Premium-only feature. Upgrade to Premium to use this feature.",
			"upgrade_required": true,
		})
	}

	log.Printf("[SetURLPassword] User is Premium, proceeding...")

	urlID := c.Params("id")
	log.Printf("[SetURLPassword] URL ID from params: %s", urlID)

	parsedID, err := uuid.Parse(urlID)
	if err != nil {
		log.Printf("[SetURLPassword] Invalid UUID: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid URL ID"})
	}

	type PasswordRequest struct {
		Password string `json:"password"`
	}

	body := new(PasswordRequest)
	if err := c.BodyParser(&body); err != nil {
		log.Printf("[SetURLPassword] Failed to parse body: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	log.Printf("[SetURLPassword] Password length: %d", len(body.Password))

	// Validate password
	if len(body.Password) < 4 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Password must be at least 4 characters"})
	}

	if len(body.Password) > 72 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Password must be at most 72 characters"})
	}

	// Verify URL belongs to user
	urlRecord, err := queries.GetUserURLs(c.Context(), user.ID)
	if err != nil {
		log.Printf("[SetURLPassword] Failed to fetch URLs: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch URL"})
	}

	log.Printf("[SetURLPassword] Found %d URLs for user", len(urlRecord))

	found := false
	for _, url := range urlRecord {
		if url.ID == parsedID {
			found = true
			break
		}
	}

	if !found {
		log.Printf("[SetURLPassword] URL %s not found in user's URLs", parsedID)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "URL not found"})
	}

	log.Printf("[SetURLPassword] URL verified, hashing password...")

	// Hash the password
	hashedPassword, err := middleware.HashPassword(body.Password)
	if err != nil {
		log.Printf("[SetURLPassword] Failed to hash password: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to set password"})
	}

	log.Printf("[SetURLPassword] Password hashed successfully")

	// Update URL with password
	updatedURL, err := queries.SetURLPassword(c.Context(), database.SetURLPasswordParams{
		ID:           parsedID,
		PasswordHash: sql.NullString{String: hashedPassword, Valid: true},
		IsLocked:     sql.NullBool{Bool: true, Valid: true},
	})

	if err != nil {
		log.Printf("[SetURLPassword] Failed to update URL: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to set password"})
	}

	log.Printf("[SetURLPassword] Successfully locked URL %s", updatedURL.ShortID)

	return c.JSON(fiber.Map{
		"message": "Password protection enabled",
		"url": fiber.Map{
			"id":        updatedURL.ID,
			"short_id":  updatedURL.ShortID,
			"is_locked": updatedURL.IsLocked.Bool,
		},
	})
}

// RemoveURLPassword removes password protection from a URL
func RemoveURLPassword(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
	}

	urlID := c.Params("id")
	parsedID, err := uuid.Parse(urlID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid URL ID"})
	}

	queries := database.GetQueries()

	// Verify URL belongs to user
	urlRecord, err := queries.GetUserURLs(c.Context(), user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch URL"})
	}

	found := false
	for _, url := range urlRecord {
		if url.ID == parsedID {
			found = true
			break
		}
	}

	if !found {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "URL not found"})
	}

	// Remove password
	updatedURL, err := queries.RemoveURLPassword(c.Context(), parsedID)
	if err != nil {
		log.Printf("[RemoveURLPassword] Failed to update URL: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to remove password"})
	}

	return c.JSON(fiber.Map{
		"message": "Password protection removed",
		"url": fiber.Map{
			"id":        updatedURL.ID,
			"short_id":  updatedURL.ShortID,
			"is_locked": false,
		},
	})
}

// UnlockURL verifies password and creates an unlock session
func UnlockURL(c *fiber.Ctx) error {
	shortID := c.Params("shortId")
	ip := c.IP()

	type UnlockRequest struct {
		Password string `json:"password"`
	}

	body := new(UnlockRequest)
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Check rate limit
	allowed, attempts, err := middleware.CheckRateLimit(shortID, ip)
	if err != nil {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"error":    err.Error(),
			"attempts": attempts,
		})
	}

	if !allowed {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"error":    "Too many failed attempts. Please try again later.",
			"attempts": attempts,
		})
	}

	// Get URL from database
	queries := database.GetQueries()
	urlRecord, err := queries.GetURLByShortID(c.Context(), shortID)
	if err != nil {
		// Increment rate limit even for wrong URL to prevent enumeration
		middleware.IncrementRateLimit(shortID, ip)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "URL not found"})
	}

	// Check if URL is locked
	if !urlRecord.IsLocked.Valid || !urlRecord.IsLocked.Bool {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "URL is not password protected"})
	}

	// Verify password
	if !middleware.VerifyPassword(body.Password, urlRecord.PasswordHash.String) {
		// Increment failed attempts
		middleware.IncrementRateLimit(shortID, ip)
		queries.IncrementPasswordAttempts(c.Context(), shortID)

		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":    "Incorrect password",
			"attempts": attempts + 1,
		})
	}

	// Password correct - create unlock session
	err = middleware.SetURLUnlocked(shortID, ip)
	if err != nil {
		log.Printf("[UnlockURL] Failed to set unlock session: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to unlock URL"})
	}

	// Reset password attempts on successful unlock
	queries.ResetPasswordAttempts(c.Context(), shortID)

	return c.JSON(fiber.Map{
		"message":      "URL unlocked successfully",
		"original_url": urlRecord.OriginalUrl,
		"redirect_url": urlRecord.OriginalUrl,
	})
}

// GetPasswordStats returns password attempt statistics for a URL
func GetPasswordStats(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
	}

	shortID := c.Params("shortId")

	queries := database.GetQueries()
	urlRecord, err := queries.GetURLByShortID(c.Context(), shortID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "URL not found"})
	}

	// Verify URL belongs to user
	if urlRecord.UserID != user.ID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Access denied"})
	}

	return c.JSON(fiber.Map{
		"short_id":             urlRecord.ShortID,
		"is_locked":            urlRecord.IsLocked.Bool,
		"password_attempts":    urlRecord.PasswordAttempts.Int32,
		"last_password_attempt": urlRecord.LastPasswordAttempt,
	})
}
