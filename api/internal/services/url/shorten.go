package url

import (
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
)

type request struct {
	URL         string        `json:"url"`
	CustomShort string        `json:"short"`
	Expiry      time.Duration `json:"expiry"`
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
		id = body.CustomShort
	}

	r := database.CreateClient(0)
	defer r.Close()

	val, _ = r.Get(database.Ctx, id).Result()
	if val != "" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "URL already taken"})
	}

	// checking the expiry

	if body.Expiry == 0 {
		body.Expiry = 24
	}

	// Store in Redis
	err = r.Set(database.Ctx, id, body.URL, body.Expiry*3600*time.Second).Err()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "unable to connect to server"})
	}

	// Also store in PostgreSQL for analytics and persistence
	queries := database.GetQueries()
	
	// Get user from context (if authenticated)
	userID := c.Locals("user_id")
	var userUUID *uuid.UUID
	if userID != nil {
		if uid, ok := userID.(uuid.UUID); ok {
			userUUID = &uid
		}
	}
	
	// If no user, create a temporary user ID for anonymous links
	if userUUID == nil {
		tempID := uuid.New()
		userUUID = &tempID
	}
	
	// Calculate expiry time
	var expiryTime *time.Time
	if body.Expiry > 0 {
		exp := time.Now().Add(time.Duration(body.Expiry) * time.Hour)
		expiryTime = &exp
	}
	
	// Store in PostgreSQL
	_, err = queries.CreateURL(database.Ctx, database.CreateURLParams{
		UserID:      *userUUID,
		ShortID:     id,
		OriginalUrl: body.URL,
		Expiry:      expiryTime,
		IsActive:    true,
	})
	
	if err != nil {
		// If PostgreSQL fails, remove from Redis to maintain consistency
		r.Del(database.Ctx, id)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "unable to save URL to database"})
	}

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
