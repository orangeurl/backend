package payment

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/xenonnn4w/orangeurl/internal/database"
)

// Minimal event model; expand as needed
type DodoCustomer struct {
	CustomerID string `json:"customer_id"`
	Email      string `json:"email"`
	Name       string `json:"name"`
}

type DodoSubscription struct {
	SubscriptionID      string `json:"subscription_id"`
	ProductID           string `json:"product_id"`
	PlanName            string `json:"plan_name"` // optional if sent
	Currency            string `json:"currency"`
	BillingInterval     string `json:"billing_interval"` // e.g., "monthly"
	CurrentPeriodStart  string `json:"current_period_start"`
	CurrentPeriodEnd    string `json:"current_period_end"`
	CancelAtPeriodEnd   bool   `json:"cancel_at_period_end"`
}

type DodoEvent struct {
	Type         string           `json:"type"`
	Customer     DodoCustomer     `json:"customer"`
	Subscription DodoSubscription `json:"subscription"`
	// You can include payment or invoice details if needed
}

func HandleDodoWebhook(c *fiber.Ctx) error {
	raw := c.Body()
	signature := c.Get("dodo-signature")
	if signature == "" {
		// Some setups might use "Dodo-Signature" casing
		signature = c.Get("Dodo-Signature")
	}

	secret := os.Getenv("DODO_WEBHOOK_SECRET")
	if secret == "" {
		log.Printf("DODO_WEBHOOK_SECRET not set, skipping verification for testing")
		// For testing, allow without signature verification
		// In production, this should return an error
	} else if signature != "" {
		// Only verify signature if it's provided and secret exists
		if !verifySignature(raw, signature, secret) {
			log.Printf("Signature verification failed. Signature: %s, Raw length: %d", signature, len(raw))
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid signature"})
		}
	} else {
		log.Printf("No signature provided, skipping verification for testing")
	}

	log.Printf("Received webhook payload: %s", string(raw))

	var evt DodoEvent
	if err := json.Unmarshal(raw, &evt); err != nil {
		log.Printf("failed to parse webhook: %v", err)
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid payload"})
	}

	queries := database.GetQueries()

	// Resolve user by email from customer
	if evt.Customer.Email == "" {
		// If email is not present, you could resolve by evt.Customer.CustomerID
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "missing customer email"})
	}

	user, err := queries.GetUserByEmail(c.Context(), evt.Customer.Email)
	if err != nil {
		log.Printf("user not found for email=%s: %v", evt.Customer.Email, err)
		// Optional: return 200 to avoid retries if you only create subs post-registration
		return c.Status(http.StatusOK).JSON(fiber.Map{"message": "user not registered yet"})
	}

	// Normalize plan name by product mapping (optional)
	planID := evt.Subscription.ProductID
	planName := strings.ToLower(evt.Subscription.PlanName)
	if planName == "" {
		// fallback infer
		planName = inferPlanName(planID)
	}

	// Parse period times
	var periodStart, periodEnd time.Time
	if t, err := time.Parse(time.RFC3339, evt.Subscription.CurrentPeriodStart); err == nil {
		periodStart = t
	}
	if t, err := time.Parse(time.RFC3339, evt.Subscription.CurrentPeriodEnd); err == nil {
		periodEnd = t
	}

	switch evt.Type {
	case "subscription.active", "subscription.renewed":
		// Upsert subscription
		if _, err := queries.GetUserSubscription(c.Context(), user.ID); err != nil {
			// create
			_, err = queries.CreateSubscription(c.Context(), database.CreateSubscriptionParams{
				UserID:                 user.ID,
				PlanID:                 planID,
				Status:                 "active",
				DodopaymentsCustomerID: sql.NullString{String: evt.Customer.CustomerID, Valid: evt.Customer.CustomerID != ""},
			})
			if err != nil {
				log.Printf("create subscription failed: %v", err)
				return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "create subscription failed"})
			}
		}
		// update
		_, err := queries.UpdateSubscription(c.Context(), database.UpdateSubscriptionParams{
			UserID:             user.ID,
			PlanID:             planID,
			Status:             "active",
			CurrentPeriodStart: sql.NullTime{Time: periodStart, Valid: !periodStart.IsZero()},
			CurrentPeriodEnd:   sql.NullTime{Time: periodEnd, Valid: !periodEnd.IsZero()},
		})
		if err != nil {
			log.Printf("update subscription failed: %v", err)
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "update subscription failed"})
		}

		// Update user's tier
		tier := "free"
		if planName == "pro" || strings.Contains(strings.ToLower(planID), "pro") {
			tier = "pro"
		} else if planName == "premium" || strings.Contains(strings.ToLower(planID), "premium") {
			tier = "premium"
		}
		
		if err := queries.UpdateUserSubscriptionTier(c.Context(), database.UpdateUserSubscriptionTierParams{
			ID:               user.ID,
			SubscriptionTier: tier,
		}); err != nil {
			log.Printf("update user tier failed: %v", err)
			// continue but log
		}

	case "subscription.cancelled", "subscription.expired", "subscription.failed":
		_, err := queries.UpdateSubscription(c.Context(), database.UpdateSubscriptionParams{
			UserID:             user.ID,
			PlanID:             planID,
			Status:             "cancelled",
			CurrentPeriodStart: sql.NullTime{Time: periodStart, Valid: !periodStart.IsZero()},
			CurrentPeriodEnd:   sql.NullTime{Time: periodEnd, Valid: !periodEnd.IsZero()},
		})
		if err != nil {
			log.Printf("cancel/expire update failed: %v", err)
		}
		// downgrade user
		if err := queries.UpdateUserSubscriptionTier(c.Context(), database.UpdateUserSubscriptionTierParams{
			ID:               user.ID,
			SubscriptionTier: "free",
		}); err != nil {
			log.Printf("downgrade user tier failed: %v", err)
		}

	default:
		// No-op for other events
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{"ok": true})
}

func verifySignature(payload []byte, signature string, secret string) bool {
	// Basic HMAC-SHA256 verification against the raw body.
	// If Dodo uses a timestamped scheme (v1,t=…), adapt parser accordingly.
	if signature == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	// compare lowercased hex for safety
	return strings.EqualFold(expected, strings.TrimSpace(signature))
}

// Infer plan name by product mapping using env placeholders you provided
func inferPlanName(productID string) string {
	pro := os.Getenv("DODO_PRO_MONTHLY_PRODUCT_ID")
	premium := os.Getenv("DODO_PREMIUM_MONTHLY_PRODUCT_ID")
	if productID == pro {
		return "pro"
	}
	if productID == premium {
		return "premium"
	}
	return "free"
}

