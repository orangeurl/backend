package urls

import (
	"database/sql"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/xenonnn4w/orangeurl/internal/database"
	"github.com/xenonnn4w/orangeurl/internal/middleware"
)

// SetURLPassword sets or updates password protection for a URL (Premium only)
func SetURLPassword(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
	}

	// Check if user has Premium subscription
	queries := database.GetQueries()
	subscription, err := queries.GetUserSubscription(c.Context(), user.ID)
	if err != nil || subscription.PlanID != "premium" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Link locking is a Premium-only feature. Upgrade to Premium to use this feature.",
			"upgrade_required": true,
		})
	}

	urlID := c.Params("id")
	parsedID, err := uuid.Parse(urlID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid URL ID"})
	}

	type PasswordRequest struct {
		Password string `json:"password"`
	}

	body := new(PasswordRequest)
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

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

	// Hash the password
	hashedPassword, err := middleware.HashPassword(body.Password)
	if err != nil {
		log.Printf("[SetURLPassword] Failed to hash password: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to set password"})
	}

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
