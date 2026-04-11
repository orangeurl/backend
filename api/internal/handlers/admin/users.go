package admin

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/xenonnn4w/orangeurl/internal/database"
	"github.com/xenonnn4w/orangeurl/internal/middleware"
)

// AdminListUsers returns all users for admin management
func AdminListUsers(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}
	if !user.IsAdmin.Valid || !user.IsAdmin.Bool {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin access required"})
	}

	queries := database.GetQueries()
	users, err := queries.AdminListAllUsers(c.Context())
	if err != nil {
		log.Printf("[AdminListUsers] Error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch users"})
	}

	result := make([]fiber.Map, 0, len(users))
	for _, u := range users {
		item := fiber.Map{
			"id":                u.ID,
			"email":             u.Email,
			"first_name":        u.FirstName,
			"last_name":         u.LastName,
			"subscription_tier": u.SubscriptionTier,
			"is_admin":          u.IsAdmin.Valid && u.IsAdmin.Bool,
			"is_banned":         u.IsBanned.Valid && u.IsBanned.Bool,
			"created_at":        u.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		}
		if u.AvatarUrl.Valid {
			item["avatar_url"] = u.AvatarUrl.String
		}
		result = append(result, item)
	}

	return c.JSON(fiber.Map{
		"users": result,
		"count": len(result),
	})
}

// AdminBanUser bans a user
func AdminBanUser(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}
	if !user.IsAdmin.Valid || !user.IsAdmin.Bool {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin access required"})
	}

	userID := c.Params("id")
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "User ID required"})
	}

	parsedID, err := uuid.Parse(userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	// Don't allow banning yourself
	if parsedID == user.ID {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot ban yourself"})
	}

	queries := database.GetQueries()

	// Check if target is also an admin
	targetUser, err := queries.GetUserByID(c.Context(), parsedID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}
	if targetUser.IsAdmin.Valid && targetUser.IsAdmin.Bool {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot ban another admin"})
	}

	if err := queries.BanUser(c.Context(), parsedID); err != nil {
		log.Printf("[AdminBanUser] Error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to ban user"})
	}

	log.Printf("[AdminBanUser] Admin %s banned user %s (%s)", user.Email, targetUser.Email, userID)

	return c.JSON(fiber.Map{
		"message": "User banned successfully",
		"user_id": userID,
		"email":   targetUser.Email,
	})
}

// AdminUnbanUser unbans a user
func AdminUnbanUser(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}
	if !user.IsAdmin.Valid || !user.IsAdmin.Bool {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin access required"})
	}

	userID := c.Params("id")
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "User ID required"})
	}

	parsedID, err := uuid.Parse(userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	queries := database.GetQueries()
	if err := queries.UnbanUser(c.Context(), parsedID); err != nil {
		log.Printf("[AdminUnbanUser] Error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to unban user"})
	}

	log.Printf("[AdminUnbanUser] Admin %s unbanned user ID: %s", user.Email, userID)

	return c.JSON(fiber.Map{
		"message": "User unbanned successfully",
		"user_id": userID,
	})
}
