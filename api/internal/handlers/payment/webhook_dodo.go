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

	log.Printf("[DodoWebhook] ========== NEW WEBHOOK RECEIVED ==========")
	log.Printf("[DodoWebhook] Request from IP: %s", c.IP())
	// DO NOT log signature or full headers in production - sensitive data

	secret := os.Getenv("DODO_WEBHOOK_SECRET")
	if secret == "" {
		log.Printf("[DodoWebhook] ⚠️  DODO_WEBHOOK_SECRET not set, skipping verification for testing")
		// For testing, allow without signature verification
		// In production, this should return an error
	} else if signature != "" {
		// Only verify signature if it's provided and secret exists
		if !verifySignature(raw, signature, secret) {
			log.Printf("[DodoWebhook] ❌ Signature verification failed. Signature: %s, Raw length: %d", signature, len(raw))
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid signature"})
		}
		log.Printf("[DodoWebhook] ✅ Signature verified successfully")
	} else {
		log.Printf("[DodoWebhook] ⚠️  No signature provided, skipping verification for testing")
	}

	// DO NOT log raw payload in production - contains customer data

	var evt DodoEvent
	if err := json.Unmarshal(raw, &evt); err != nil {
		log.Printf("[DodoWebhook] ❌ Failed to parse webhook: %v", err)
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "invalid payload"})
	}

	log.Printf("[DodoWebhook] Event Type: %s", evt.Type)
	log.Printf("[DodoWebhook] Business ID: %s", evt.BusinessID)
	log.Printf("[DodoWebhook] Subscription ID: %s", evt.Data.SubscriptionID)
	log.Printf("[DodoWebhook] Product ID: %s", evt.Data.ProductID)
	log.Printf("[DodoWebhook] Status: %s", evt.Data.Status)

	queries := database.GetQueries()

	// Extract customer from nested data
	customerEmail := evt.Data.Customer.Email
	if customerEmail == "" {
		log.Printf("[DodoWebhook] ❌ No email in customer data")
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "missing customer email"})
	}

	log.Printf("[DodoWebhook] Customer Email: %s", customerEmail)
	log.Printf("[DodoWebhook] Customer ID: %s", evt.Data.Customer.CustomerID)

	user, err := queries.GetUserByEmail(c.Context(), customerEmail)
	if err != nil {
		log.Printf("[DodoWebhook] ⚠️  User not found for email=%s: %v", customerEmail, err)
		// Optional: return 200 to avoid retries if you only create subs post-registration
		return c.Status(http.StatusOK).JSON(fiber.Map{"message": "user not registered yet"})
	}

	log.Printf("[DodoWebhook] ✅ Found user: ID=%s, Email=%s", user.ID, user.Email)

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
		log.Printf("[DodoWebhook] Processing %s event...", evt.Type)

		// Determine tier from plan
		tier := "free"
		if planName == "pro" || strings.Contains(strings.ToLower(planID), "pro") {
			tier = "pro"
		} else if planName == "premium" || strings.Contains(strings.ToLower(planID), "premium") {
			tier = "premium"
		}
		log.Printf("[DodoWebhook] Determined tier: %s (from planID: %s, planName: %s)", tier, planID, planName)

		// Check if subscription exists
		existingSub, err := queries.GetUserSubscription(c.Context(), user.ID)
		customerID := evt.Data.Customer.CustomerID
		subscriptionID := evt.Data.SubscriptionID

		if err != nil {
			// Subscription doesn't exist, create it
			log.Printf("[DodoWebhook] Creating new subscription for user %s", user.ID)
			newSub, createErr := queries.CreateSubscription(c.Context(), database.CreateSubscriptionParams{
				UserID:                 user.ID,
				PlanID:                 planName, // Use planName instead of planID
				Status:                 "active",
				DodopaymentsCustomerID: sql.NullString{String: customerID, Valid: customerID != ""},
			})
			if createErr != nil {
				log.Printf("[DodoWebhook] ❌ Failed to create subscription: %v", createErr)
				return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "create subscription failed"})
			}
			log.Printf("[DodoWebhook] ✅ Created subscription: %s", newSub.ID)

			// Update with DodoPayments IDs
			_, updateErr := queries.UpdateSubscriptionSetDPIDs(c.Context(), database.UpdateSubscriptionSetDPIDsParams{
				UserID:                     user.ID,
				DodopaymentsSubscriptionID: sql.NullString{String: subscriptionID, Valid: subscriptionID != ""},
				DodopaymentsCustomerID:     sql.NullString{String: customerID, Valid: customerID != ""},
			})
			if updateErr != nil {
				log.Printf("[DodoWebhook] ⚠️  Failed to update DodoPayments IDs: %v", updateErr)
			} else {
				log.Printf("[DodoWebhook] ✅ Updated DodoPayments IDs (sub: %s, customer: %s)", subscriptionID, customerID)
			}
		} else {
			log.Printf("[DodoWebhook] Updating existing subscription %s", existingSub.ID)
		}

		// Update subscription details
		_, err = queries.UpdateSubscription(c.Context(), database.UpdateSubscriptionParams{
			UserID:             user.ID,
			PlanID:             planName, // Use planName instead of planID
			Status:             "active",
			CurrentPeriodStart: sql.NullTime{Time: periodStart, Valid: !periodStart.IsZero()},
			CurrentPeriodEnd:   sql.NullTime{Time: periodEnd, Valid: !periodEnd.IsZero()},
		})
		if err != nil {
			log.Printf("[DodoWebhook] ❌ Failed to update subscription: %v", err)
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "update subscription failed"})
		}
		log.Printf("[DodoWebhook] ✅ Updated subscription status to active")

		// Update user's tier
		if err := queries.UpdateUserSubscriptionTier(c.Context(), database.UpdateUserSubscriptionTierParams{
			ID:               user.ID,
			SubscriptionTier: tier,
		}); err != nil {
			log.Printf("[DodoWebhook] ❌ Failed to update user tier: %v", err)
			// continue but log
		} else {
			log.Printf("[DodoWebhook] ✅ Updated user tier to: %s", tier)
		}

	case "subscription.cancelled", "subscription.expired", "subscription.failed":
		log.Printf("[DodoWebhook] Processing %s event...", evt.Type)

		_, err := queries.UpdateSubscription(c.Context(), database.UpdateSubscriptionParams{
			UserID:             user.ID,
			PlanID:             planName,
			Status:             "cancelled",
			CurrentPeriodStart: sql.NullTime{Time: periodStart, Valid: !periodStart.IsZero()},
			CurrentPeriodEnd:   sql.NullTime{Time: periodEnd, Valid: !periodEnd.IsZero()},
		})
		if err != nil {
			log.Printf("[DodoWebhook] ❌ Failed to update subscription to cancelled: %v", err)
		} else {
			log.Printf("[DodoWebhook] ✅ Updated subscription status to cancelled")
		}

		// downgrade user
		if err := queries.UpdateUserSubscriptionTier(c.Context(), database.UpdateUserSubscriptionTierParams{
			ID:               user.ID,
			SubscriptionTier: "free",
		}); err != nil {
			log.Printf("[DodoWebhook] ❌ Failed to downgrade user tier: %v", err)
		} else {
			log.Printf("[DodoWebhook] ✅ Downgraded user to free tier")
		}

	default:
		log.Printf("[DodoWebhook] ⚠️  Unhandled event type: %s", evt.Type)
	}

	log.Printf("[DodoWebhook] ========== WEBHOOK PROCESSED SUCCESSFULLY ==========")
	return c.Status(http.StatusOK).JSON(fiber.Map{"ok": true, "message": "webhook processed"})
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
	proMonthly := os.Getenv("DODO_PRO_MONTHLY_PRODUCT_ID")
	premiumMonthly := os.Getenv("DODO_PREMIUM_MONTHLY_PRODUCT_ID")
	proAnnual := os.Getenv("DODO_PRO_ANNUAL_PRODUCT_ID")
	premiumAnnual := os.Getenv("DODO_PREMIUM_ANNUAL_PRODUCT_ID")
	
	// Check Pro plans (both monthly and annual)
	if productID == proMonthly || productID == proAnnual {
		return "pro"
	}
	// Check Premium plans (both monthly and annual)
	if productID == premiumMonthly || productID == premiumAnnual {
		return "premium"
	}
	return "free"
}

