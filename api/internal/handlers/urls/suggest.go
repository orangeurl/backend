package urls

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/xenonnn4w/orangeurl/internal/database"
	"github.com/xenonnn4w/orangeurl/internal/middleware"
	"github.com/xenonnn4w/orangeurl/internal/services/ai"
)

type SuggestRequest struct {
	URL string `json:"url"`
}

type SuggestResponse struct {
	Suggestion string `json:"suggestion"`
}

// SuggestShortID generates an AI-powered short ID suggestion without creating the URL
func SuggestShortID(c *fiber.Ctx) error {
	body := new(SuggestRequest)

	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cannot parse json"})
	}

	if body.URL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "URL is required"})
	}

	// Check if user is authenticated and has Pro or Premium tier
	user, userErr := middleware.GetUserFromContext(c)
	userTier := "free"
	canUseAI := false

	if userErr == nil {
		queries := database.GetQueries()
		if queries != nil {
			userDetails, err := queries.GetUserByID(c.Context(), user.ID)
			if err == nil {
				userTier = userDetails.SubscriptionTier

				// Both Pro and Premium users get full Gemini AI
				if userTier == "pro" || userTier == "premium" {
					canUseAI = true
				}
			}
		}
	}

	if !canUseAI {
		// Free users: AI feature not available
		log.Printf("[SuggestShortID] AI suggestions denied - user tier: %s (requires Pro or Premium)", userTier)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error":           "AI-powered URL generation is a Pro/Premium feature",
			"upgrade_required": true,
			"current_tier":    userTier,
		})
	}

	// Pro & Premium: Use Gemini AI for intelligent generation
	log.Printf("[SuggestShortID] %s user - generating AI suggestion for: %s", userTier, body.URL)
	aiID, err := ai.GenerateSmartShortID(body.URL, true)
	if err != nil {
		log.Printf("[SuggestShortID] Gemini AI failed: %v, falling back to local smart generation", err)
		// Fallback to local smart generation
		smartID, fallbackErr := ai.GenerateSmartShortID(body.URL, false)
		if fallbackErr != nil {
			log.Printf("[SuggestShortID] Local smart generation also failed")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to generate suggestion",
			})
		}
		aiID = smartID
		log.Printf("[SuggestShortID] Fallback local smart generated ID: %s", aiID)
	} else {
		log.Printf("[SuggestShortID] Gemini AI generated suggestion: %s", aiID)
	}

	return c.Status(fiber.StatusOK).JSON(SuggestResponse{
		Suggestion: aiID,
	})
}
