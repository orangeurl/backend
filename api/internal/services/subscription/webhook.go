package subscription

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/xenonnn4w/orangeurl/internal/database"
)

// DodoWebhookEvent represents the webhook payload from DodoPayments
type DodoWebhookEvent struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// DodoSubscriptionData represents subscription data in webhook
type DodoSubscriptionData struct {
	SubscriptionID     string `json:"subscription_id"`
	CustomerID         string `json:"customer_id"`
	ProductID          string `json:"product_id"`
	Status             string `json:"status"`
	BillingInterval    string `json:"billing_interval"` // "month" or "year"
	CurrentPeriodStart string `json:"current_period_start"`
	CurrentPeriodEnd   string `json:"current_period_end"`
	NextBillingDate    string `json:"next_billing_date"`
	PreviousBillingDate string `json:"previous_billing_date"`
	PaymentFrequencyInterval string `json:"payment_frequency_interval"`
	SubscriptionPeriodInterval string `json:"subscription_period_interval"`
	Metadata           struct {
		UserID string `json:"user_id"`
		Plan   string `json:"plan"`
		BillingInterval string `json:"billing_interval"`
	} `json:"metadata"`
}

// DodoPaymentData represents payment data in webhook
type DodoPaymentData struct {
	PaymentID      string `json:"payment_id"`
	SubscriptionID string `json:"subscription_id"`
	CustomerID     string `json:"customer_id"`
	Amount         int64  `json:"amount"`
	Currency       string `json:"currency"`
	Status         string `json:"status"`
	Metadata       struct {
		UserID string `json:"user_id"`
	} `json:"metadata"`
}

// HandleDodoWebhook handles incoming webhooks from DodoPayments
func HandleDodoWebhook(c *fiber.Ctx) error {
	// Verify webhook signature if secret is configured
	webhookSecret := os.Getenv("DODO_WEBHOOK_SECRET")
	if webhookSecret != "" {
		signature := c.Get("X-Dodo-Signature")
		if !verifyWebhookSignature(c.Body(), signature, webhookSecret) {
			log.Println("Webhook signature verification failed")
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid signature",
			})
		}
	}

	var event DodoWebhookEvent
	if err := json.Unmarshal(c.Body(), &event); err != nil {
		log.Printf("Failed to parse webhook event: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid payload",
		})
	}

	log.Printf("Received DodoPayments webhook: %s", event.Type)

	switch event.Type {
	case "subscription.active", "subscription.created":
		return handleSubscriptionActive(c, event.Data)
	case "subscription.renewed":
		return handleSubscriptionRenewed(c, event.Data)
	case "subscription.failed", "payment.failed":
		return handlePaymentFailed(c, event.Data)
	case "subscription.cancelled":
		return handleSubscriptionCancelled(c, event.Data)
	case "subscription.expired":
		return handleSubscriptionExpired(c, event.Data)
	default:
		log.Printf("Unhandled webhook event type: %s", event.Type)
		return c.JSON(fiber.Map{"received": true})
	}
}

func verifyWebhookSignature(payload []byte, signature, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expectedSig))
}

// parseUserID parses the user ID string to uuid.UUID
func parseUserID(ctx context.Context, userIDStr string) (uuid.UUID, error) {
	// Try UUID first
	if id, err := uuid.Parse(userIDStr); err == nil {
		return id, nil
	}

	// Fallback: treat as Clerk ID
	queries := database.GetQueries()
	user, err := queries.GetUserByClerkID(ctx, userIDStr)
	if err != nil {
		return uuid.Nil, err
	}
	return user.ID, nil
}

func normalizeInterval(interval string) string {
	val := strings.ToLower(strings.TrimSpace(interval))
	switch val {
	case "month", "monthly":
		return "month"
	case "year", "yearly", "annual", "annually":
		return "year"
	default:
		return val
	}
}

func parsePeriodDates(subData DodoSubscriptionData) (time.Time, time.Time) {
	// Prefer current period fields if present
	if subData.CurrentPeriodStart != "" || subData.CurrentPeriodEnd != "" {
		start, _ := time.Parse(time.RFC3339, subData.CurrentPeriodStart)
		end, _ := time.Parse(time.RFC3339, subData.CurrentPeriodEnd)
		return start, end
	}

	// Fallback to previous/next billing dates
	start, _ := time.Parse(time.RFC3339, subData.PreviousBillingDate)
	end, _ := time.Parse(time.RFC3339, subData.NextBillingDate)
	return start, end
}

func handleSubscriptionActive(c *fiber.Ctx, data json.RawMessage) error {
	var subData DodoSubscriptionData
	if err := json.Unmarshal(data, &subData); err != nil {
		log.Printf("Failed to parse subscription data: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid subscription data"})
	}

	userIDStr := subData.Metadata.UserID
	if userIDStr == "" {
		log.Println("No user_id in subscription metadata")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing user_id"})
	}

	userID, err := parseUserID(c.Context(), userIDStr)
	if err != nil {
		log.Printf("Invalid user_id format: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user_id format"})
	}

	// Parse dates
	periodStart, periodEnd := parsePeriodDates(subData)

	// Map product ID to plan (fallback to metadata plan)
	planID := mapProductToPlan(subData.ProductID)
	if planID == "" && subData.Metadata.Plan != "" {
		planID = strings.ToLower(subData.Metadata.Plan)
	}
	if planID == "" {
		planID = "pro"
	}

	billingInterval := normalizeInterval(subData.BillingInterval)
	if billingInterval == "" {
		billingInterval = normalizeInterval(subData.Metadata.BillingInterval)
	}
	if billingInterval == "" {
		billingInterval = normalizeInterval(subData.PaymentFrequencyInterval)
	}
	if billingInterval == "" {
		billingInterval = normalizeInterval(subData.SubscriptionPeriodInterval)
	}
	if billingInterval == "" {
		billingInterval = "month"
	}

	queries := database.GetQueries()

	// Try to update existing subscription first
	_, err = queries.UpdateSubscriptionWithBilling(c.Context(), database.UpdateSubscriptionWithBillingParams{
		UserID:             userID,
		PlanID:             planID,
		Status:             "active",
		BillingInterval:    toNullString(billingInterval),
		CurrentPeriodStart: toNullTime(periodStart),
		CurrentPeriodEnd:   toNullTime(periodEnd),
	})

	if err != nil {
		// Create new subscription if update failed
		_, err = queries.CreateSubscription(c.Context(), database.CreateSubscriptionParams{
			UserID:                 userID,
			PlanID:                 planID,
			Status:                 "active",
			DodopaymentsCustomerID: toNullString(subData.CustomerID),
		})
		if err != nil {
			log.Printf("Failed to create subscription for user %s: %v", userIDStr, err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create subscription"})
		}
	}

	// Update user's subscription tier
	err = queries.UpdateUserSubscriptionTier(c.Context(), database.UpdateUserSubscriptionTierParams{
		ID:               userID,
		SubscriptionTier: planID,
	})
	if err != nil {
		log.Printf("Failed to update user tier for %s: %v", userIDStr, err)
	}

	log.Printf("Subscription activated for user %s: plan=%s, interval=%s", userIDStr, planID, billingInterval)
	return c.JSON(fiber.Map{"received": true, "action": "subscription_activated"})
}

func handleSubscriptionRenewed(c *fiber.Ctx, data json.RawMessage) error {
	var subData DodoSubscriptionData
	if err := json.Unmarshal(data, &subData); err != nil {
		log.Printf("Failed to parse subscription data: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid subscription data"})
	}

	userIDStr := subData.Metadata.UserID
	if userIDStr == "" {
		log.Println("No user_id in subscription metadata")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing user_id"})
	}

	userID, err := parseUserID(c.Context(), userIDStr)
	if err != nil {
		log.Printf("Invalid user_id format: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user_id format"})
	}

	// Parse dates
	periodStart, periodEnd := parsePeriodDates(subData)

	queries := database.GetQueries()

	// Reset URL usage and update period
	_, err = queries.ResetSubscriptionPeriod(c.Context(), database.ResetSubscriptionPeriodParams{
		UserID:             userID,
		CurrentPeriodStart: toNullTime(periodStart),
		CurrentPeriodEnd:   toNullTime(periodEnd),
	})

	if err != nil {
		log.Printf("Failed to reset subscription period for user %s: %v", userIDStr, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to reset period"})
	}

	log.Printf("Subscription renewed for user %s: URLs reset, new period ends %s", userIDStr, periodEnd.Format("2006-01-02"))
	return c.JSON(fiber.Map{"received": true, "action": "subscription_renewed", "urls_reset": true})
}

func handlePaymentFailed(c *fiber.Ctx, data json.RawMessage) error {
	// Try to parse as subscription data first
	var subData DodoSubscriptionData
	if err := json.Unmarshal(data, &subData); err != nil {
		// Try payment data
		var payData DodoPaymentData
		if err := json.Unmarshal(data, &payData); err != nil {
			log.Printf("Failed to parse payment/subscription data: %v", err)
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid data"})
		}
		subData.Metadata.UserID = payData.Metadata.UserID
	}

	userIDStr := subData.Metadata.UserID
	if userIDStr == "" {
		log.Println("No user_id in metadata")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing user_id"})
	}

	userID, err := parseUserID(c.Context(), userIDStr)
	if err != nil {
		log.Printf("Invalid user_id format: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user_id format"})
	}

	queries := database.GetQueries()

	// Record the payment failure
	sub, err := queries.RecordPaymentFailure(c.Context(), userID)
	if err != nil {
		log.Printf("Failed to record payment failure for user %s: %v", userIDStr, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to record failure"})
	}

	// If 3 or more failures, downgrade to free
	if sub.FailedPaymentCount.Int32 >= 3 {
		_, err = queries.DowngradeToFree(c.Context(), userID)
		if err != nil {
			log.Printf("Failed to downgrade user %s: %v", userIDStr, err)
		}

		// Update user tier to free
		err = queries.UpdateUserSubscriptionTier(c.Context(), database.UpdateUserSubscriptionTierParams{
			ID:               userID,
			SubscriptionTier: "free",
		})
		if err != nil {
			log.Printf("Failed to update user tier for %s: %v", userIDStr, err)
		}

		log.Printf("User %s downgraded to free after %d payment failures", userIDStr, sub.FailedPaymentCount.Int32)
		return c.JSON(fiber.Map{"received": true, "action": "downgraded_to_free"})
	}

	log.Printf("Payment failure recorded for user %s (count: %d)", userIDStr, sub.FailedPaymentCount.Int32)
	return c.JSON(fiber.Map{"received": true, "action": "payment_failure_recorded"})
}

func handleSubscriptionCancelled(c *fiber.Ctx, data json.RawMessage) error {
	var subData DodoSubscriptionData
	if err := json.Unmarshal(data, &subData); err != nil {
		log.Printf("Failed to parse subscription data: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid subscription data"})
	}

	userIDStr := subData.Metadata.UserID
	if userIDStr == "" {
		log.Println("No user_id in subscription metadata")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing user_id"})
	}

	userID, err := parseUserID(c.Context(), userIDStr)
	if err != nil {
		log.Printf("Invalid user_id format: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user_id format"})
	}

	queries := database.GetQueries()

	// Update subscription status
	_, err = queries.UpdateSubscriptionStatus(c.Context(), database.UpdateSubscriptionStatusParams{
		UserID: userID,
		Status: "cancelled",
	})
	if err != nil {
		log.Printf("Failed to cancel subscription for user %s: %v", userIDStr, err)
	}

	// Downgrade to free
	err = queries.UpdateUserSubscriptionTier(c.Context(), database.UpdateUserSubscriptionTierParams{
		ID:               userID,
		SubscriptionTier: "free",
	})
	if err != nil {
		log.Printf("Failed to update user tier for %s: %v", userIDStr, err)
	}

	log.Printf("Subscription cancelled for user %s", userIDStr)
	return c.JSON(fiber.Map{"received": true, "action": "subscription_cancelled"})
}

func handleSubscriptionExpired(c *fiber.Ctx, data json.RawMessage) error {
	var subData DodoSubscriptionData
	if err := json.Unmarshal(data, &subData); err != nil {
		log.Printf("Failed to parse subscription data: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid subscription data"})
	}

	userIDStr := subData.Metadata.UserID
	if userIDStr == "" {
		log.Println("No user_id in subscription metadata")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing user_id"})
	}

	userID, err := parseUserID(c.Context(), userIDStr)
	if err != nil {
		log.Printf("Invalid user_id format: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid user_id format"})
	}

	queries := database.GetQueries()

	// Update subscription status
	_, err = queries.UpdateSubscriptionStatus(c.Context(), database.UpdateSubscriptionStatusParams{
		UserID: userID,
		Status: "expired",
	})
	if err != nil {
		log.Printf("Failed to expire subscription for user %s: %v", userIDStr, err)
	}

	// Downgrade to free
	err = queries.UpdateUserSubscriptionTier(c.Context(), database.UpdateUserSubscriptionTierParams{
		ID:               userID,
		SubscriptionTier: "free",
	})
	if err != nil {
		log.Printf("Failed to update user tier for %s: %v", userIDStr, err)
	}

	log.Printf("Subscription expired for user %s", userIDStr)
	return c.JSON(fiber.Map{"received": true, "action": "subscription_expired"})
}

// mapProductToPlan maps DodoPayments product IDs to plan names
func mapProductToPlan(productID string) string {
	productPlanMap := map[string]string{
		"pdt_RoSmAEKfLT":      "pro",
		"pdt_EAdypDoWVz":      "pro",
		"pdt_U14gV50OuglMMHUnQpFAe": "pro",
		"pdt_pro_monthly":     "pro",
		"pdt_pro_yearly":      "pro",
		"pdt_premium_monthly": "premium",
		"pdt_premium_yearly":  "premium",
	}

	if plan, ok := productPlanMap[productID]; ok {
		return plan
	}
	return ""
}
