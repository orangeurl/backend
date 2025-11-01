package bio

import (
	"database/sql"
	"encoding/json"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/xenonnn4w/orangeurl/internal/database"
)

type UpdateBioPageRequest struct {
	DisplayName     *string          `json:"display_name"`
	Bio             *string          `json:"bio"`
	AvatarURL       *string          `json:"avatar_url"`
	FaviconURL      *string          `json:"favicon_url"`
	Theme           *string          `json:"theme"`
	CustomColors    *json.RawMessage `json:"custom_colors"`
	BackgroundType  *string          `json:"background_type"`
	BackgroundValue *string          `json:"background_value"`
	ButtonStyle     *string          `json:"button_style"`
	FontFamily      *string          `json:"font_family"`
	SocialLinks     *json.RawMessage `json:"social_links"`
	Links           *json.RawMessage `json:"links"`
	SEOTitle        *string          `json:"seo_title"`
	SEODescription  *string          `json:"seo_description"`
	OGImageURL      *string          `json:"og_image_url"`
	IsPublished     *bool            `json:"is_published"`
}

func UpdateBioPage(c *fiber.Ctx) error {
	username := c.Params("username")
	userID := c.Locals("user_id").(uuid.UUID).String()

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

	// Parse userID to UUID
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID",
		})
	}

	var req UpdateBioPageRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	queries := database.GetQueries()

	// Build update params with null handling
	params := database.UpdateBioPageParams{
		Username: username,
		UserID:   userUUID,
	}

	if req.DisplayName != nil {
		params.DisplayName = *req.DisplayName
	}
	if req.Bio != nil {
		params.Bio = sql.NullString{String: *req.Bio, Valid: true}
	}
	if req.AvatarURL != nil {
		params.AvatarUrl = sql.NullString{String: *req.AvatarURL, Valid: true}
	}
	if req.FaviconURL != nil {
		params.FaviconUrl = sql.NullString{String: *req.FaviconURL, Valid: true}
	}
	if req.Theme != nil {
		params.Theme = sql.NullString{String: *req.Theme, Valid: true}
	}
	if req.CustomColors != nil {
		params.CustomColors = pqtype.NullRawMessage{RawMessage: *req.CustomColors, Valid: true}
	}
	if req.BackgroundType != nil {
		params.BackgroundType = sql.NullString{String: *req.BackgroundType, Valid: true}
	}
	if req.BackgroundValue != nil {
		params.BackgroundValue = sql.NullString{String: *req.BackgroundValue, Valid: true}
	}
	if req.ButtonStyle != nil {
		params.ButtonStyle = sql.NullString{String: *req.ButtonStyle, Valid: true}
	}
	if req.FontFamily != nil {
		params.FontFamily = sql.NullString{String: *req.FontFamily, Valid: true}
	}
	if req.SocialLinks != nil {
		params.SocialLinks = pqtype.NullRawMessage{RawMessage: *req.SocialLinks, Valid: true}
	}
	if req.Links != nil {
		params.Links = pqtype.NullRawMessage{RawMessage: *req.Links, Valid: true}
	}
	if req.SEOTitle != nil {
		params.SeoTitle = sql.NullString{String: *req.SEOTitle, Valid: true}
	}
	if req.SEODescription != nil {
		params.SeoDescription = sql.NullString{String: *req.SEODescription, Valid: true}
	}
	if req.OGImageURL != nil {
		params.OgImageUrl = sql.NullString{String: *req.OGImageURL, Valid: true}
	}
	if req.IsPublished != nil {
		params.IsPublished = sql.NullBool{Bool: *req.IsPublished, Valid: true}
	}

	bioPage, err := queries.UpdateBioPage(c.Context(), params)
	if err != nil {
		log.Printf("Error updating bio page %s: %v", username, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update bio page",
		})
	}

	log.Printf("Bio page updated: %s by user %s", username, userID)

	return c.JSON(fiber.Map{
		"message":  "Bio page updated successfully",
		"bio_page": bioPage,
	})
}

func DeleteBioPage(c *fiber.Ctx) error {
	username := c.Params("username")
	userID := c.Locals("user_id").(uuid.UUID).String()

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

	// Parse userID to UUID
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID",
		})
	}

	queries := database.GetQueries()
	err = queries.DeleteBioPage(c.Context(), database.DeleteBioPageParams{
		Username: username,
		UserID:   userUUID,
	})

	if err != nil {
		log.Printf("Error deleting bio page %s: %v", username, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete bio page",
		})
	}

	log.Printf("Bio page deleted: %s by user %s", username, userID)

	return c.JSON(fiber.Map{
		"message": "Bio page deleted successfully",
	})
}

func CheckUsernameAvailability(c *fiber.Ctx) error {
	username := c.Params("username")
	if username == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Username is required",
		})
	}

	queries := database.GetQueries()
	exists, err := queries.CheckUsernameAvailability(c.Context(), username)
	if err != nil {
		log.Printf("Error checking username availability: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to check username availability",
		})
	}

	return c.JSON(fiber.Map{
		"username":  username,
		"available": !exists,
	})
}

func PublishBioPage(c *fiber.Ctx) error {
	username := c.Params("username")
	userID := c.Locals("user_id").(uuid.UUID).String()

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

	// Parse userID to UUID
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID",
		})
	}

	queries := database.GetQueries()
	bioPage, err := queries.PublishBioPage(c.Context(), database.PublishBioPageParams{
		Username: username,
		UserID:   userUUID,
	})

	if err != nil {
		log.Printf("Error publishing bio page %s: %v", username, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to publish bio page",
		})
	}

	log.Printf("Bio page published: %s by user %s", username, userID)

	return c.JSON(fiber.Map{
		"message":  "Bio page published successfully",
		"bio_page": bioPage,
	})
}

func UnpublishBioPage(c *fiber.Ctx) error {
	username := c.Params("username")
	userID := c.Locals("user_id").(uuid.UUID).String()

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

	// Parse userID to UUID
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID",
		})
	}

	queries := database.GetQueries()
	bioPage, err := queries.UnpublishBioPage(c.Context(), database.UnpublishBioPageParams{
		Username: username,
		UserID:   userUUID,
	})

	if err != nil {
		log.Printf("Error unpublishing bio page %s: %v", username, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to unpublish bio page",
		})
	}

	log.Printf("Bio page unpublished: %s by user %s", username, userID)

	return c.JSON(fiber.Map{
		"message":  "Bio page unpublished successfully",
		"bio_page": bioPage,
	})
}
