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
