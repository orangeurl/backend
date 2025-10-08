package waitlist

import (
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/xenonnn4w/orangeurl/internal/database"
)

type WaitlistRequest struct {
	Email string `json:"email"`
}

func JoinWaitlist(c *fiber.Ctx) error {
	var req WaitlistRequest
	if err := c.BodyParser(&req); err != nil {
		log.Printf("[WAITLIST] Failed to parse request body: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Validate email
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		log.Printf("[WAITLIST] Invalid email provided: %s", req.Email)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Valid email address is required"})
	}

	// Clean email
	email := strings.TrimSpace(strings.ToLower(req.Email))

	log.Printf("[WAITLIST] Adding email to waitlist: %s", email)

	queries := database.GetQueries()

	// Create waitlist entry
	entry, err := queries.CreateWaitlistEntry(c.Context(), email)
	if err != nil {
		// Check if it's a duplicate email error
		if strings.Contains(err.Error(), "duplicate key value") {
			log.Printf("[WAITLIST] Email already exists in waitlist: %s", email)
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Email already registered for waitlist"})
		}

		log.Printf("[WAITLIST] Failed to create waitlist entry: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to join waitlist"})
	}

	log.Printf("[WAITLIST] Successfully added to waitlist: %s (ID: %s)", email, entry.ID)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Successfully joined waitlist",
		"email":   entry.Email,
		"id":      entry.ID,
	})
}
