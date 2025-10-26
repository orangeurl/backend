package dashboard

import (
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
	RecentURLs       []URLWithStats    `json:"recent_urls"`
	
	// Feature availability based on tier
	Features         TierFeatures      `json:"features"`
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
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "User not found"})
	}

	queries := database.GetQueries()

	// Get user's subscription to determine tier
	subscription, subErr := queries.GetUserSubscription(c.Context(), user.ID)
	planTier := "free"
	if subErr == nil && subscription.PlanID != "" {
		planTier = subscription.PlanID
	}

	// Get URL count
	urlCount, err := queries.GetUserURLCount(c.Context(), user.ID)
	if err != nil {
		urlCount = 0
	}

	// Get click stats
	clickStats, err := queries.GetUserClickStats(c.Context(), user.ID)
	totalClicks := int64(0)
	if err == nil {
		totalClicks = clickStats.TotalClicks
	}

	// Get top countries
	countries, _ := queries.GetUserClicksByCountry(c.Context(), user.ID)
	topCountries := make([]CountryStats, 0)
	for _, c := range countries {
		if c.Country.Valid {
			topCountries = append(topCountries, CountryStats{
				Country: c.Country.String,
				Clicks:  c.Clicks,
			})
		}
	}

	// Get recent URLs with stats
	recentURLs := make([]URLWithStats, 0)
	urls, err := queries.GetUserURLs(c.Context(), user.ID)
	if err == nil {
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
		BrowserStats:    []BrowserStats{}, // TODO: Implement if needed
		DeviceStats:     []DeviceStats{},  // TODO: Implement if needed
		RecentURLs:      recentURLs,
		Features:        features,
	}

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

