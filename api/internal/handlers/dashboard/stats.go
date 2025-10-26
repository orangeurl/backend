package dashboard

import (
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/xenonnn4w/orangeurl/internal/database"
	"github.com/xenonnn4w/orangeurl/internal/middleware"
)

// Pricing tier limits
const (
	FreeTierLinks      = 5
	ProTierLinks       = 100
	PremiumTierLinks   = 500
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

type URLWithStats struct {
	ShortID     string    `json:"short_id"`
	OriginalURL string    `json:"original_url"`
	Clicks      int64     `json:"clicks"`
	CreatedAt   time.Time `json:"created_at"`
	IsActive    bool      `json:"is_active"`
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
	subscription, subErr := queries.GetUserSubscription(c.Context(), user.ID)
	planTier := "free"
	if subErr == nil && subscription.PlanID != "" {
		planTier = subscription.PlanID
		log.Printf("[Dashboard] Subscription found: plan=%s", planTier)
	} else {
		log.Printf("[Dashboard] No subscription found or error: %v - defaulting to free", subErr)
	}

	// Get URL count
	log.Println("[Dashboard] Fetching URL count...")
	urlCount, err := queries.GetUserURLCount(c.Context(), user.ID)
	if err != nil {
		log.Printf("[Dashboard] Error getting URL count: %v", err)
		urlCount = 0
	} else {
		log.Printf("[Dashboard] URL count: %d", urlCount)
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

	// Get recent URLs with stats
	log.Println("[Dashboard] Fetching recent URLs...")
	recentURLs := make([]URLWithStats, 0)
	urls, err := queries.GetUserURLs(c.Context(), user.ID)
	if err == nil {
		log.Printf("[Dashboard] Found %d URLs", len(urls))
		for _, url := range urls {
			// Get click count for this URL
			clicks, _ := queries.GetURLAnalytics(c.Context(), url.ID)
			recentURLs = append(recentURLs, URLWithStats{
				ShortID:     url.ShortID,
				OriginalURL: url.OriginalUrl,
				Clicks:      int64(len(clicks)),
				CreatedAt:   url.CreatedAt.Time,
				IsActive:    url.IsActive.Bool,
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
		URLsThisMonth:   urlCount, // TODO: Implement monthly filtering
		ClicksThisMonth: totalClicks, // TODO: Implement monthly filtering
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

	// Get URL and verify ownership
	url, err := queries.GetURLByShortID(c.Context(), shortID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "URL not found"})
	}

	if url.UserID != user.ID {
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

	response := fiber.Map{
		"url": fiber.Map{
			"short_id":     url.ShortID,
			"original_url": url.OriginalUrl,
			"created_at":   url.CreatedAt,
			"is_active":    url.IsActive,
		},
		"total_clicks":    len(clicks),
		"clicks_by_date":  clicksByDate,
		"browser_stats":   browsers,
		"device_stats":    devices,
		"recent_clicks":   clicks[:min(len(clicks), 50)], // Return last 50 clicks
	}

	return c.JSON(response)
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

