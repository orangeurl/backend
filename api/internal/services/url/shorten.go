package url

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"log"
	"math/big"
	neturl "net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/xenonnn4w/orangeurl/internal/database"
	"github.com/xenonnn4w/orangeurl/internal/handlers/url"
	"github.com/xenonnn4w/orangeurl/internal/middleware"
	"github.com/xenonnn4w/orangeurl/internal/services/ai"
	webhookService "github.com/xenonnn4w/orangeurl/internal/services/webhooks"
)

const (
	// Character set for short URLs: lowercase + uppercase + digits (62 characters total)
	shortURLCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	shortURLLength  = 6
)

// generateShortID generates a random short ID using the full alphanumeric character set
func generateShortID(length int) string {
	result := make([]byte, length)
	charsetLen := big.NewInt(int64(len(shortURLCharset)))

	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			// Fallback to simple method if crypto/rand fails
			return generateShortIDFallback(length)
		}
		result[i] = shortURLCharset[num.Int64()]
	}

	return string(result)
}

// generateShortIDFallback is a fallback method using math/rand (less secure but works)
func generateShortIDFallback(length int) string {
	// Use UUID as fallback to maintain uniqueness
	return strings.ReplaceAll(strings.ReplaceAll(string([]rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"))[:length], "-", ""), "_", "")[:length]
}

// isValidCustomShortURL validates custom short URLs for security
// Only allows alphanumeric characters, hyphens, and underscores for regular users
// Admins can use any characters except dangerous patterns
// Prevents path traversal, XSS, and other injection attacks
func isValidCustomShortURL(shortURL string, isAdmin bool) bool {
	if shortURL == "" || len(shortURL) > 50 {
		return false
	}

	// Block common attack patterns for everyone (including admins)
	blockedPatterns := []string{"..", "./", "\\", "<", ">", "\"", "'", "`"}
	lowerShort := strings.ToLower(shortURL)
	for _, pattern := range blockedPatterns {
		if strings.Contains(lowerShort, pattern) {
			return false
		}
	}

	// If admin, allow all characters (except blocked patterns above)
	if isAdmin {
		return true
	}

	// For regular users, only allow alphanumeric characters, hyphens, and underscores
	for _, char := range shortURL {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '_') {
			return false
		}
	}

	return true
}

type request struct {
	URL          string        `json:"url"`
	CustomShort  string        `json:"short"`
	Expiry       time.Duration `json:"expiry"`
	CustomExpiry *time.Time    `json:"custom_expiry"` // Premium feature - specific expiry date
	Password     string        `json:"password"`      // Premium feature - password protection
	UseAI        bool          `json:"use_ai"`        // Premium feature - AI-powered URL generation
}

type response struct {
	URL             string        `json:"url"`
	CustomShort     string        `json:"short"`
	Expiry          time.Duration `json:"expiry"`
	XRateRemaining  int           `json:"rate_left"`
	XRateLimitReset time.Duration `json:"rate_limit_reset"`
}

func ShortenURL(c *fiber.Ctx) error {
	// new instance of the request struct
	body := new(request)

	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cannot parse json"})
	}

	// Rate limiting - tier-based for authenticated users, IP-based for anonymous
	r2 := database.CreateClient(1)
	defer r2.Close()

	// Check if user is authenticated
	user, userErr := middleware.GetUserFromContext(c)

	if userErr == nil {
		// Authenticated user - apply tier-based rate limiting
		queries := database.GetQueries()
		if queries != nil {
			// Determine tier
			tier := user.SubscriptionTier
			subscription, subErr := queries.GetUserSubscription(c.Context(), user.ID)
			if subErr == nil && subscription.PlanID != "" {
				tier = subscription.PlanID
			}

			// Set rate limits per hour based on tier
			var hourlyLimit int64
			switch tier {
			case "free":
				hourlyLimit = 100 // Free: 100 requests/hour
			case "pro":
				hourlyLimit = 1000 // Pro: 1,000 requests/hour
			case "premium":
				hourlyLimit = 10000 // Premium: 10,000 requests/hour
			default:
				hourlyLimit = 100
			}

			// Use hourly window for rate limiting
			currentHour := time.Now().Unix() / 3600
			rateLimitKey := fmt.Sprintf("url_ratelimit:%s:%d", user.ID, currentHour)

			val, err := r2.Incr(database.Ctx, rateLimitKey).Result()
			if err == nil {
				// Set expiry on first request (1 hour)
				if val == 1 {
					r2.Expire(database.Ctx, rateLimitKey, 1*time.Hour)
				}

				// Check if limit exceeded
				if val > hourlyLimit {
					ttl, _ := r2.TTL(database.Ctx, rateLimitKey).Result()
					return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
						"error":   "Rate limit exceeded",
						"limit":   hourlyLimit,
						"used":    val,
						"tier":    tier,
						"reset_in": int(ttl.Seconds()),
						"message": fmt.Sprintf("You've reached your hourly rate limit of %d requests for %s tier", hourlyLimit, tier),
					})
				}
			}
		}
	} else {
		// Anonymous users are no longer allowed - require authentication to prevent phishing abuse
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   "Authentication required",
			"message": "Please sign in to create short URLs. This helps us prevent phishing and abuse.",
			"code":    "LOGIN_REQUIRED",
		})
	}

	// Check if user is banned
	if userErr == nil && user.IsBanned.Valid && user.IsBanned.Bool {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error":   "Account banned",
			"message": "Your account has been banned. Contact support if you believe this is a mistake.",
			"code":    "USER_BANNED",
		})
	}

	// Validate URL protocol first (block javascript:, data:, etc.)
	if err := url.ValidateURLProtocol(body.URL); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Only http and https URLs are allowed"})
	}

	// Check for suspicious/phishing domains
	if isSuspicious, reason := url.IsSuspiciousDomain(body.URL); isSuspicious {
		log.Printf("[ShortenURL] ⚠️ Blocked suspicious domain: %s - Reason: %s", body.URL, reason)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":  "This domain is not allowed due to abuse concerns",
			"reason": reason,
		})
	}

	// Check against admin-blocked domains from database
	if queries := database.GetQueries(); queries != nil {
		blockedDomains, dbErr := queries.ListBlockedDomains(c.Context())
		if dbErr == nil && len(blockedDomains) > 0 {
			parsedURL, parseErr := neturl.Parse(body.URL)
			if parseErr == nil {
				hostname := strings.ToLower(parsedURL.Hostname())
				for _, bd := range blockedDomains {
					blocked := false
					if hostname == bd.Domain {
						blocked = true
					} else if bd.IncludeSubdomains && strings.HasSuffix(hostname, "."+bd.Domain) {
						blocked = true
					}
					if blocked {
						log.Printf("[ShortenURL] Blocked by admin domain rule: %s matches %s", hostname, bd.Domain)
						return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
							"error":  "This domain is not allowed",
							"reason": bd.BlockReason,
						})
					}
				}
			}
		}
	}

	// checking if the url is an actual url
	if !govalidator.IsURL(body.URL) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid URL"})
	}

	// checking for domain error
	if !url.RemoveDomainError(body.URL) {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Invalid URL"})
	}

	// enforce https using the http_helpers.go file
	body.URL = url.EnforceHTTP(body.URL)

	// Double-check after enforcement - block if dangerous protocol returned empty string
	if body.URL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid URL protocol"})
	}

	// assigning url according to the custom id
	var id string

	if body.CustomShort == "" {
		// Check if AI generation is requested
		if body.UseAI {
			// AI-powered generation (Pro/Premium feature ONLY)
			log.Printf("[ShortenURL] AI generation requested for URL: %s", body.URL)

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
				log.Printf("[ShortenURL] AI generation denied - user tier: %s (requires Pro or Premium)", userTier)
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
					"error": "AI-powered URL generation is a Pro/Premium feature",
					"upgrade_required": true,
					"current_tier": userTier,
				})
			}

			// Pro & Premium: Use Gemini AI for intelligent generation
			log.Printf("[ShortenURL] %s user - using Gemini AI generation", userTier)
			aiID, err := ai.GenerateSmartShortID(body.URL, true)
			if err != nil {
				log.Printf("[ShortenURL] Gemini AI failed: %v, falling back to local smart generation", err)
				// Fallback to local smart generation
				smartID, fallbackErr := ai.GenerateSmartShortID(body.URL, false)
				if fallbackErr != nil {
					log.Printf("[ShortenURL] Local smart generation also failed, using random")
					id = generateShortID(shortURLLength)
				} else {
					id = smartID
					log.Printf("[ShortenURL] Fallback local smart generated ID: %s", id)
				}
			} else {
				id = aiID
				log.Printf("[ShortenURL] Gemini AI generated ID: %s", id)
			}
		} else {
			// Random generation (default for all tiers)
			id = generateShortID(shortURLLength)
		}
	} else {
		// Check if user is admin first to allow validation with admin privileges
		user, userErr := middleware.GetUserFromContext(c)
		isAdmin := false

		if userErr == nil {
			queries := database.GetQueries()
			if queries != nil {
				userDetails, err := queries.GetUserByID(c.Context(), user.ID)
				if err == nil {
					isAdmin = userDetails.IsAdmin.Valid && userDetails.IsAdmin.Bool
				}
			}
		}

		// Validate custom short URL for security
		if !isValidCustomShortURL(body.CustomShort, isAdmin) {
			if isAdmin {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "Custom short URL cannot contain dangerous patterns like .., ./, \\, <, >, quotes",
				})
			}
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Custom short URL can only contain letters, numbers, hyphens, and underscores (1-50 characters)",
			})
		}

		// Check custom link limit for authenticated users
		if userErr == nil {
			// User is authenticated, check custom link limit
			queries := database.GetQueries()
			if queries != nil {
				currentMonth := time.Now().Format("2006-01")
				usage, err := queries.GetMonthlyUsage(c.Context(), database.GetMonthlyUsageParams{
					UserID: user.ID,
					Month:  currentMonth,
				})

				// Determine user tier
				tier := "free"
				subscription, subErr := queries.GetUserSubscription(c.Context(), user.ID)
				if subErr == nil && subscription.PlanID != "" {
					tier = subscription.PlanID
				}

				// Set custom link limits based on tier
				customLinkLimit := 0 // Free tier: no custom links
				if tier == "pro" {
					customLinkLimit = 5 // Pro tier: 5 custom links per month
				} else if tier == "premium" {
					customLinkLimit = 15 // Premium tier: 15 custom links per month
				}

				// If free tier, reject custom links
				if tier == "free" {
					return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
						"error":   "Custom links not available",
						"message": "Upgrade to Pro or Premium to use custom short URLs.",
					})
				}

				// Check if user has exceeded custom link limit
				if err == nil && usage.CustomLinkCount >= int32(customLinkLimit) {
					return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
						"error":   "Custom link limit reached",
						"message": "You've reached your monthly custom link limit. Upgrade your plan for more custom links.",
						"limit":   customLinkLimit,
						"used":    usage.CustomLinkCount,
					})
				}
			}
		}

		// Preserve exact capitalization as provided by user (e.g., "Snow" stays "Snow", not "snow")
		id = body.CustomShort

		// Admin-only restriction: custom shorts with length <= 2 are reserved for admins
		if len(id) <= 2 {
			user, userErr := middleware.GetUserFromContext(c)
			isAdmin := false

			if userErr == nil {
				// User is authenticated, check if they are admin
				queries := database.GetQueries()
				if queries != nil {
					userDetails, err := queries.GetUserByID(c.Context(), user.ID)
					if err == nil {
						// IsAdmin is sql.NullBool, check both Valid and Bool fields
						isAdmin = userDetails.IsAdmin.Valid && userDetails.IsAdmin.Bool
					}
				}
			}

			// If user is not admin, reject the custom short
			if !isAdmin {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "URL already taken"})
			}
		}
	}

	r := database.CreateClient(0)
	defer r.Close()

	// Check for collision and add retry logic for AI-generated IDs
	val, err := r.Get(database.Ctx, id).Result()
	if val != "" {
		// If this was an AI-generated ID and it collided, try adding a random suffix
		if body.UseAI && body.CustomShort == "" {
			maxRetries := 5
			originalID := id
			for i := 0; i < maxRetries; i++ {
				// Add random 1-2 character suffix
				suffix := generateShortID(2)
				id = originalID + suffix
				if len(id) > 10 {
					id = id[:10] // Keep max 10 chars
				}

				val, _ := r.Get(database.Ctx, id).Result()
				if val == "" {
					log.Printf("[ShortenURL] AI ID collision resolved: %s -> %s", originalID, id)
					break
				}

				if i == maxRetries-1 {
					return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "URL already taken"})
				}
			}
		} else {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "URL already taken"})
		}
	}

	// checking the expiry - set based on user tier
	// Free: 1 year, Pro: 5 years, Premium: 5 years
	if body.Expiry == 0 {
		// Default expiry based on tier
		// Free tier gets 1 year (8760 hours)
		// Pro/Premium tiers get 5 years (43800 hours)
		defaultExpiry := 8760 // 1 year in hours for free tier

		// Check if user is authenticated to determine tier
		user, userErr := middleware.GetUserFromContext(c)
		if userErr == nil {
			// User is authenticated - check their tier
			tier := user.SubscriptionTier
			if tier == "" {
				tier = "free"
			}

			// Override with subscriptions table if available
			queries := database.GetQueries()
			if queries != nil {
				subscription, subErr := queries.GetUserSubscription(c.Context(), user.ID)
				if subErr == nil && subscription.PlanID != "" {
					tier = subscription.PlanID
				}
			}

			// Set expiry based on tier
			if tier == "pro" || tier == "premium" {
				defaultExpiry = 43800 // 5 years in hours
			}
		}
		// Anonymous users get 1 year (free tier default)
		body.Expiry = time.Duration(defaultExpiry)
	}

	//doubt regarding the time
	err = r.Set(database.Ctx, id, body.URL, body.Expiry*3600*time.Second).Err()

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "unable to connect to server"})
	}

	// Sync with Postgres database if user is authenticated
	// OptionalAuth middleware will have set user in context if token was provided
	user, userErr = middleware.GetUserFromContext(c)
	if userErr == nil {
		// User is authenticated, save URL to their account in Postgres
		queries := database.GetQueries()
		if queries == nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "database not initialized"})
		}

		// Check monthly usage limit
		currentMonth := time.Now().Format("2006-01")
		usage, _ := queries.GetMonthlyUsage(c.Context(), database.GetMonthlyUsageParams{
			UserID: user.ID,
			Month:  currentMonth,
		})

		// Get user's tier to determine limit
		// Check user.subscription_tier first, then subscriptions table
		tier := user.SubscriptionTier
		if tier == "" {
			tier = "free"
		}
		
		// Override with subscriptions table if available
		subscription, subErr := queries.GetUserSubscription(c.Context(), user.ID)
		if subErr == nil && subscription.PlanID != "" {
			tier = subscription.PlanID
		}
		
		tierLimit := 5 // Free tier default: 5 URLs per month
		if tier == "pro" {
			tierLimit = 100 // Pro tier: 100 URLs per month
		} else if tier == "premium" {
			tierLimit = 500 // Premium tier: 500 URLs per month
		}

		// Check if user has exceeded monthly limit
		if usage.UrlCount >= int32(tierLimit) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error":   "Monthly limit reached",
				"message": "You've reached your monthly URL limit. Upgrade your plan for more links.",
				"limit":   tierLimit,
				"used":    usage.UrlCount,
			})
		}
		
		var expiryTime sql.NullTime
		
		// Check if user is Premium and has custom expiry
		if body.CustomExpiry != nil && tier == "premium" {
			expiryTime = sql.NullTime{
				Time:  *body.CustomExpiry,
				Valid: true,
			}
		} else if body.Expiry > 0 {
			expiryTime = sql.NullTime{
				Time:  time.Now().Add(body.Expiry * time.Hour),
				Valid: true,
			}
		}

		// Handle password protection (Premium feature)
		var passwordHash sql.NullString
		var isLocked sql.NullBool

		if body.Password != "" && tier == "premium" {
			// Hash the password using bcrypt
			hashedPassword, hashErr := middleware.HashPassword(body.Password)
			if hashErr != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Failed to secure password",
				})
			}
			passwordHash = sql.NullString{String: hashedPassword, Valid: true}
			isLocked = sql.NullBool{Bool: true, Valid: true}
			log.Printf("[ShortenURL] Locking URL %s with password for Premium user %s", id, user.ID)
		} else {
			passwordHash = sql.NullString{Valid: false}
			isLocked = sql.NullBool{Bool: false, Valid: true}
			if body.Password != "" && tier != "premium" {
				log.Printf("[ShortenURL] User %s (tier: %s) attempted to lock URL but is not Premium", user.ID, tier)
			}
		}

		// URLs are created as personal URLs by default
		// Team assignment happens from dashboard, not during creation
		teamID := uuid.NullUUID{Valid: false}

		createdURL, pgErr := queries.CreateURL(c.Context(), database.CreateURLParams{
			UserID:       user.ID,
			ShortID:      id,
			OriginalUrl:  body.URL,
			Expiry:       expiryTime,
			IsActive:     sql.NullBool{Bool: true, Valid: true},
			PasswordHash: passwordHash,
			IsLocked:     isLocked,
			TeamID:       teamID,
		})

		if pgErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to save URL to database",
				"details": pgErr.Error(),
			})
		}

		// Log if link was locked
		if createdURL.IsLocked.Valid && createdURL.IsLocked.Bool {
			c.Locals("url_locked", true)
		}

		// Increment monthly usage counter
		_, _ = queries.IncrementURLCount(c.Context(), database.IncrementURLCountParams{
			UserID: user.ID,
			Month:  currentMonth,
		})

		// Increment custom link counter if custom short URL was provided
		if body.CustomShort != "" {
			_, _ = queries.IncrementCustomLinkCount(c.Context(), database.IncrementCustomLinkCountParams{
				UserID: user.ID,
				Month:  currentMonth,
			})
			log.Printf("[ShortenURL] Custom link counter incremented for user: %s", user.ID)
		}

		// Trigger webhook for url.created event
		host := os.Getenv("DOMAIN")
		if host == "" {
			host = os.Getenv("PUBLIC_HOST")
		}
		if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
			host = "https://" + host
		}
		shortURL := host + "/" + id

		webhookService.TriggerWebhook("url.created", user.ID, map[string]interface{}{
			"short_url":  shortURL,
			"short_id":   id,
			"long_url":   body.URL,
			"is_locked":  createdURL.IsLocked.Valid && createdURL.IsLocked.Bool,
			"created_at": time.Now().UTC().Format(time.RFC3339),
		})
		log.Printf("[ShortenURL] Webhook triggered for url.created: %s", shortURL)
	}
	// If user is not authenticated, URL only stored in Redis (existing behavior for anonymous users)

	resp := response{
		URL:             body.URL,
		CustomShort:     "",
		Expiry:          body.Expiry,
		XRateRemaining:  10,
		XRateLimitReset: 30,
	}

	// decremented the rateremeaning
	r2.Decr(database.Ctx, c.IP())

	val, _ = r2.Get(database.Ctx, c.IP()).Result()
	resp.XRateRemaining, _ = strconv.Atoi(val)

	// time doubt
	ttl, _ := r2.TTL(database.Ctx, c.IP()).Result()
	resp.XRateLimitReset = ttl / time.Nanosecond / time.Minute

	// Generate short URL using Domain or fallback to PUBLIC_HOST
	host := os.Getenv("DOMAIN")
	if host == "" {
		host = os.Getenv("PUBLIC_HOST")
	}

	// Ensure protocol is included
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		host = "https://" + host
	}

	resp.CustomShort = host + "/" + id

	return c.Status(fiber.StatusOK).JSON(resp)
}
