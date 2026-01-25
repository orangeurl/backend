package admin

import (
	"database/sql"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/xenonnn4w/orangeurl/internal/database"
	"github.com/xenonnn4w/orangeurl/internal/middleware"
)

// BlockURLRequest represents the request body for blocking a URL
type BlockURLRequest struct {
	ShortID        string `json:"short_id"`
	BlockReason    string `json:"block_reason"`
	AbuseReportRef string `json:"abuse_report_ref,omitempty"`
}

// BlockedURLResponse represents a blocked URL in responses
type BlockedURLResponse struct {
	ID             string `json:"id"`
	ShortID        string `json:"short_id"`
	OriginalURL    string `json:"original_url,omitempty"`
	BlockReason    string `json:"block_reason"`
	BlockedBy      string `json:"blocked_by,omitempty"`
	AbuseReportRef string `json:"abuse_report_ref,omitempty"`
	CreatedAt      string `json:"created_at"`
}

// BlockURL blocks a URL by short_id (for abuse reports)
// This removes the URL from Redis and adds it to the blocked_urls table
func BlockURL(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Verify admin status
	if !user.IsAdmin.Valid || !user.IsAdmin.Bool {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin access required"})
	}

	var req BlockURLRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.ShortID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "short_id is required"})
	}

	if req.BlockReason == "" {
		req.BlockReason = "Abuse report"
	}

	// Get the original URL from Redis before blocking (for record keeping)
	redisClient := database.CreateClient(0)
	defer redisClient.Close()

	originalURL, _ := redisClient.Get(database.Ctx, req.ShortID).Result()

	// 1. Remove from Redis immediately to stop redirects
	delResult := redisClient.Del(database.Ctx, req.ShortID)
	if delResult.Err() != nil {
		log.Printf("[BlockURL] Warning: Failed to delete from Redis: %v", delResult.Err())
	} else {
		log.Printf("[BlockURL] ✅ Deleted '%s' from Redis", req.ShortID)
	}

	// 2. Add to blocked_urls table in PostgreSQL
	queries := database.GetQueries()

	blockedURL, err := queries.BlockURL(c.Context(), database.BlockURLParams{
		ShortID:        req.ShortID,
		OriginalUrl:    sql.NullString{String: originalURL, Valid: originalURL != ""},
		BlockReason:    req.BlockReason,
		BlockedBy:      uuid.NullUUID{UUID: user.ID, Valid: true},
		AbuseReportRef: sql.NullString{String: req.AbuseReportRef, Valid: req.AbuseReportRef != ""},
	})

	if err != nil {
		log.Printf("[BlockURL] Error adding to blocked_urls: %v", err)
		// Still return success if Redis deletion worked - the URL is blocked
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"message":      "URL removed from Redis but failed to record in database",
			"short_id":     req.ShortID,
			"redis_status": "deleted",
			"db_status":    "failed",
			"warning":      err.Error(),
		})
	}

	// 3. Also deactivate in urls table if it exists
	_ = queries.DeactivateURLByShortID(c.Context(), req.ShortID)

	log.Printf("[BlockURL] ✅ Admin %s blocked URL '%s' - Reason: %s", user.Email, req.ShortID, req.BlockReason)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message":          "URL blocked successfully",
		"short_id":         req.ShortID,
		"original_url":     originalURL,
		"block_reason":     req.BlockReason,
		"abuse_report_ref": req.AbuseReportRef,
		"blocked_at":       blockedURL.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// UnblockURL removes a URL from the blocked list
func UnblockURL(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	if !user.IsAdmin.Valid || !user.IsAdmin.Bool {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin access required"})
	}

	shortID := c.Params("shortId")
	if shortID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "short_id is required"})
	}

	queries := database.GetQueries()
	err = queries.UnblockURL(c.Context(), shortID)
	if err != nil {
		log.Printf("[UnblockURL] Error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to unblock URL"})
	}

	log.Printf("[UnblockURL] ✅ Admin %s unblocked URL '%s'", user.Email, shortID)

	return c.JSON(fiber.Map{
		"message":  "URL unblocked successfully",
		"short_id": shortID,
	})
}

// ListBlockedURLs returns all blocked URLs
func ListBlockedURLs(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	if !user.IsAdmin.Valid || !user.IsAdmin.Bool {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin access required"})
	}

	queries := database.GetQueries()
	blockedURLs, err := queries.ListBlockedURLs(c.Context())
	if err != nil {
		log.Printf("[ListBlockedURLs] Error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch blocked URLs"})
	}

	response := make([]BlockedURLResponse, 0, len(blockedURLs))
	for _, url := range blockedURLs {
		item := BlockedURLResponse{
			ID:          url.ID.String(),
			ShortID:     url.ShortID,
			BlockReason: url.BlockReason,
			CreatedAt:   url.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		}

		if url.OriginalUrl.Valid {
			item.OriginalURL = url.OriginalUrl.String
		}
		if url.BlockedBy.Valid {
			item.BlockedBy = url.BlockedBy.UUID.String()
		}
		if url.AbuseReportRef.Valid {
			item.AbuseReportRef = url.AbuseReportRef.String
		}

		response = append(response, item)
	}

	count, _ := queries.GetBlockedURLCount(c.Context())

	return c.JSON(fiber.Map{
		"blocked_urls": response,
		"count":        count,
	})
}

// IsURLBlocked checks if a specific URL is blocked (for internal use)
func IsURLBlocked(c *fiber.Ctx) error {
	shortID := c.Params("shortId")
	if shortID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "short_id is required"})
	}

	queries := database.GetQueries()
	isBlocked, err := queries.IsURLBlocked(c.Context(), shortID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to check blocked status"})
	}

	return c.JSON(fiber.Map{
		"short_id":   shortID,
		"is_blocked": isBlocked,
	})
}
