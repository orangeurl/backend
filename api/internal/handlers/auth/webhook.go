package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/xenonnn4w/orangeurl/internal/database"
)

type ClerkEmailAddress struct {
	EmailAddress string `json:"email_address"`
	ID           string `json:"id"`
	Verification struct {
		Status string `json:"status"`
	} `json:"verification"`
}

type ClerkUser struct {
	ID                    string              `json:"id"`
	EmailAddresses        []ClerkEmailAddress `json:"email_addresses"`
	FirstName             *string             `json:"first_name"`
	LastName              *string             `json:"last_name"`
	ImageURL              *string             `json:"image_url"`
	ProfileImageURL       *string             `json:"profile_image_url"`
	PrimaryEmailAddressID *string             `json:"primary_email_address_id"`
	CreatedAt             int64               `json:"created_at"`
	UpdatedAt             int64               `json:"updated_at"`
}

type ClerkWebhookEvent struct {
	Type   string    `json:"type"`
	Data   ClerkUser `json:"data"`
	Object string    `json:"object"`
}

func HandleClerkWebhook(c *fiber.Ctx) error {
	log.Printf("=== WEBHOOK CALLED ===")
	// DO NOT log full headers or body in production - may contain sensitive data
	log.Printf("Request received from IP: %s", c.IP())
	log.Printf("=======================")

	// Parse the webhook event first
	var event ClerkWebhookEvent
	if err := c.BodyParser(&event); err != nil {
		log.Printf("Failed to parse webhook body: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid JSON"})
	}

	// Smart detection: Check if user already exists to determine intent
	email := getPrimaryEmail(event.Data.EmailAddresses, event.Data.PrimaryEmailAddressID)
	if email == "" {
		log.Printf("❌ ERROR: No email found in webhook - this should not happen with Google OAuth")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Email required"})
	}

	queries := database.GetQueries()
	_, err := queries.GetUserByEmail(c.Context(), email)
	isSignUp := err != nil // If error (user not found), it's a sign-up

	log.Printf("Smart detection - Mode: %s (email: %s)", map[bool]string{true: "signup", false: "signin"}[isSignUp], email)

	// For now, skip signature verification to test
	if !verifyWebhookSignature(c) {
		log.Printf("Webhook signature verification failed")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	log.Printf("Webhook event type: %s", event.Type)

	// Handle different event types
	switch event.Type {
	case "user.created":
		return handleUserCreated(c, event.Data, isSignUp)
	case "user.updated":
		return handleUserUpdated(c, event.Data)
	default:
		log.Printf("Unhandled event type: %s", event.Type)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Event received"})
	}
}

func handleUserCreated(c *fiber.Ctx, user ClerkUser, isSignUp bool) error {
	log.Printf("=== HANDLE USER CREATED ===")
	log.Printf("User ID: %s", user.ID)
	log.Printf("Mode: %s", map[bool]string{true: "signup", false: "signin"}[isSignUp])
	log.Printf("First Name: %v", user.FirstName)
	log.Printf("Last Name: %v", user.LastName)
	log.Printf("Email addresses: %+v", user.EmailAddresses)
	log.Printf("Primary email ID: %v", user.PrimaryEmailAddressID)
	log.Printf("Image URL: %v", user.ImageURL)
	log.Printf("Profile Image URL: %v", user.ProfileImageURL)

	email := getPrimaryEmail(user.EmailAddresses, user.PrimaryEmailAddressID)
	if email == "" {
		// Since we only use Google OAuth, every user MUST have an email
		log.Printf("❌ ERROR: No email found for user: %s - this should not happen with Google OAuth", user.ID)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Email required for registration"})
	}

	log.Printf("✅ Found email for user %s: %s", user.ID, email)

	queries := database.GetQueries()

	// Handle nullable fields properly
	var firstName, lastName sql.NullString
	var avatarURL sql.NullString

	if user.FirstName != nil {
		firstName = sql.NullString{String: *user.FirstName, Valid: true}
	}
	if user.LastName != nil {
		lastName = sql.NullString{String: *user.LastName, Valid: true}
	}

	// Use image_url or profile_image_url
	if user.ImageURL != nil {
		avatarURL = sql.NullString{String: *user.ImageURL, Valid: true}
	} else if user.ProfileImageURL != nil {
		avatarURL = sql.NullString{String: *user.ProfileImageURL, Valid: true}
	}

	// Check if user with this email already exists
	log.Printf("🔍 Checking if user with email %s exists in database...", email)
	existingUser, err := queries.GetUserByEmail(c.Context(), email)
	if err == nil {
		// User already exists
		log.Printf("👤 User found in database: ID=%d, ClerkID=%s, Email=%s", existingUser.ID, existingUser.ClerkID, existingUser.Email)

		if !isSignUp {
			// This is a sign-in - update existing user with new clerk_id (allowed)
			log.Printf("🔄 SIGN-IN MODE: User with email %s already exists (old clerk_id: %s), updating with new clerk_id: %s",
				email, existingUser.ClerkID, user.ID)

			_, err := queries.UpdateUser(c.Context(), database.UpdateUserParams{
				ClerkID:   user.ID, // Update with new Clerk ID
				Email:     email,
				FirstName: firstName.String,
				LastName:  lastName.String,
				AvatarUrl: avatarURL,
			})

			if err != nil {
				log.Printf("❌ Failed to update existing user: %v", err)
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update user"})
			}

			log.Printf("✅ SIGN-IN SUCCESSFUL: Updated existing user with new clerk_id: %s", user.ID)
			return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Sign-in successful"})
		} else {
			// This is a sign-up but user already exists - reject
			log.Printf("❌ SIGN-UP REJECTED: User with email %s already exists (clerk_id: %s)", email, existingUser.ClerkID)
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error":   "User already exists",
				"message": "Please use sign-in instead",
			})
		}
	} else {
		log.Printf("👻 User NOT found in database (error: %v)", err)
	}

	// User doesn't exist
	if !isSignUp {
		// This is a sign-in but user doesn't exist - reject
		log.Printf("❌ SIGN-IN REJECTED: User with email %s not found in database", email)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   "User not registered",
			"message": "Please sign up first",
		})
	}

	// This is a sign-up and user doesn't exist - create new user
	log.Printf("🆕 SIGN-UP MODE: Creating new user with email %s", email)
	_, err = queries.CreateUser(c.Context(), database.CreateUserParams{
		ClerkID:   user.ID,
		Email:     email,
		FirstName: firstName.String,
		LastName:  lastName.String,
		AvatarUrl: avatarURL,
	})

	if err != nil {
		// Check if it's a duplicate clerk_id error (shouldn't happen now, but just in case)
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			log.Printf("🔄 Clerk ID already exists: %s, updating instead", user.ID)
			return handleUserUpdated(c, user)
		}
		log.Printf("❌ Failed to create user: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create user"})
	}

	log.Printf("✅ SIGN-UP SUCCESSFUL: User created with ID: %s", user.ID)
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Sign-up successful"})
}

func handleUserUpdated(c *fiber.Ctx, user ClerkUser) error {
	log.Printf("=== HANDLE USER UPDATED ===")
	log.Printf("User ID: %s", user.ID)
	log.Printf("Email addresses: %+v", user.EmailAddresses)

	email := getPrimaryEmail(user.EmailAddresses, user.PrimaryEmailAddressID)
	if email == "" {
		// Since we only use Google OAuth, every user MUST have an email
		log.Printf("❌ ERROR: No email found for user: %s - this should not happen with Google OAuth", user.ID)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Email required for user update"})
	}

	log.Printf("✅ Found email for user %s: %s", user.ID, email)

	queries := database.GetQueries()

	// Handle nullable fields properly
	var firstName, lastName sql.NullString
	var avatarURL sql.NullString

	if user.FirstName != nil {
		firstName = sql.NullString{String: *user.FirstName, Valid: true}
	}
	if user.LastName != nil {
		lastName = sql.NullString{String: *user.LastName, Valid: true}
	}

	// Use image_url or profile_image_url
	if user.ImageURL != nil {
		avatarURL = sql.NullString{String: *user.ImageURL, Valid: true}
	} else if user.ProfileImageURL != nil {
		avatarURL = sql.NullString{String: *user.ProfileImageURL, Valid: true}
	}

	_, err := queries.UpdateUser(c.Context(), database.UpdateUserParams{
		ClerkID:   user.ID,
		Email:     email,
		FirstName: firstName.String,
		LastName:  lastName.String,
		AvatarUrl: avatarURL,
	})

	if err != nil {
		// If user doesn't exist, create them instead (assume sign-up mode for updates)
		if strings.Contains(err.Error(), "sql: no rows in result set") {
			log.Printf("🔄 User doesn't exist: %s, creating instead", user.ID)
			return handleUserCreated(c, user, true) // Assume sign-up for user.updated events
		}
		log.Printf("❌ Failed to update user: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update user"})
	}

	log.Printf("✅ USER UPDATED SUCCESSFULLY: %s", user.ID)
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "User updated successfully"})
}

func getPrimaryEmail(emails []ClerkEmailAddress, primaryEmailID *string) string {
	// If we have a primary email ID, find that specific email
	if primaryEmailID != nil {
		for _, email := range emails {
			if email.ID == *primaryEmailID {
				return email.EmailAddress
			}
		}
	}

	// Otherwise, find the first verified email
	for _, email := range emails {
		if email.Verification.Status == "verified" {
			return email.EmailAddress
		}
	}

	// If no verified email, return the first one
	if len(emails) > 0 {
		return emails[0].EmailAddress
	}

	return ""
}

func verifyWebhookSignature(c *fiber.Ctx) bool {
	// Use single webhook secret for all webhooks
	webhookSecret := os.Getenv("CLERK_WEBHOOK_SECRET")

	if webhookSecret == "" {
		log.Printf("[Webhook] CLERK_WEBHOOK_SECRET not set, skipping verification")
		return true
	}

	// Get Svix headers for signature verification
	// Try both capitalized and lowercase versions (Fiber is case-sensitive)
	svixID := c.Get("Svix-Id")
	if svixID == "" {
		svixID = c.Get("svix-id")
	}

	svixTimestamp := c.Get("Svix-Timestamp")
	if svixTimestamp == "" {
		svixTimestamp = c.Get("svix-timestamp")
	}

	svixSignature := c.Get("Svix-Signature")
	if svixSignature == "" {
		svixSignature = c.Get("svix-signature")
	}

	if svixID == "" || svixTimestamp == "" || svixSignature == "" {
		log.Printf("[Webhook] Missing Svix headers - ID: %v, Timestamp: %v, Signature: %v",
			svixID != "", svixTimestamp != "", svixSignature != "")
		return false
	}

	log.Printf("[Webhook] Found Svix headers - ID: %s, Timestamp: %s", svixID, svixTimestamp)

	// Manual HMAC-SHA256 verification (Svix standard)
	// The signature format is: "v1,<base64_signature>"
	signatureParts := strings.Split(svixSignature, ",")
	if len(signatureParts) != 2 || signatureParts[0] != "v1" {
		log.Printf("[Webhook] Invalid signature format: %s", svixSignature)
		return false
	}

	expectedSignature := signatureParts[1]

	// Remove 'whsec_' prefix from webhook secret if present
	secret := strings.TrimPrefix(webhookSecret, "whsec_")

	// Decode the base64-encoded secret (Svix secrets are base64-encoded)
	decodedSecret, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		log.Printf("[Webhook] Failed to decode webhook secret: %v", err)
		return false
	}

	// Create the signed content: "{msg_id}.{timestamp}.{body}"
	payload := c.Body()
	signedContent := fmt.Sprintf("%s.%s.%s", svixID, svixTimestamp, string(payload))

	// Compute HMAC-SHA256 using the decoded secret
	h := hmac.New(sha256.New, decodedSecret)
	h.Write([]byte(signedContent))
	computedSignature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	// Compare signatures
	if !hmac.Equal([]byte(computedSignature), []byte(expectedSignature)) {
		log.Printf("[Webhook] ❌ Signature verification FAILED")
		log.Printf("[Webhook] Expected: %s", expectedSignature)
		log.Printf("[Webhook] Computed: %s", computedSignature)
		return false
	}

	log.Printf("[Webhook] ✅ Signature verification successful")
	return true
}
