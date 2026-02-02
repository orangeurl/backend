package subscription

import (
	"github.com/gofiber/fiber/v2"
	"github.com/xenonnn4w/orangeurl/internal/middleware"
)

// HandleGetSubscriptionInfo returns subscription info for the authenticated user
func HandleGetSubscriptionInfo(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	info, err := GetSubscriptionInfo(c.Context(), user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get subscription info",
		})
	}

	return c.JSON(info)
}

// HandleCheckURLLimit checks if user can create more URLs
func HandleCheckURLLimit(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	canCreate, remaining, err := CheckURLLimit(c.Context(), user.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to check limit",
		})
	}

	return c.JSON(fiber.Map{
		"can_create": canCreate,
		"remaining":  remaining,
	})
}
