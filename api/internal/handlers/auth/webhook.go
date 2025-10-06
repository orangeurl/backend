package auth

import (
	"database/sql"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/xenonnn4w/orangeurl/internal/database"
)

type ClerkEmailAddress struct {
	EmailAddress string `json:"email_address"`
	Primary      bool   `json:"primary"`
}

// *string means we can have a null value
type ClerkUser struct {
	ID        string              `json:"id"`
	Email     []ClerkEmailAddress `json:"email"`
	FirstName *string             `json:"first_name"`
	LastName  *string             `json:"last_name"`
	AvatarURL *string             `json:"avatar_url"`
}

type ClerkWebhookEvent struct {
	Type string    `json:"type"`
	Data ClerkUser `json:"data"`
}

func HandleClerkWebhook(c *fiber.Ctx) error {
	log.Printf("Webhook called with body: %s", string(c.Body()))
	if !verifyWebhookSignature(c) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Parse the body into the ClerkWebhookEvent struct
	var event ClerkWebhookEvent
	if err := c.BodyParser(&event); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{})
	}

	// Handle different event types
	switch event.Type {
	case "user.created":
		return handleUserCreated(c, event.Data)
	case "user.updated":
		return handleUserUpdated(c, event.Data)
	default:
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Event received"})
	}

}

func handleUserCreated(c *fiber.Ctx, user ClerkUser) error {
	email := getPrimaryEmail(user.Email)
	if email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Primary email not found"})
	}

	queries := database.GetQueries()
	_, err := queries.CreateUser(c.Context(), database.CreateUserParams{
		ClerkID:   user.ID,
		Email:     email,
		FirstName: *user.FirstName,
		LastName:  *user.LastName,
		AvatarUrl: sql.NullString{String: *user.AvatarURL, Valid: true},
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create user"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "User created successfully"})
}

func handleUserUpdated(c *fiber.Ctx, user ClerkUser) error {
	email := getPrimaryEmail(user.Email)
	if email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Primary email not found"})
	}

	queries := database.GetQueries()
	_, err := queries.UpdateUser(c.Context(), database.UpdateUserParams{
		ClerkID:   user.ID,
		Email:     email,
		FirstName: *user.FirstName,
		LastName:  *user.LastName,
		AvatarUrl: sql.NullString{String: *user.AvatarURL, Valid: true},
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update user"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "User updated successfully"})
}

func getPrimaryEmail(emails []ClerkEmailAddress) string {
	for _, email := range emails {
		if email.Primary {
			return email.EmailAddress
		}
	}
	return ""
}

func verifyWebhookSignature(c *fiber.Ctx) bool {
	//TODO: Implement Clerks Webhook Signature Verification
	// For now, return true for development
	webhookSecret := os.Getenv("CLERK_WEBHOOK_SECRET")
	return webhookSecret != ""
}
