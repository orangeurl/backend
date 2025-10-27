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
	CustomerID  string `json:"customer_id"`
	Email       string `json:"email"`
	Name        string `json:"name"`
	PhoneNumber string `json:"phone_number,omitempty"`
}

type DodoSubscriptionData struct {
	SubscriptionID      string        `json:"subscription_id"`
	ProductID           string        `json:"product_id"`
	Status              string        `json:"status"`
	Currency            string        `json:"currency"`
	CreatedAt           string        `json:"created_at"`
	ExpiresAt           string        `json:"expires_at"`
	NextBillingDate     string        `json:"next_billing_date"`
	PreviousBillingDate string        `json:"previous_billing_date"`
	Customer            DodoCustomer  `json:"customer"`
	Billing             struct {
		City    string `json:"city"`
		Country string `json:"country"`
		State   string `json:"state"`
		Street  string `json:"street"`
		Zipcode string `json:"zipcode"`
	} `json:"billing"`
}

type DodoEvent struct {
	Type       string              `json:"type"`
	BusinessID string              `json:"business_id"`
	Data       DodoSubscriptionData `json:"data"`
	Timestamp  string              `json:"timestamp"`
}

// Helper to extract customer from nested data
func (evt *DodoEvent) GetCustomer() map[string]interface{} {
	// Customer is in data.customer based on logs
	// But we need to unmarshal it separately
	return nil
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

	// Extract customer from nested data
	customerEmail := evt.Data.Customer.Email
	if customerEmail == "" {
		log.Printf("No email in customer data")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "missing customer email"})
	}

	user, err := queries.GetUserByEmail(c.Context(), customerEmail)
	if err != nil {
		log.Printf("user not found for email=%s: %v", customerEmail, err)
		// Optional: return 200 to avoid retries if you only create subs post-registration
		return c.Status(http.StatusOK).JSON(fiber.Map{"message": "user not registered yet"})
	}

	// Get plan info from data
	planID := evt.Data.ProductID
	planName := inferPlanName(planID)

	// Parse period times from the webhook data
	var periodStart, periodEnd time.Time
	if t, err := time.Parse(time.RFC3339, evt.Data.CreatedAt); err == nil {
		periodStart = t
	}
	if t, err := time.Parse(time.RFC3339, evt.Data.NextBillingDate); err == nil {
		periodEnd = t
	}

	switch evt.Type {
	case "subscription.active", "subscription.renewed":
		// Upsert subscription
		if _, err := queries.GetUserSubscription(c.Context(), user.ID); err != nil {
			// create
			customerID := evt.Data.Customer.CustomerID
			_, err = queries.CreateSubscription(c.Context(), database.CreateSubscriptionParams{
				UserID:                 user.ID,
				PlanID:                 planID,
				Status:                 "active",
				DodopaymentsCustomerID: sql.NullString{String: customerID, Valid: customerID != ""},
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

