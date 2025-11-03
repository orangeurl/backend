package middleware

import (
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/xenonnn4w/orangeurl/internal/database"
)

// WebhookRateLimit creates a rate limiter specifically for webhook endpoints
// Allows 100 requests per minute per IP to prevent abuse
func WebhookRateLimit() fiber.Handler {
	return func(c *fiber.Ctx) error {
		ip := c.IP()

		// Extract first IP from comma-separated list (if behind proxy)
		if idx := len(ip); idx > 0 {
			// Already handled by Fiber proxy config, but just in case
			ip = c.IP()
		}

		redisClient := database.GetRedis()
		if redisClient == nil {
			log.Printf("[RateLimit] Redis client not available, skipping rate limit")
			return c.Next()
		}

		key := fmt.Sprintf("webhook_ratelimit:%s", ip)

		// Increment counter
		val, err := redisClient.Incr(database.Ctx, key).Result()
		if err != nil {
			log.Printf("[RateLimit] Redis error: %v", err)
			return c.Next() // Allow request on Redis error
		}

		// Set expiry on first request
		if val == 1 {
			redisClient.Expire(database.Ctx, key, 1*time.Minute)
		}

		// Allow 100 requests per minute for webhooks
		limit := int64(100)
		if val > limit {
			log.Printf("[RateLimit] Webhook rate limit exceeded for IP: %s (count: %d)", ip, val)
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Rate limit exceeded",
				"retry_after": "60 seconds",
			})
		}

		// Add rate limit headers
		c.Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
		c.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", limit-val))

		return c.Next()
	}
}

// AuthRateLimit creates a rate limiter for authentication endpoints
// More strict: 10 requests per 5 minutes per IP to prevent brute force
func AuthRateLimit() fiber.Handler {
	return func(c *fiber.Ctx) error {
		ip := c.IP()

		redisClient := database.GetRedis()
		if redisClient == nil {
			log.Printf("[RateLimit] Redis client not available, skipping rate limit")
			return c.Next()
		}

		key := fmt.Sprintf("auth_ratelimit:%s", ip)

		// Increment counter
		val, err := redisClient.Incr(database.Ctx, key).Result()
		if err != nil {
			log.Printf("[RateLimit] Redis error: %v", err)
			return c.Next() // Allow request on Redis error
		}

		// Set expiry on first request (5 minutes window)
		if val == 1 {
			redisClient.Expire(database.Ctx, key, 5*time.Minute)
		}

		// Allow only 10 auth attempts per 5 minutes
		limit := int64(10)
		if val > limit {
			ttl, _ := redisClient.TTL(database.Ctx, key).Result()
			log.Printf("[RateLimit] Auth rate limit exceeded for IP: %s (count: %d)", ip, val)
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Too many authentication attempts",
				"retry_after": fmt.Sprintf("%d seconds", int(ttl.Seconds())),
			})
		}

		// Add rate limit headers
		c.Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
		c.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", limit-val))

		return c.Next()
	}
}


// APIRateLimit creates a rate limiter for API key requests based on subscription tier
func APIRateLimit() fiber.Handler {
	return func(c *fiber.Ctx) error {
		user, err := GetUserFromContext(c)
		if err != nil {
			// If no user in context, skip rate limiting (authentication will fail anyway)
			return c.Next()
		}

		redisClient := database.GetRedis()
		if redisClient == nil {
			log.Printf("[RateLimit] Redis client not available, skipping rate limit")
			return c.Next()
		}

		// Determine rate limit based on subscription tier
		var limit int64
		var window time.Duration
		switch user.SubscriptionTier {
		case "free":
			limit = 100
			window = 1 * time.Hour
		case "pro":
			limit = 1000
			window = 1 * time.Hour
		case "premium":
			limit = 10000
			window = 1 * time.Hour
		default:
			limit = 100
			window = 1 * time.Hour
		}

		key := fmt.Sprintf("api_ratelimit:%s:%d", user.ID, time.Now().Unix()/int64(window.Seconds()))

		// Increment counter
		val, err := redisClient.Incr(database.Ctx, key).Result()
		if err != nil {
			log.Printf("[RateLimit] Redis error: %v", err)
			return c.Next() // Allow request on Redis error
		}

		// Set expiry on first request
		if val == 1 {
			redisClient.Expire(database.Ctx, key, window)
		}

		// Check limit
		if val > limit {
			ttl, _ := redisClient.TTL(database.Ctx, key).Result()
			resetTime := time.Now().Add(ttl).Unix()

			log.Printf("[RateLimit] API rate limit exceeded for user: %s (tier: %s, count: %d/%d)",
				user.Email, user.SubscriptionTier, val, limit)

			// Add rate limit headers
			c.Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
			c.Set("X-RateLimit-Remaining", "0")
			c.Set("X-RateLimit-Reset", fmt.Sprintf("%d", resetTime))

			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "API rate limit exceeded",
				"limit": limit,
				"reset": resetTime,
				"message": fmt.Sprintf("Rate limit: %d requests per hour for %s tier", limit, user.SubscriptionTier),
			})
		}

		// Add rate limit headers
		c.Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
		c.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", limit-val))
		ttl, _ := redisClient.TTL(database.Ctx, key).Result()
		resetTime := time.Now().Add(ttl).Unix()
		c.Set("X-RateLimit-Reset", fmt.Sprintf("%d", resetTime))

		return c.Next()
	}
}
