package database

import (
	"context"
	"os"

	"github.com/go-redis/redis/v8"
)

var Ctx = context.Background()

func CreateClient(dbNo int) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("DB_ADDR"),
		Password: os.Getenv(("DB_PASS")),
		DB:       dbNo,
	})
	return rdb
}

// GetRedis returns a Redis client for general use (DB 0)
// Caller is responsible for closing the client
func GetRedis() *redis.Client {
	return CreateClient(0)
}

// GetAllURLs returns all URL mappings (short_id -> original_url)
// Uses SCAN instead of KEYS * to avoid blocking production traffic
func GetAllURLs() (map[string]string, error) {
	r := CreateClient(0)
	defer r.Close()

	urlMap := make(map[string]string)
	var cursor uint64

	// Use SCAN to iterate through keys without blocking
	for {
		var keys []string
		var err error

		// SCAN returns a batch of keys and a cursor for the next iteration
		keys, cursor, err = r.Scan(Ctx, cursor, "*", 100).Result()
		if err != nil {
			return nil, err
		}

		// Get values for this batch of keys
		for _, key := range keys {
			val, err := r.Get(Ctx, key).Result()
			if err == nil {
				urlMap[key] = val
			}
		}

		// cursor == 0 means we've iterated through all keys
		if cursor == 0 {
			break
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
