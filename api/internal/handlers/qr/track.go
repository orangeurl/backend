package qr

import (
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/xenonnn4w/orangeurl/internal/database"
	"github.com/xenonnn4w/orangeurl/internal/middleware"
)

// TrackQRCodeGeneration increments the QR code counter for authenticated users
func TrackQRCodeGeneration(c *fiber.Ctx) error {
	// Get authenticated user
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	// Get database queries
	queries := database.GetQueries()
	if queries == nil {
		log.Printf("[TrackQRCode] Failed to get database queries")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	// Get current month in format YYYY-MM
	currentMonth := time.Now().Format("2006-01")

	// Increment QR code counter
	_, err = queries.IncrementQRCodeCount(c.Context(), database.IncrementQRCodeCountParams{
		UserID: user.ID,
		Month:  currentMonth,
	})

	if err != nil {
		log.Printf("[TrackQRCode] Failed to increment QR code counter: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to track QR code generation",
		})
	}

	log.Printf("[TrackQRCode] QR code counter incremented for user: %s", user.ID)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "QR code generation tracked",
	})
}
