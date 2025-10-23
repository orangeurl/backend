package tracking

import (
	"database/sql"
	"log"

	"github.com/google/uuid"
	"github.com/xenonnn4w/orangeurl/internal/database"
	"github.com/xenonnn4w/orangeurl/internal/utils"
)

// TrackClick records a click event in both PostgreSQL and Redis
func TrackClick(urlID uuid.UUID, ipAddress, userAgent, referer string) error {
	// Parse user agent
	browser, os, deviceType := utils.ParseUserAgent(userAgent)
	isBot := utils.IsBot(userAgent)

	// Get geolocation
	country, city := utils.GetLocationFromIPCached(ipAddress)

	// Create database entry
	queries := database.GetQueries()
	
	// Convert empty strings to sql.NullString  
	var ipAddressInet sql.NullString
	if ipAddress != "" {
		ipAddressInet = sql.NullString{String: ipAddress, Valid: true}
	}

	var userAgentNull sql.NullString
	if userAgent != "" {
		userAgentNull = sql.NullString{String: userAgent, Valid: true}
	}

	var refererNull sql.NullString
	if referer != "" {
		refererNull = sql.NullString{String: referer, Valid: true}
	}

	var countryNull sql.NullString
	if country != "" && country != "Unknown" {
		countryNull = sql.NullString{String: country, Valid: true}
	}

	var cityNull sql.NullString
	if city != "" && city != "Unknown" {
		cityNull = sql.NullString{String: city, Valid: true}
	}

	var deviceTypeNull sql.NullString
	if deviceType != "" {
		deviceTypeNull = sql.NullString{String: deviceType, Valid: true}
	}

	var browserNull sql.NullString
	if browser != "" {
		browserNull = sql.NullString{String: browser, Valid: true}
	}

	var osNull sql.NullString
	if os != "" {
		osNull = sql.NullString{String: os, Valid: true}
	}

	// Create inet type for IP address
	var ipInet interface{}
	if ipAddress != "" {
		ipInet = ipAddress
	}

	// Store in PostgreSQL - we need to use raw query since pqtype.Inet is complex
	query := `INSERT INTO url_clicks (url_id, ip_address, user_agent, referer, country, city, device_type, browser, os, is_bot)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`
	
	_, err := database.DB.ExecContext(database.Ctx, query,
		urlID,
		ipInet,
		userAgentNull,
		refererNull,
		countryNull,
		cityNull,
		deviceTypeNull,
		browserNull,
		osNull,
		isBot,
	)

	if err != nil {
		log.Printf("Error storing click in PostgreSQL: %v", err)
		return err
	}

	// Update Redis counters asynchronously
	go func() {
		if err := database.IncrementURLClickCount(urlID); err != nil {
			log.Printf("Error incrementing click count in Redis: %v", err)
		}
	}()

	return nil
}

// SyncClickCountsToRedis syncs all URL click counts from PostgreSQL to Redis
func SyncClickCountsToRedis() error {
	queries := database.GetQueries()
	
	// Get all URLs
	urls, err := queries.ListURLs(database.Ctx)
	if err != nil {
		return err
	}

	// For each URL, get click count and sync to Redis
	for _, url := range urls {
		count, err := queries.GetURLClickCount(database.Ctx, url.ID)
		if err != nil {
			log.Printf("Error getting click count for URL %s: %v", url.ShortID, err)
			continue
		}

		if err := database.SyncURLClicksToRedis(url.ID, count); err != nil {
			log.Printf("Error syncing click count to Redis for URL %s: %v", url.ShortID, err)
		}
	}

	return nil
}

