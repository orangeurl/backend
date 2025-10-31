package bio

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/xenonnn4w/orangeurl/internal/database"
)

// GetBioPage - Public endpoint to get a published bio page
func GetBioPage(c *fiber.Ctx) error {
	username := c.Params("username")
	if username == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Username is required",
		})
	}

	queries := database.GetQueries()
	bioPage, err := queries.GetBioPageByUsername(c.Context(), username)
	if err != nil {
		log.Printf("Bio page not found: %s, error: %v", username, err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Bio page not found",
		})
	}

	return c.JSON(fiber.Map{
		"bio_page": bioPage,
	})
}

// GetBioPageForEdit - Get bio page for editing (requires auth)
func GetBioPageForEdit(c *fiber.Ctx) error {
	username := c.Params("username")
	userID := c.Locals("userId").(string)

	if username == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Username is required",
		})
	}

	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	queries := database.GetQueries()
	bioPage, err := queries.GetBioPageByUsernameForEdit(c.Context(), database.GetBioPageByUsernameForEditParams{
		Username: username,
		UserID:   userID,
	})

	if err != nil {
		log.Printf("Bio page not found for editing: %s, user: %s, error: %v", username, userID, err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Bio page not found or you don't have permission to edit it",
		})
	}

	return c.JSON(fiber.Map{
		"bio_page": bioPage,
	})
}

// GetUserBioPages - Get all bio pages for the authenticated user
func GetUserBioPages(c *fiber.Ctx) error {
	userID := c.Locals("userId").(string)

	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	queries := database.GetQueries()
	bioPages, err := queries.GetUserBioPages(c.Context(), userID)
	if err != nil {
		log.Printf("Error getting bio pages for user %s: %v", userID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get bio pages",
		})
	}

	return c.JSON(fiber.Map{
		"bio_pages": bioPages,
		"count":     len(bioPages),
	})
}
