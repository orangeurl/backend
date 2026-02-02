package subscription

import (
	"database/sql"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/xenonnn4w/orangeurl/internal/database"
)

// UpdateSubscriptionRequest represents the request to update a subscription
type UpdateSubscriptionRequest struct {
	UserID             string  `json:"user_id"`
	Plan               string  `json:"plan"`
	Status             string  `json:"status"`
	CurrentPeriodStart *string `json:"current_period_start"`
	CurrentPeriodEnd   *string `json:"current_period_end"`
	BillingInterval    string  `json:"billing_interval"`
	SubscriptionID     string  `json:"subscription_id"`
}

// DowngradeRequest represents a downgrade request
type DowngradeRequest struct {
	UserID         string `json:"user_id"`
	SubscriptionID string `json:"subscription_id"`
}

// ResetUsageRequest represents a reset usage request
type ResetUsageRequest struct {
	UserID string `json:"user_id"`
}

// verifyInternalAPIKey checks if the request has a valid internal API key
func verifyInternalAPIKey(c *fiber.Ctx) bool {
	expectedKey := os.Getenv("INTERNAL_API_KEY")
	if expectedKey == "" {
		log.Println("⚠️ INTERNAL_API_KEY not set")
		return false
	}

	providedKey := c.Get("X-Internal-API-Key")
	return providedKey == expectedKey
}

// HandleUpdateSubscription handles subscription updates from webhooks
func HandleUpdateSubscription(c *fiber.Ctx) error {
	log.Println("📥 [Subscription] Update subscription request received")

	// Verify internal API key
	if !verifyInternalAPIKey(c) {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	var req UpdateSubscriptionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	log.Printf("📝 [Subscription] Updating subscription for user: %s, plan: %s, status: %s", req.UserID, req.Plan, req.Status)

	queries := database.GetQueries()

	// Get user by clerk_id
	user, err := queries.GetUserByClerkID(c.Context(), req.UserID)
	if err != nil {
		log.Printf("❌ [Subscription] User not found for clerk_id %s: %v", req.UserID, err)
		return c.Status(404).JSON(fiber.Map{"error": "User not found"})
	}

	// Parse dates
	var periodStart, periodEnd sql.NullTime
	if req.CurrentPeriodStart != nil {
		if t, err := time.Parse(time.RFC3339, *req.CurrentPeriodStart); err == nil {
			periodStart = sql.NullTime{Time: t, Valid: true}
		}
	}
	if req.CurrentPeriodEnd != nil {
		if t, err := time.Parse(time.RFC3339, *req.CurrentPeriodEnd); err == nil {
			periodEnd = sql.NullTime{Time: t, Valid: true}
		}
	}

	// Update or create subscription
	sub, err := queries.GetUserSubscription(c.Context(), user.ID)
	if err == sql.ErrNoRows {
		// Create new subscription
		log.Printf("📝 [Subscription] Creating new subscription for user %s", user.ID)
		_, err = queries.CreateSubscription(c.Context(), database.CreateSubscriptionParams{
			UserID:                 user.ID,
			PlanID:                 req.Plan,
			Status:                 req.Status,
			DodopaymentsCustomerID: sql.NullString{String: req.SubscriptionID, Valid: req.SubscriptionID != ""},
			BillingInterval:        sql.NullString{String: req.BillingInterval, Valid: req.BillingInterval != ""},
			CurrentPeriodStart:     periodStart,
			CurrentPeriodEnd:       periodEnd,
		})
	} else if err == nil {
		// Update existing subscription
		log.Printf("📝 [Subscription] Updating existing subscription %s", sub.ID)
		_, err = queries.UpdateSubscriptionWithBilling(c.Context(), database.UpdateSubscriptionWithBillingParams{
			UserID:                     user.ID,
			PlanID:                     req.Plan,
			Status:                     req.Status,
			CurrentPeriodStart:         periodStart,
			CurrentPeriodEnd:           periodEnd,
			BillingInterval:            sql.NullString{String: req.BillingInterval, Valid: req.BillingInterval != ""},
			DodopaymentsSubscriptionID: sql.NullString{String: req.SubscriptionID, Valid: req.SubscriptionID != ""},
		})
	}

	if err != nil {
		log.Printf("❌ [Subscription] Failed to update subscription: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update subscription"})
	}

	// Also update user's subscription tier
	err = queries.UpdateUserSubscriptionTier(c.Context(), database.UpdateUserSubscriptionTierParams{
		ID:               user.ID,
		SubscriptionTier: req.Plan,
	})
	if err != nil {
		log.Printf("⚠️ [Subscription] Failed to update user tier: %v", err)
	}

	log.Printf("✅ [Subscription] Subscription updated for user %s", user.ID)
	return c.JSON(fiber.Map{"success": true})
}

// HandleDowngradeSubscription handles downgrade requests from webhooks
func HandleDowngradeSubscription(c *fiber.Ctx) error {
	log.Println("📥 [Subscription] Downgrade request received")

	// Verify internal API key
	if !verifyInternalAPIKey(c) {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	var req DowngradeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	log.Printf("⬇️ [Subscription] Downgrading subscription for user: %s", req.UserID)

	queries := database.GetQueries()

	// Get user by clerk_id
	user, err := queries.GetUserByClerkID(c.Context(), req.UserID)
	if err != nil {
		log.Printf("❌ [Subscription] User not found for clerk_id %s: %v", req.UserID, err)
		return c.Status(404).JSON(fiber.Map{"error": "User not found"})
	}

	// Get subscription
	sub, err := queries.GetUserSubscription(c.Context(), user.ID)
	if err != nil {
		log.Printf("❌ [Subscription] Subscription not found for user %s: %v", user.ID, err)
		return c.Status(404).JSON(fiber.Map{"error": "Subscription not found"})
	}

	// Downgrade subscription
	service := NewSubscriptionRenewalService()
	if err := service.DowngradeToFree(c.Context(), sub.ID, user.ID); err != nil {
		log.Printf("❌ [Subscription] Failed to downgrade: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to downgrade subscription"})
	}

	log.Printf("✅ [Subscription] Subscription downgraded for user %s", user.ID)
	return c.JSON(fiber.Map{"success": true})
}

// HandleResetUsage handles URL usage reset requests from webhooks
func HandleResetUsage(c *fiber.Ctx) error {
	log.Println("📥 [Subscription] Reset usage request received")

	// Verify internal API key
	if !verifyInternalAPIKey(c) {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	var req ResetUsageRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	log.Printf("🔄 [Subscription] Resetting URL usage for user: %s", req.UserID)

	queries := database.GetQueries()

	// Get user by clerk_id
	user, err := queries.GetUserByClerkID(c.Context(), req.UserID)
	if err != nil {
		log.Printf("❌ [Subscription] User not found for clerk_id %s: %v", req.UserID, err)
		return c.Status(404).JSON(fiber.Map{"error": "User not found"})
	}

	// Reset URL usage
	service := NewSubscriptionRenewalService()
	if err := service.ResetURLUsageForUser(c.Context(), user.ID); err != nil {
		log.Printf("❌ [Subscription] Failed to reset usage: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to reset usage"})
	}

	log.Printf("✅ [Subscription] URL usage reset for user %s", user.ID)
	return c.JSON(fiber.Map{"success": true})
}

// HandleGetSubscriptionInfo returns subscription info for dashboard
func HandleGetSubscriptionInfo(c *fiber.Ctx) error {
	userID := c.Locals("user_id")
	if userID == nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	uid, ok := userID.(uuid.UUID)
	if !ok {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	service := NewSubscriptionRenewalService()
	info, err := service.GetSubscriptionInfo(c.Context(), uid)
	if err != nil {
		log.Printf("❌ [Subscription] Failed to get subscription info: %v", err)
		return c.Status(500).JSON(fiber.Map{"error": "Failed to get subscription info"})
	}

	return c.JSON(info)
}
