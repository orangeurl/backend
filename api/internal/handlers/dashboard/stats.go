package dashboard

import (
	"log"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/xenonnn4w/orangeurl/internal/database"
	"github.com/xenonnn4w/orangeurl/internal/middleware"
)

// Pricing tier limits
const (
	FreeTierLinks      = 5   // Free: 5 URLs per month
	ProTierLinks       = 100 // Pro: 100 URLs per month
	PremiumTierLinks   = 500 // Premium: 500 URLs per month
	FreeTierRetention  = 30  // 1 month in days
	ProTierRetention   = 365 // 1 year in days
	// Premium has custom retention
)

type DashboardStats struct {
	// General stats
	TotalURLs        int64             `json:"total_urls"`
	TotalClicks      int64             `json:"total_clicks"`
	URLsThisMonth    int64             `json:"urls_this_month"`
	ClicksThisMonth  int64             `json:"clicks_this_month"`
	
	// User tier info
	PlanName         string            `json:"plan_name"`
	URLLimit         int               `json:"url_limit"`
	URLsRemaining    int               `json:"urls_remaining"`
	
	// Analytics
	TopCountries     []CountryStats    `json:"top_countries"`
	BrowserStats     []BrowserStats    `json:"browser_stats"`
	DeviceStats      []DeviceStats     `json:"device_stats"`
	ClicksByDate     []DateStats       `json:"clicks_by_date"`
	RecentURLs       []URLWithStats    `json:"recent_urls"`
	
	// Feature availability based on tier
	Features         TierFeatures      `json:"features"`
}

type DateStats struct {
	Date   string `json:"date"`
	Clicks int64  `json:"clicks"`
}

type CountryStats struct {
	Country string `json:"country"`
	Clicks  int64  `json:"clicks"`
}

type BrowserStats struct {
	Browser string `json:"browser"`
	Clicks  int64  `json:"clicks"`
}

type DeviceStats struct {
	DeviceType string `json:"device_type"`
	Clicks     int64  `json:"clicks"`
}

type ReferrerStats struct {
	Source string `json:"source"`
	Clicks int64  `json:"clicks"`
}

type URLWithStats struct {
	ID          string    `json:"id"`
	ShortID     string    `json:"short_id"`
	OriginalURL string    `json:"original_url"`
	Clicks      int64     `json:"clicks"`
	CreatedAt   time.Time `json:"created_at"`
	IsActive    bool      `json:"is_active"`
	IsLocked    bool      `json:"is_locked"`
}

type TierFeatures struct {
	AdvancedAnalytics    bool   `json:"advanced_analytics"`
	CustomLinks          int    `json:"custom_links"`
	CustomQRCodes        int    `json:"custom_qr_codes"`
	BioLinks             int    `json:"bio_links"`
	PrioritySupport      bool   `json:"priority_support"`
	AIShortener          bool   `json:"ai_shortener"`
	CustomExpiry         bool   `json:"custom_expiry"`
	LinkLocking          bool   `json:"link_locking"`
	DataRetentionDays    int    `json:"data_retention_days"`
}

// GetDashboardStats returns comprehensive dashboard statistics based on user's tier
func GetDashboardStats(c *fiber.Ctx) error {
	log.Println("[Dashboard] GetDashboardStats called")
	
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		log.Printf("[Dashboard] Failed to get user from context: %v", err)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
	}

	log.Printf("[Dashboard] User found: ID=%s, Email=%s", user.ID, user.Email)
	queries := database.GetQueries()
	if queries == nil {
		log.Println("[Dashboard] ERROR: queries is nil - database not initialized")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database not initialized"})
	}

	// Get user's subscription to determine tier
	// Check both subscriptions table and user.subscription_tier as fallback
	subscription, subErr := queries.GetUserSubscription(c.Context(), user.ID)
	planTier := user.SubscriptionTier // Use user table tier as default
	
	if planTier == "" {
		planTier = "free"
	}
	
	// Subscription table overrides user table if exists
	if subErr == nil && subscription.PlanID != "" {
		planTier = subscription.PlanID
		log.Printf("[Dashboard] Subscription found: plan=%s", planTier)
	} else {
		log.Printf("[Dashboard] No subscription found (error: %v) - using user.subscription_tier: %s", subErr, planTier)
	}

	// Get personal URL count (excluding team URLs - for limits)
	log.Println("[Dashboard] Fetching personal URL count...")
	urlCount, err := queries.GetUserURLCount(c.Context(), user.ID)
	if err != nil {
		log.Printf("[Dashboard] Error getting personal URL count: %v", err)
		urlCount = 0
	} else {
		log.Printf("[Dashboard] Personal URL count: %d", urlCount)
	}

	// Get click stats
	log.Println("[Dashboard] Fetching click stats...")
	clickStats, err := queries.GetUserClickStats(c.Context(), user.ID)
	totalClicks := int64(0)
	if err == nil {
		totalClicks = clickStats.TotalClicks
		log.Printf("[Dashboard] Total clicks: %d", totalClicks)
	} else {
		log.Printf("[Dashboard] Error getting click stats: %v", err)
	}

	// Get monthly click stats
	log.Println("[Dashboard] Fetching monthly click stats...")
	monthlyClicks, err := queries.GetMonthlyClickStats(c.Context(), user.ID)
	if err != nil {
		log.Printf("[Dashboard] Error getting monthly click stats: %v", err)
		monthlyClicks = 0
	} else {
		log.Printf("[Dashboard] Monthly clicks: %d", monthlyClicks)
	}

	// Get monthly URL count
	log.Println("[Dashboard] Fetching monthly URL count...")
	monthlyURLCount, err := queries.GetMonthlyURLCount(c.Context(), user.ID)
	if err != nil {
		log.Printf("[Dashboard] Error getting monthly URL count: %v", err)
		monthlyURLCount = 0
	} else {
		log.Printf("[Dashboard] Monthly URL count: %d", monthlyURLCount)
	}

	// Get top countries
	log.Println("[Dashboard] Fetching top countries...")
	countries, err := queries.GetUserClicksByCountry(c.Context(), user.ID)
	topCountries := make([]CountryStats, 0)
	if err != nil {
		log.Printf("[Dashboard] Error getting countries: %v", err)
	} else {
		for _, country := range countries {
			if country.Country.Valid {
				topCountries = append(topCountries, CountryStats{
					Country: country.Country.String,
					Clicks:  country.Clicks,
				})
			}
		}
		log.Printf("[Dashboard] Found %d countries", len(topCountries))
	}

	// Get browser stats
	log.Println("[Dashboard] Fetching browser stats...")
	browsers, err := queries.GetUserClicksByBrowser(c.Context(), user.ID)
	browserStats := make([]BrowserStats, 0)
	if err != nil {
		log.Printf("[Dashboard] Error getting browsers: %v", err)
	} else {
		for _, browser := range browsers {
			if browser.Browser.Valid {
				browserStats = append(browserStats, BrowserStats{
					Browser: browser.Browser.String,
					Clicks:  browser.Clicks,
				})
			}
		}
		log.Printf("[Dashboard] Found %d browsers", len(browserStats))
	}

	// Get device stats
	log.Println("[Dashboard] Fetching device stats...")
	devices, err := queries.GetUserClicksByDevice(c.Context(), user.ID)
	deviceStats := make([]DeviceStats, 0)
	if err != nil {
		log.Printf("[Dashboard] Error getting devices: %v", err)
	} else {
		for _, device := range devices {
			if device.DeviceType.Valid {
				deviceStats = append(deviceStats, DeviceStats{
					DeviceType: device.DeviceType.String,
					Clicks:     device.Clicks,
				})
			}
		}
		log.Printf("[Dashboard] Found %d device types", len(deviceStats))
	}

	// Get clicks by date
	log.Println("[Dashboard] Fetching clicks by date...")
	clicksByDate, err := queries.GetUserClicksByDate(c.Context(), user.ID)
	dateStats := make([]DateStats, 0)
	if err != nil {
		log.Printf("[Dashboard] Error getting clicks by date: %v", err)
	} else {
		for _, day := range clicksByDate {
			dateStats = append(dateStats, DateStats{
				Date:   day.Date.Format("2006-01-02"),
				Clicks: day.Clicks,
			})
		}
		log.Printf("[Dashboard] Found %d days of data", len(dateStats))
	}

	// Get recent URLs with stats (including team URLs)
	log.Println("[Dashboard] Fetching recent URLs...")
	recentURLs := make([]URLWithStats, 0)
	urls, err := queries.GetUserAndTeamURLs(c.Context(), user.ID)
	if err == nil {
		log.Printf("[Dashboard] Found %d URLs", len(urls))
		for _, url := range urls {
			// Get click count for this URL
			clicks, _ := queries.GetURLAnalytics(c.Context(), url.ID)

			// Use current time if CreatedAt is not valid
			createdAt := time.Now()
			if url.CreatedAt.Valid {
				createdAt = url.CreatedAt.Time
			}

			recentURLs = append(recentURLs, URLWithStats{
				ID:          url.ID.String(),
				ShortID:     url.ShortID,
				OriginalURL: url.OriginalUrl,
				Clicks:      int64(len(clicks)),
				CreatedAt:   createdAt,
				IsActive:    url.IsActive.Bool,
				IsLocked:    url.IsLocked.Valid && url.IsLocked.Bool,
			})
			if len(recentURLs) >= 10 {
				break
			}
		}
	} else {
		log.Printf("[Dashboard] Error getting URLs: %v", err)
	}

	// Determine tier features and limits
	features := getTierFeatures(planTier)
	urlLimit := getURLLimit(planTier)
	urlsRemaining := urlLimit - int(urlCount)
	if urlsRemaining < 0 {
		urlsRemaining = 0
	}

	stats := DashboardStats{
		TotalURLs:       urlCount,
		TotalClicks:     totalClicks,
		URLsThisMonth:   monthlyURLCount,
		ClicksThisMonth: monthlyClicks,
		PlanName:        getPlanDisplayName(planTier),
		URLLimit:        urlLimit,
		URLsRemaining:   urlsRemaining,
		TopCountries:    topCountries,
		BrowserStats:    browserStats,
		DeviceStats:     deviceStats,
		ClicksByDate:    dateStats,
		RecentURLs:      recentURLs,
		Features:        features,
	}

	log.Printf("[Dashboard] Returning stats: URLs=%d, Clicks=%d, Plan=%s", urlCount, totalClicks, planTier)
	return c.JSON(stats)
}

// GetURLAnalytics returns detailed analytics for a specific URL
func GetURLAnalytics(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
	}

	shortID := c.Params("shortId")
	if shortID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Short ID required"})
	}

	queries := database.GetQueries()

	// Get URL and verify ownership or team access
	url, err := queries.GetURLByShortID(c.Context(), shortID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "URL not found"})
	}

	// Check if user can access this URL (owner or team member)
	canAccess, err := queries.CanUserAccessURL(c.Context(), database.CanUserAccessURLParams{
		ID:     url.ID,
		UserID: user.ID,
	})
	if err != nil || !canAccess {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Access denied"})
	}

	// Get analytics data
	clicks, err := queries.GetURLAnalytics(c.Context(), url.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch analytics"})
	}

	// Get clicks by date
	clicksByDate, _ := queries.GetURLClicksByDate(c.Context(), url.ID)

	// Get browser stats
	browserStats, _ := queries.GetURLClicksByBrowser(c.Context(), url.ID)
	browsers := make([]BrowserStats, 0)
	for _, b := range browserStats {
		if b.Browser.Valid {
			browsers = append(browsers, BrowserStats{
				Browser: b.Browser.String,
				Clicks:  b.Clicks,
			})
		}
	}

	// Get device stats
	deviceStats, _ := queries.GetURLClicksByDevice(c.Context(), url.ID)
	devices := make([]DeviceStats, 0)
	for _, d := range deviceStats {
		if d.DeviceType.Valid {
			devices = append(devices, DeviceStats{
				DeviceType: d.DeviceType.String,
				Clicks:     d.Clicks,
			})
		}
	}

	// Parse referrer sources from clicks - use exact URLs
	referrerMap := make(map[string]int64)
	for _, click := range clicks {
		if click.Referer.Valid && click.Referer.String != "" && click.Referer.String != "-" {
			// Use exact referrer URL instead of categorizing
			referrerMap[click.Referer.String]++
		} else {
			referrerMap["Direct"]++
		}
	}
	
	referrerStats := make([]ReferrerStats, 0)
	for source, clicks := range referrerMap {
		referrerStats = append(referrerStats, ReferrerStats{
			Source: source,
			Clicks: clicks,
		})
	}

	// Get referrer clicks by date for the area chart
	referrerClicksByDate, _ := queries.GetURLClicksByReferrerAndDate(c.Context(), url.ID)

	// Transform data: group by date with referrer breakdown
	dateReferrerMap := make(map[string]map[string]int64)
	allSources := make(map[string]bool)

	for _, row := range referrerClicksByDate {
		dateStr := row.Date.Format("2006-01-02")

		// Use exact referrer URL instead of categorizing
		var source string
		if row.Referer.Valid && row.Referer.String != "" && row.Referer.String != "-" {
			source = row.Referer.String
		} else {
			source = "Direct"
		}

		// Initialize date map if not exists
		if dateReferrerMap[dateStr] == nil {
			dateReferrerMap[dateStr] = make(map[string]int64)
		}

		// Add clicks to the source for this date
		dateReferrerMap[dateStr][source] += row.Clicks
		allSources[source] = true
	}

	// Ensure all dates have all sources (fill in 0 for missing sources)
	for dateStr := range dateReferrerMap {
		for source := range allSources {
			if _, exists := dateReferrerMap[dateStr][source]; !exists {
				dateReferrerMap[dateStr][source] = 0
			}
		}
	}

	// Format as array of objects with date and all sources as keys
	referrerClicksByDateFormatted := make([]map[string]interface{}, 0)
	for dateStr, sources := range dateReferrerMap {
		entry := map[string]interface{}{
			"date": dateStr,
		}

		// Add each source as a key with click count
		totalForDate := int64(0)
		for source, clicks := range sources {
			entry[source] = clicks
			totalForDate += clicks
		}

		// Only include dates with clicks > 0
		if totalForDate > 0 {
			referrerClicksByDateFormatted = append(referrerClicksByDateFormatted, entry)
		}
	}

	// Sort by date descending
	sort.Slice(referrerClicksByDateFormatted, func(i, j int) bool {
		return referrerClicksByDateFormatted[i]["date"].(string) > referrerClicksByDateFormatted[j]["date"].(string)
	})

	// Format clicks by date for response
	formattedClicksByDate := make([]DateStats, 0)
	for _, dateStat := range clicksByDate {
		formattedClicksByDate = append(formattedClicksByDate, DateStats{
			Date:   dateStat.Date.Format("2006-01-02"),
			Clicks: dateStat.Clicks,
		})
	}

	// Format recent clicks with proper timestamps
	recentClicksFormatted := make([]map[string]interface{}, 0)
	for _, click := range clicks[:min(len(clicks), 50)] {
		recentClick := map[string]interface{}{
			"clicked_at": click.ClickedAt.Time,
			"device_type": click.DeviceType.String,
			"browser": click.Browser.String,
			"os": click.Os.String,
			"country": click.Country.String,
			"city": click.City.String,
			"referer": click.Referer.String,
			"bot": click.IsBot.Bool,
		}
		recentClicksFormatted = append(recentClicksFormatted, recentClick)
	}

	// Format created_at properly
	var createdAt interface{}
	if url.CreatedAt.Valid {
		createdAt = url.CreatedAt.Time
	} else {
		createdAt = nil
	}

	response := fiber.Map{
		"url": fiber.Map{
			"short_id":     url.ShortID,
			"original_url": url.OriginalUrl,
			"created_at":   createdAt,
			"is_active":    url.IsActive,
		},
		"total_clicks":             len(clicks),
		"clicks_by_date":           formattedClicksByDate,
		"browser_stats":            browsers,
		"device_stats":             devices,
		"referrer_stats":           referrerStats,
		"referrer_clicks_by_date":  referrerClicksByDateFormatted,
		"recent_clicks":            recentClicksFormatted,
	}

	return c.JSON(response)
}

// parseReferrerSource categorizes referrer URLs into meaningful sources
func parseReferrerSource(referer string) string {
	ref := strings.ToLower(referer)

	if strings.Contains(ref, "youtube.com") || strings.Contains(ref, "youtu.be") {
		return "YouTube"
	} else if strings.Contains(ref, "twitter.com") || strings.Contains(ref, "x.com") {
		return "X"
	} else if strings.Contains(ref, "instagram.com") {
		return "Instagram"
	} else if strings.Contains(ref, "linkedin.com") {
		return "LinkedIn"
	} else if strings.Contains(ref, "medium.com") {
		return "Medium"
	} else if strings.Contains(ref, "facebook.com") || strings.Contains(ref, "fb.com") {
		return "Facebook"
	} else if strings.Contains(ref, "reddit.com") {
		return "Reddit"
	} else if strings.Contains(ref, "tiktok.com") {
		return "TikTok"
	} else if strings.Contains(ref, "t.me") || strings.Contains(ref, "telegram.org") {
		return "Telegram"
	} else if strings.Contains(ref, "discord.com") || strings.Contains(ref, "discord.gg") {
		return "Discord"
	} else if strings.Contains(ref, "google.com") || strings.Contains(ref, "google.") {
		return "Google"
	}

	return "Other"
}

// Helper functions
func getTierFeatures(planTier string) TierFeatures {
	switch planTier {
	case "pro":
		return TierFeatures{
			AdvancedAnalytics:    true,
			CustomLinks:          5,
			CustomQRCodes:        5,
			BioLinks:             1,
			PrioritySupport:      true,
			AIShortener:          true,
			CustomExpiry:         false,
			LinkLocking:          false,
			DataRetentionDays:    ProTierRetention,
		}
	case "premium":
		return TierFeatures{
			AdvancedAnalytics:    true,
			CustomLinks:          15,
			CustomQRCodes:        15,
			BioLinks:             3,
			PrioritySupport:      true,
			AIShortener:          true,
			CustomExpiry:         true,
			LinkLocking:          true,
			DataRetentionDays:    -1, // Custom/unlimited
		}
	default: // free
		return TierFeatures{
			AdvancedAnalytics:    true,
			CustomLinks:          0,
			CustomQRCodes:        0,
			BioLinks:             0,
			PrioritySupport:      false,
			AIShortener:          false,
			CustomExpiry:         false,
			LinkLocking:          false,
			DataRetentionDays:    FreeTierRetention,
		}
	}
}

func getURLLimit(planTier string) int {
	switch planTier {
	case "pro":
		return ProTierLinks
	case "premium":
		return PremiumTierLinks
	default:
		return FreeTierLinks
	}
}

func getPlanDisplayName(planTier string) string {
	switch planTier {
	case "pro":
		return "Pro"
	case "premium":
		return "Premium"
	default:
		return "Free"
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetPersonalURLs returns only personal URLs (not shared with team)
func GetPersonalURLs(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	queries := database.GetQueries()
	urls, err := queries.GetUserPersonalURLs(c.Context(), user.ID)
	if err != nil {
		log.Printf("[GetPersonalURLs] Error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch URLs"})
	}

	// Format URLs with click stats
	urlsWithStats := make([]URLWithStats, 0)
	for _, url := range urls {
		clicks, _ := queries.GetURLAnalytics(c.Context(), url.ID)

		createdAt := time.Now()
		if url.CreatedAt.Valid {
			createdAt = url.CreatedAt.Time
		}

		urlsWithStats = append(urlsWithStats, URLWithStats{
			ID:          url.ID.String(),
			ShortID:     url.ShortID,
			OriginalURL: url.OriginalUrl,
			Clicks:      int64(len(clicks)),
			CreatedAt:   createdAt,
			IsActive:    url.IsActive.Bool,
			IsLocked:    url.IsLocked.Valid && url.IsLocked.Bool,
		})
	}

	return c.JSON(fiber.Map{
		"urls": urlsWithStats,
		"count": len(urlsWithStats),
	})
}

// GetTeamURLs returns only URLs shared with user's team
func GetTeamURLs(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	queries := database.GetQueries()
	urls, err := queries.GetUserTeamURLs(c.Context(), user.ID)
	if err != nil {
		log.Printf("[GetTeamURLs] Error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch team URLs"})
	}

	// Format URLs with click stats
	urlsWithStats := make([]URLWithStats, 0)
	for _, url := range urls {
		clicks, _ := queries.GetURLAnalytics(c.Context(), url.ID)

		createdAt := time.Now()
		if url.CreatedAt.Valid {
			createdAt = url.CreatedAt.Time
		}

		urlsWithStats = append(urlsWithStats, URLWithStats{
			ID:          url.ID.String(),
			ShortID:     url.ShortID,
			OriginalURL: url.OriginalUrl,
			Clicks:      int64(len(clicks)),
			CreatedAt:   createdAt,
			IsActive:    url.IsActive.Bool,
			IsLocked:    url.IsLocked.Valid && url.IsLocked.Bool,
		})
	}

	return c.JSON(fiber.Map{
		"urls": urlsWithStats,
		"count": len(urlsWithStats),
	})
}

