package subscription

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/xenonnn4w/orangeurl/internal/database"
)

// DodoWebhookPayload represents the incoming webhook from DodoPayments
type DodoWebhookPayload struct {
	Type string `json:"type"`
	Data struct {
		Payload struct {
			SubscriptionID     string `json:"subscription_id"`
			CustomerID         string `json:"customer_id"`
			ProductID          string `json:"product_id"`
			PaymentID          string `json:"payment_id"`
			Status             string `json:"status"`
			CurrentPeriodStart string `json:"current_period_start"`
			CurrentPeriodEnd   string `json:"current_period_end"`
			Metadata           struct {
				UserID          string `json:"user_id"`
				Plan            string `json:"plan"`
				BillingInterval string `json:"billing_interval"`
				CustomerEmail   string `json:"customer_email"`
			} `json:"metadata"`
		} `json:"payload"`
		CreatedAt  string `json:"created_at"`
		BusinessID string `json:"business_id"`
	} `json:"data"`
}

// verifyDodoWebhookSignature verifies the webhook signature from DodoPayments
func verifyDodoWebhookSignature(body []byte, signature string) bool {
	secret := os.Getenv("DODO_WEBHOOK_SECRET")
	if secret == "" {
		log.Println("⚠️ DODO_WEBHOOK_SECRET not set, skipping verification")
		return true // Allow in development
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

// HandleDodoWebhook handles incoming webhooks from DodoPayments
func HandleDodoWebhook(c *fiber.Ctx) error {
	log.Println("=== DODO PAYMENTS WEBHOOK RECEIVED ===")

	body := c.Body()
	signature := c.Get("x-dodo-signature")
	if signature == "" {
		signature = c.Get("dodo-signature")
	}

	// Verify signature
	if !verifyDodoWebhookSignature(body, signature) {
		log.Println("❌ Invalid webhook signature")
		return c.Status(401).JSON(fiber.Map{"error": "Invalid signature"})
	}

	var event DodoWebhookPayload
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("❌ Failed to parse webhook: %v", err)
		return c.Status(400).JSON(fiber.Map{"error": "Invalid payload"})
	}

	log.Printf("📥 Webhook event type: %s", event.Type)
	log.Printf("📦 Payload: %+v", event.Data.Payload)

	payload := event.Data.Payload
	metadata := payload.Metadata
	userClerkID := metadata.UserID

	if userClerkID == "" {
		log.Println("⚠️ No user_id in webhook metadata")
		return c.JSON(fiber.Map{"received": true, "warning": "No user_id in metadata"})
	}

	queries := database.GetQueries()

	// Get user by clerk_id
	user, err := queries.GetUserByClerkID(c.Context(), userClerkID)
	if err != nil {
		log.Printf("❌ User not found for clerk_id %s: %v", userClerkID, err)
		return c.JSON(fiber.Map{"received": true, "warning": "User not found"})
	}

	switch event.Type {
	case "subscription.active", "subscription.renewed":
		log.Printf("✅ Processing %s for user %s", event.Type, user.ID)

		plan := metadata.Plan
		if plan == "" {
			plan = "pro"
		}
		billingInterval := metadata.BillingInterval
		if billingInterval == "" {
			billingInterval = "monthly"
		}

		// Parse period dates
		var periodStart, periodEnd sql.NullTime
		if payload.CurrentPeriodStart != "" {
			if t, err := time.Parse(time.RFC3339, payload.CurrentPeriodStart); err == nil {
				periodStart = sql.NullTime{Time: t, Valid: true}
			}
		}
		if payload.CurrentPeriodEnd != "" {
			if t, err := time.Parse(time.RFC3339, payload.CurrentPeriodEnd); err == nil {
				periodEnd = sql.NullTime{Time: t, Valid: true}
			}
		}

		// If no period end provided, calculate it
		if !periodEnd.Valid {
			now := time.Now()
			var end time.Time
			if billingInterval == "annual" {
				end = now.AddDate(1, 0, 0)
			} else {
				end = now.AddDate(0, 1, 0)
			}
			periodEnd = sql.NullTime{Time: end, Valid: true}
			periodStart = sql.NullTime{Time: now, Valid: true}
		}

		// Get or create subscription
		sub, err := queries.GetUserSubscription(c.Context(), user.ID)
		if err == sql.ErrNoRows {
			// Create new subscription
			_, err = queries.CreateSubscription(c.Context(), database.CreateSubscriptionParams{
				UserID:                 user.ID,
				PlanID:                 plan,
				Status:                 "active",
				DodopaymentsCustomerID: sql.NullString{String: payload.CustomerID, Valid: payload.CustomerID != ""},
				BillingInterval:        sql.NullString{String: billingInterval, Valid: true},
				CurrentPeriodStart:     periodStart,
				CurrentPeriodEnd:       periodEnd,
			})
			if err != nil {
				log.Printf("❌ Failed to create subscription: %v", err)
			}
		} else if err == nil {
			// Reset subscription period (this also resets URL usage)
			_, err = queries.ResetSubscriptionPeriod(c.Context(), database.ResetSubscriptionPeriodParams{
				ID:                 sub.ID,
				CurrentPeriodStart: periodStart,
				CurrentPeriodEnd:   periodEnd,
			})
			if err != nil {
				log.Printf("❌ Failed to reset subscription period: %v", err)
			}
		}

		// Update user's subscription tier
		err = queries.UpdateUserSubscriptionTier(c.Context(), database.UpdateUserSubscriptionTierParams{
			ID:               user.ID,
			SubscriptionTier: plan,
		})
		if err != nil {
			log.Printf("⚠️ Failed to update user tier: %v", err)
		}

		log.Printf("✅ Subscription %s processed for user %s", event.Type, user.ID)

	case "subscription.failed", "payment.failed":
		log.Printf("⚠️ Processing %s for user %s", event.Type, user.ID)

		// Get subscription and downgrade
		sub, err := queries.GetUserSubscription(c.Context(), user.ID)
		if err == nil {
			_, err = queries.DowngradeToFree(c.Context(), sub.ID)
			if err != nil {
				log.Printf("❌ Failed to downgrade subscription: %v", err)
			}
		}

		// Update user tier to free
		err = queries.UpdateUserSubscriptionTier(c.Context(), database.UpdateUserSubscriptionTierParams{
			ID:               user.ID,
			SubscriptionTier: "free",
		})
		if err != nil {
			log.Printf("⚠️ Failed to update user tier to free: %v", err)
		}

		log.Printf("✅ User %s downgraded to free due to %s", user.ID, event.Type)

	case "subscription.cancelled", "subscription.expired":
		log.Printf("📋 Processing %s for user %s", event.Type, user.ID)

		// Get subscription and update status
		sub, err := queries.GetUserSubscription(c.Context(), user.ID)
		if err == nil {
			status := "cancelled"
			if event.Type == "subscription.expired" {
				status = "expired"
			}
			err = queries.UpdateSubscriptionStatus(c.Context(), database.UpdateSubscriptionStatusParams{
				ID:     sub.ID,
				Status: status,
			})
			if err != nil {
				log.Printf("❌ Failed to update subscription status: %v", err)
			}
		}

		log.Printf("✅ Subscription %s processed for user %s", event.Type, user.ID)

	default:
		log.Printf("ℹ️ Unhandled webhook event type: %s", event.Type)
	}

	return c.JSON(fiber.Map{"received": true})
}
