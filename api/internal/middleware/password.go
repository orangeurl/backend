package middleware

import (
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/xenonnn4w/orangeurl/internal/database"
	"golang.org/x/crypto/bcrypt"
)

const (
	UnlockSessionTTL     = 30 * time.Minute  // Session lasts 30 minutes after successful unlock
	MaxPasswordAttempts  = 5                 // Max attempts before rate limit
	RateLimitWindow      = 15 * time.Minute  // Rate limit window
	TemporaryLockAttempts = 10               // Lock after this many attempts
)

// GetUnlockSessionKey returns the Redis key for unlock sessions
func GetUnlockSessionKey(shortID, ip string) string {
	return fmt.Sprintf("unlock:%s:%s", shortID, ip)
}

// GetRateLimitKey returns the Redis key for rate limiting
func GetRateLimitKey(shortID, ip string) string {
	return fmt.Sprintf("ratelimit:%s:%s", shortID, ip)
}

// IsURLUnlocked checks if a URL has been unlocked for this session
func IsURLUnlocked(shortID, ip string) bool {
	r := database.CreateClient(0)
	defer r.Close()

	key := GetUnlockSessionKey(shortID, ip)
	exists, err := r.Exists(database.Ctx, key).Result()
	return err == nil && exists > 0
}

// SetURLUnlocked marks a URL as unlocked for this session
func SetURLUnlocked(shortID, ip string) error {
	r := database.CreateClient(0)
	defer r.Close()

	key := GetUnlockSessionKey(shortID, ip)
	return r.Set(database.Ctx, key, "1", UnlockSessionTTL).Err()
}

// CheckRateLimit checks if IP has exceeded rate limit for password attempts
func CheckRateLimit(shortID, ip string) (bool, int, error) {
	r := database.CreateClient(0)
	defer r.Close()

	key := GetRateLimitKey(shortID, ip)
	attempts, err := r.Get(database.Ctx, key).Int()

	if err == redis.Nil {
		// No attempts yet
		return true, 0, nil
	}

	if err != nil {
		return false, 0, err
	}

	if attempts >= MaxPasswordAttempts {
		// Get TTL to inform user when they can try again
		ttl, _ := r.TTL(database.Ctx, key).Result()
		if ttl > 0 {
			return false, attempts, fmt.Errorf("rate limit exceeded, try again in %v", ttl.Round(time.Second))
		}
		// TTL expired, reset counter
		r.Del(database.Ctx, key)
		return true, 0, nil
	}

	return true, attempts, nil
}

// IncrementRateLimit increments the rate limit counter
func IncrementRateLimit(shortID, ip string) error {
	r := database.CreateClient(0)
	defer r.Close()

	key := GetRateLimitKey(shortID, ip)
	pipe := r.Pipeline()

	pipe.Incr(database.Ctx, key)
	pipe.Expire(database.Ctx, key, RateLimitWindow)

	_, err := pipe.Exec(database.Ctx)
	return err
}

// VerifyPassword verifies a password against a bcrypt hash
func VerifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// HashPassword creates a bcrypt hash of the password
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// PasswordProtectionMiddleware checks if a URL is password-protected
func PasswordProtectionMiddleware(c *fiber.Ctx) error {
	shortID := c.Params("url")
	ip := c.IP()

	// Check if already unlocked in this session
	if IsURLUnlocked(shortID, ip) {
		return c.Next()
	}

	// Check if URL exists and is locked
	queries := database.GetQueries()
	urlRecord, err := queries.GetURLByShortID(c.Context(), shortID)

	if err != nil {
		// URL not found, let the resolve handler deal with it
		return c.Next()
	}

	// If URL is not locked, continue
	if !urlRecord.IsLocked.Valid || !urlRecord.IsLocked.Bool {
		return c.Next()
	}

	// URL is locked, redirect to unlock page
	unlockURL := fmt.Sprintf("https://app.orangeurl.live/l/%s", shortID)
	fmt.Printf("[PasswordMiddleware] Redirecting locked URL %s to %s\n", shortID, unlockURL)
	return c.Redirect(unlockURL, fiber.StatusTemporaryRedirect)
}
