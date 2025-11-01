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

type CreateBioPageRequest struct {
	Username         string          `json:"username"`
	DisplayName      string          `json:"display_name"`
	Bio              string          `json:"bio"`
	AvatarURL        string          `json:"avatar_url"`
	FaviconURL       string          `json:"favicon_url"`
	Theme            string          `json:"theme"`
	CustomColors     json.RawMessage `json:"custom_colors"`
	BackgroundType   string          `json:"background_type"`
	BackgroundValue  string          `json:"background_value"`
	ButtonStyle      string          `json:"button_style"`
	FontFamily       string          `json:"font_family"`
	SocialLinks      json.RawMessage `json:"social_links"`
	Links            json.RawMessage `json:"links"`
	SEOTitle         string          `json:"seo_title"`
	SEODescription   string          `json:"seo_description"`
	OGImageURL       string          `json:"og_image_url"`
	IsPublished      bool            `json:"is_published"`
}

func CreateBioPage(c *fiber.Ctx) error {
	// Get user ID from context (set by auth middleware)
	userUUID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}
	userID := userUUID.String()

	var req CreateBioPageRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate required fields
	if req.Username == "" || req.DisplayName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Username and display name are required",
		})
	}

	// Check if username is available
	queries := database.GetQueries()
	exists, err := queries.CheckUsernameAvailability(c.Context(), req.Username)
	if err != nil {
		log.Printf("Error checking username availability: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to check username availability",
		})
	}

	if exists {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "Username already taken",
		})
	}

	// Set defaults if not provided
	if req.Theme == "" {
		req.Theme = "light"
	}
	if req.BackgroundType == "" {
		req.BackgroundType = "gradient"
	}
	if req.ButtonStyle == "" {
		req.ButtonStyle = "rounded"
	}
	if req.FontFamily == "" {
		req.FontFamily = "Inter"
	}
	if req.CustomColors == nil {
		req.CustomColors = json.RawMessage(`{"primary":"#FF6B35","secondary":"#4ECDC4","background":"#FFFFFF","text":"#000000"}`)
	}
	if req.SocialLinks == nil {
		req.SocialLinks = json.RawMessage(`{}`)
	}
	if req.Links == nil {
		req.Links = json.RawMessage(`[]`)
	}

	// userUUID is already a UUID from Locals, no need to parse

	// Create bio page
	bioPage, err := queries.CreateBioPage(c.Context(), database.CreateBioPageParams{
		UserID:          userUUID,
		Username:        req.Username,
		DisplayName:     req.DisplayName,
		Bio:             sql.NullString{String: req.Bio, Valid: req.Bio != ""},
		AvatarUrl:       sql.NullString{String: req.AvatarURL, Valid: req.AvatarURL != ""},
		FaviconUrl:      sql.NullString{String: req.FaviconURL, Valid: req.FaviconURL != ""},
		Theme:           sql.NullString{String: req.Theme, Valid: req.Theme != ""},
		CustomColors:    pqtype.NullRawMessage{RawMessage: req.CustomColors, Valid: len(req.CustomColors) > 0},
		BackgroundType:  sql.NullString{String: req.BackgroundType, Valid: req.BackgroundType != ""},
		BackgroundValue: sql.NullString{String: req.BackgroundValue, Valid: req.BackgroundValue != ""},
		ButtonStyle:     sql.NullString{String: req.ButtonStyle, Valid: req.ButtonStyle != ""},
		FontFamily:      sql.NullString{String: req.FontFamily, Valid: req.FontFamily != ""},
		SocialLinks:     pqtype.NullRawMessage{RawMessage: req.SocialLinks, Valid: len(req.SocialLinks) > 0},
		Links:           pqtype.NullRawMessage{RawMessage: req.Links, Valid: len(req.Links) > 0},
		SeoTitle:        sql.NullString{String: req.SEOTitle, Valid: req.SEOTitle != ""},
		SeoDescription:  sql.NullString{String: req.SEODescription, Valid: req.SEODescription != ""},
		OgImageUrl:      sql.NullString{String: req.OGImageURL, Valid: req.OGImageURL != ""},
		IsPublished:     sql.NullBool{Bool: req.IsPublished, Valid: true},
	})

	if err != nil {
		log.Printf("Error creating bio page: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create bio page",
		})
	}

	log.Printf("Bio page created: %s for user %s", bioPage.Username, userID)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message":  "Bio page created successfully",
		"bio_page": bioPage,
	})
}
