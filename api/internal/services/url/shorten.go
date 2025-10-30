package url

import (
	"database/sql"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/xenonnn4w/orangeurl/internal/database"
	"github.com/xenonnn4w/orangeurl/internal/handlers/url"
	"github.com/xenonnn4w/orangeurl/internal/middleware"
)

type request struct {
	URL          string        `json:"url"`
	CustomShort  string        `json:"short"`
	Expiry       time.Duration `json:"expiry"`
	CustomExpiry *time.Time    `json:"custom_expiry"` // Premium feature - specific expiry date
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

	// rate limiting
	r2 := database.CreateClient(1)
	defer r2.Close()

	// determine effective quota once (fallback to 10 if unset)
	quota := os.Getenv("API_QUOTA")
	if quota == "" {
		quota = "100"
	}

	val, err := r2.Get(database.Ctx, c.IP()).Result()
	if err == redis.Nil {
		// initialize quota for this IP
		r2.Set(database.Ctx, c.IP(), quota, 30*60*time.Second).Err()
	} else {
		valInt, convErr := strconv.Atoi(val)
		if convErr != nil {
			// repair bad value by resetting quota
			r2.Set(database.Ctx, c.IP(), quota, 30*60*time.Second).Err()
		} else if valInt <= 0 {
			limit, _ := r2.TTL(database.Ctx, c.IP()).Result()
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "Rate limit exceeded",
				//doubt
				"rate_limit_rest": limit / time.Nanosecond / time.Minute,
			})
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
	// TODO: investigate the working of this; misbheaving.

	body.URL = url.EnforceHTTP(body.URL)

	// assigning url according to the custom id
	// TODO: allow all 26+9+26 combination; currently only supporting 26+9

	var id string

	if body.CustomShort == "" {
		id = uuid.New().String()[:6]
	} else {
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

	val, _ = r.Get(database.Ctx, id).Result()
	if val != "" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "URL already taken"})
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
	user, userErr := middleware.GetUserFromContext(c)
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
		
		tierLimit := 5 // Free tier default
		if tier == "pro" {
			tierLimit = 100
		} else if tier == "premium" {
			tierLimit = 500
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

		_, pgErr := queries.CreateURL(c.Context(), database.CreateURLParams{
			UserID:       user.ID,
			ShortID:      id,
			OriginalUrl:  body.URL,
			Expiry:       expiryTime,
			IsActive:     sql.NullBool{Bool: true, Valid: true},
			PasswordHash: sql.NullString{Valid: false}, // No password by default
			IsLocked:     sql.NullBool{Bool: false, Valid: true},
		})

		if pgErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to save URL to database",
				"details": pgErr.Error(),
			})
		}

		// Increment monthly usage counter
		_, _ = queries.IncrementURLCount(c.Context(), database.IncrementURLCountParams{
			UserID: user.ID,
			Month:  currentMonth,
		})
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
