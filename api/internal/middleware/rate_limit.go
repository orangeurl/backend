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
