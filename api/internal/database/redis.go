package database

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

var Ctx = context.Background()

func CreateClient(dbNo int) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("DB_ADDR"),
		Password: os.Getenv("DB_PASS"),
		DB:       dbNo,
	})
	return rdb
}

// GetAllURLs returns all URL mappings (short_id -> original_url)
func GetAllURLs() (map[string]string, error) {
	r := CreateClient(0)
	defer r.Close()

	keys, err := r.Keys(Ctx, "*").Result()
	if err != nil {
		return nil, err
	}

	urlMap := make(map[string]string)
	for _, key := range keys {
		val, err := r.Get(Ctx, key).Result()
		if err == nil {
			urlMap[key] = val
		}
	}

	return urlMap, nil
}

// GetURLByShortID returns the original URL for a given short ID
func GetURLByShortID(shortID string) (string, error) {
	r := CreateClient(0)
	defer r.Close()

	return r.Get(Ctx, shortID).Result()
}

// GetTotalURLCount returns the total number of URLs stored
func GetTotalURLCount() (int64, error) {
	r := CreateClient(0)
	defer r.Close()

	return r.DBSize(Ctx).Result()
}

// CacheURL stores URL mapping in Redis with optional expiry
func CacheURL(shortID string, originalURL string, expiry time.Duration) error {
	r := CreateClient(0)
	defer r.Close()

	if expiry > 0 {
		return r.Set(Ctx, shortID, originalURL, expiry).Err()
	}
	return r.Set(Ctx, shortID, originalURL, 0).Err()
}

// DeleteURLCache removes URL from Redis cache
func DeleteURLCache(shortID string) error {
	r := CreateClient(0)
	defer r.Close()

	return r.Del(Ctx, shortID).Err()
}

// Click Tracking with Redis

// IncrementURLClickCount increments click count for a URL in Redis
func IncrementURLClickCount(urlID uuid.UUID) error {
	r := CreateClient(1)
	defer r.Close()

	key := fmt.Sprintf("url:clicks:%s", urlID.String())
	return r.Incr(Ctx, key).Err()
}

// GetURLClickCountFromCache gets click count from Redis cache
func GetURLClickCountFromCache(urlID uuid.UUID) (int64, error) {
	r := CreateClient(1)
	defer r.Close()

	key := fmt.Sprintf("url:clicks:%s", urlID.String())
	count, err := r.Get(Ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return count, err
}

// User Session Management

type UserSession struct {
	UserID        uuid.UUID              `json:"user_id"`
	ClerkID       string                 `json:"clerk_id"`
	Email         string                 `json:"email"`
	Tier          string                 `json:"tier"`
	DeviceInfo    map[string]string      `json:"device_info"`
	LastActivity  time.Time              `json:"last_activity"`
}

// SetUserSession stores user session in Redis
func SetUserSession(clerkID string, session UserSession) error {
	r := CreateClient(2)
	defer r.Close()

	sessionData, err := json.Marshal(session)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("session:%s", clerkID)
	// Sessions expire after 24 hours of inactivity
	return r.Set(Ctx, key, sessionData, 24*time.Hour).Err()
}

// GetUserSession retrieves user session from Redis
func GetUserSession(clerkID string) (*UserSession, error) {
	r := CreateClient(2)
	defer r.Close()

	key := fmt.Sprintf("session:%s", clerkID)
	sessionData, err := r.Get(Ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var session UserSession
	if err := json.Unmarshal([]byte(sessionData), &session); err != nil {
		return nil, err
	}

	return &session, nil
}

// DeleteUserSession removes user session from Redis
func DeleteUserSession(clerkID string) error {
	r := CreateClient(2)
	defer r.Close()

	key := fmt.Sprintf("session:%s", clerkID)
	return r.Del(Ctx, key).Err()
}

// Analytics Caching

// CacheUserAnalytics caches user analytics data for quick retrieval
func CacheUserAnalytics(userID uuid.UUID, analyticsData interface{}, expiry time.Duration) error {
	r := CreateClient(3)
	defer r.Close()

	data, err := json.Marshal(analyticsData)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("analytics:user:%s", userID.String())
	return r.Set(Ctx, key, data, expiry).Err()
}

// GetCachedUserAnalytics retrieves cached analytics data
func GetCachedUserAnalytics(userID uuid.UUID) ([]byte, error) {
	r := CreateClient(3)
	defer r.Close()

	key := fmt.Sprintf("analytics:user:%s", userID.String())
	data, err := r.Get(Ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	return data, err
}

// InvalidateUserAnalyticsCache removes cached analytics
func InvalidateUserAnalyticsCache(userID uuid.UUID) error {
	r := CreateClient(3)
	defer r.Close()

	key := fmt.Sprintf("analytics:user:%s", userID.String())
	return r.Del(Ctx, key).Err()
}

// Track click metrics in real-time
func IncrementClickMetric(userID uuid.UUID, metricType string, value string) error {
	r := CreateClient(1)
	defer r.Close()

	key := fmt.Sprintf("metrics:%s:%s:%s", userID.String(), metricType, value)
	return r.Incr(Ctx, key).Err()
}

// Get click metrics
func GetClickMetrics(userID uuid.UUID, metricType string) (map[string]int64, error) {
	r := CreateClient(1)
	defer r.Close()

	pattern := fmt.Sprintf("metrics:%s:%s:*", userID.String(), metricType)
	keys, err := r.Keys(Ctx, pattern).Result()
	if err != nil {
		return nil, err
	}

	metrics := make(map[string]int64)
	for _, key := range keys {
		count, err := r.Get(Ctx, key).Int64()
		if err == nil {
			// Extract value from key
			// Format: metrics:userID:metricType:value
			parts := len(fmt.Sprintf("metrics:%s:%s:", userID.String(), metricType))
			value := key[parts:]
			metrics[value] = count
		}
	}

	return metrics, nil
}

// SyncURLClicksToRedis syncs click counts from PostgreSQL to Redis
func SyncURLClicksToRedis(urlID uuid.UUID, clickCount int64) error {
	r := CreateClient(1)
	defer r.Close()

	key := fmt.Sprintf("url:clicks:%s", urlID.String())
	return r.Set(Ctx, key, clickCount, 0).Err()
}
