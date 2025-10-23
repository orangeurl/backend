package analytics

import (
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/xenonnn4w/orangeurl/internal/database"
	"github.com/xenonnn4w/orangeurl/internal/middleware"
)

type AnalyticsResponse struct {
	TotalClicks      int64                  `json:"total_clicks"`
	TotalURLs        int64                  `json:"total_urls"`
	ClicksByCountry  []CountryClicks        `json:"clicks_by_country"`
	ClicksByDevice   []DeviceClicks         `json:"clicks_by_device"`
	ClicksByBrowser  []BrowserClicks        `json:"clicks_by_browser"`
	ClicksByOS       []OSClicks             `json:"clicks_by_os"`
	ClicksOverTime   []TimeSeriesData       `json:"clicks_over_time"`
	TopURLs          []TopURL               `json:"top_urls"`
	BotPercentage    float64                `json:"bot_percentage"`
	DataRetention    string                 `json:"data_retention"`
}

type CountryClicks struct {
	Country    string `json:"country"`
	ClickCount int64  `json:"click_count"`
}

type DeviceClicks struct {
	DeviceType string `json:"device_type"`
	ClickCount int64  `json:"click_count"`
}

type BrowserClicks struct {
	Browser    string `json:"browser"`
	ClickCount int64  `json:"click_count"`
}

type OSClicks struct {
	OS         string `json:"os"`
	ClickCount int64  `json:"click_count"`
}

type TimeSeriesData struct {
	Date       string `json:"date"`
	ClickCount int64  `json:"click_count"`
}

type TopURL struct {
	ID          uuid.UUID `json:"id"`
	ShortID     string    `json:"short_id"`
	OriginalURL string    `json:"original_url"`
	ClickCount  int64     `json:"click_count"`
}

type URLAnalyticsResponse struct {
	URL              database.Url           `json:"url"`
	TotalClicks      int64                  `json:"total_clicks"`
	ClicksByCountry  []CountryClicks        `json:"clicks_by_country"`
	ClicksByDevice   []DeviceClicks         `json:"clicks_by_device"`
	ClicksByBrowser  []BrowserClicks        `json:"clicks_by_browser"`
	ClicksByOS       []OSClicks             `json:"clicks_by_os"`
	ClicksOverTime   []TimeSeriesData       `json:"clicks_over_time"`
	TopReferrers     []ReferrerData         `json:"top_referrers"`
	RecentClicks     []database.UrlClick    `json:"recent_clicks"`
}

type ReferrerData struct {
	Referer    string `json:"referer"`
	ClickCount int64  `json:"click_count"`
}

// GetUserAnalytics returns analytics for the authenticated user based on their tier
func GetUserAnalytics(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	// Check cache first
	cachedData, err := database.GetCachedUserAnalytics(user.ID)
	if err == nil && cachedData != nil {
		var analytics AnalyticsResponse
		if err := json.Unmarshal(cachedData, &analytics); err == nil {
			return c.JSON(analytics)
		}
	}

	queries := database.GetQueries()

	// Get total clicks
	totalClicksResult, err := queries.GetUserTotalClicks(c.Context(), user.ID)
	if err != nil {
		totalClicksResult = 0
	}

	// Get user URLs to count
	userURLs, err := queries.GetUserURLs(c.Context(), user.ID)
	if err != nil {
		userURLs = []database.Url{}
	}

	// Determine data retention period based on tier
	var startDate time.Time
	var dataRetention string
	
	switch user.SubscriptionTier {
	case "free":
		startDate = time.Now().AddDate(0, -1, 0) // 1 month
		dataRetention = "1 month"
	case "pro":
		startDate = time.Now().AddDate(-1, 0, 0) // 1 year
		dataRetention = "1 year"
	case "premium":
		startDate = time.Time{} // All time
		dataRetention = "unlimited"
	default:
		startDate = time.Now().AddDate(0, -1, 0)
		dataRetention = "1 month"
	}

	// Get clicks by country
	clicksByCountryRows, err := queries.GetUserClicksByCountry(c.Context(), user.ID)
	clicksByCountry := make([]CountryClicks, 0)
	if err == nil {
		for _, row := range clicksByCountryRows {
			if row.Country.Valid {
				clicksByCountry = append(clicksByCountry, CountryClicks{
					Country:    row.Country.String,
					ClickCount: row.ClickCount,
				})
			}
		}
	}

	// Get clicks by device
	clicksByDeviceRows, err := queries.GetUserClicksByDevice(c.Context(), user.ID)
	clicksByDevice := make([]DeviceClicks, 0)
	if err == nil {
		for _, row := range clicksByDeviceRows {
			if row.DeviceType.Valid {
				clicksByDevice = append(clicksByDevice, DeviceClicks{
					DeviceType: row.DeviceType.String,
					ClickCount: row.ClickCount,
				})
			}
		}
	}

	// Get clicks by browser
	clicksByBrowserRows, err := queries.GetUserClicksByBrowser(c.Context(), user.ID)
	clicksByBrowser := make([]BrowserClicks, 0)
	if err == nil {
		for _, row := range clicksByBrowserRows {
			if row.Browser.Valid {
				clicksByBrowser = append(clicksByBrowser, BrowserClicks{
					Browser:    row.Browser.String,
					ClickCount: row.ClickCount,
				})
			}
		}
	}

	// Get clicks by OS
	clicksByOSRows, err := queries.GetUserClicksByOS(c.Context(), user.ID)
	clicksByOS := make([]OSClicks, 0)
	if err == nil {
		for _, row := range clicksByOSRows {
			if row.Os.Valid {
				clicksByOS = append(clicksByOS, OSClicks{
					OS:         row.Os.String,
					ClickCount: row.ClickCount,
				})
			}
		}
	}

	// Get clicks over time
	clicksOverTimeRows, err := queries.GetUserClicksOverTime(c.Context(), database.GetUserClicksOverTimeParams{
		UserID: user.ID,
		ClickedAt: startDate,
	})
	clicksOverTime := make([]TimeSeriesData, 0)
	if err == nil {
		for _, row := range clicksOverTimeRows {
			if row.Date.Valid {
				clicksOverTime = append(clicksOverTime, TimeSeriesData{
					Date:       row.Date.Time.Format("2006-01-02"),
					ClickCount: row.ClickCount,
				})
			}
		}
	}

	// Get top URLs
	topURLsRows, err := queries.GetUserTopURLs(c.Context(), database.GetUserTopURLsParams{
		UserID: user.ID,
		Limit:  10,
	})
	topURLs := make([]TopURL, 0)
	if err == nil {
		for _, row := range topURLsRows {
			topURLs = append(topURLs, TopURL{
				ID:          row.ID,
				ShortID:     row.ShortID,
				OriginalURL: row.OriginalUrl,
				ClickCount:  row.ClickCount.Int64,
			})
		}
	}

	// Get bot percentage
	botPercentageResult, err := queries.GetUserBotClicksPercentage(c.Context(), user.ID)
	botPercentage := 0.0
	if err == nil && botPercentageResult.Valid {
		botPercentage = botPercentageResult.Float64
	}

	analytics := AnalyticsResponse{
		TotalClicks:     totalClicksResult,
		TotalURLs:       int64(len(userURLs)),
		ClicksByCountry: clicksByCountry,
		ClicksByDevice:  clicksByDevice,
		ClicksByBrowser: clicksByBrowser,
		ClicksByOS:      clicksByOS,
		ClicksOverTime:  clicksOverTime,
		TopURLs:         topURLs,
		BotPercentage:   botPercentage,
		DataRetention:   dataRetention,
	}

	// Cache the results for 5 minutes
	database.CacheUserAnalytics(user.ID, analytics, 5*time.Minute)

	return c.JSON(analytics)
}

// GetURLAnalytics returns analytics for a specific URL
func GetURLAnalytics(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	urlIDParam := c.Params("urlId")
	urlID, err := uuid.Parse(urlIDParam)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid URL ID"})
	}

	queries := database.GetQueries()

	// Get URL and verify ownership
	url, err := queries.GetURLByShortID(c.Context(), urlIDParam)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "URL not found"})
	}

	if url.UserID != user.ID {
		return c.Status(403).JSON(fiber.Map{"error": "Access denied"})
	}

	// Get total clicks
	clickCountResult, err := queries.GetURLClickCount(c.Context(), urlID)
	totalClicks := int64(0)
	if err == nil {
		totalClicks = clickCountResult
	}

	// Determine data retention period based on tier
	var startDate time.Time
	
	switch user.SubscriptionTier {
	case "free":
		startDate = time.Now().AddDate(0, -1, 0) // 1 month
	case "pro":
		startDate = time.Now().AddDate(-1, 0, 0) // 1 year
	case "premium":
		startDate = time.Time{} // All time
	default:
		startDate = time.Now().AddDate(0, -1, 0)
	}

	// Get clicks by country
	clicksByCountryRows, err := queries.GetURLClicksByCountry(c.Context(), urlID)
	clicksByCountry := make([]CountryClicks, 0)
	if err == nil {
		for _, row := range clicksByCountryRows {
			if row.Country.Valid {
				clicksByCountry = append(clicksByCountry, CountryClicks{
					Country:    row.Country.String,
					ClickCount: row.ClickCount,
				})
			}
		}
	}

	// Get clicks by device
	clicksByDeviceRows, err := queries.GetURLClicksByDevice(c.Context(), urlID)
	clicksByDevice := make([]DeviceClicks, 0)
	if err == nil {
		for _, row := range clicksByDeviceRows {
			if row.DeviceType.Valid {
				clicksByDevice = append(clicksByDevice, DeviceClicks{
					DeviceType: row.DeviceType.String,
					ClickCount: row.ClickCount,
				})
			}
		}
	}

	// Get clicks by browser
	clicksByBrowserRows, err := queries.GetURLClicksByBrowser(c.Context(), urlID)
	clicksByBrowser := make([]BrowserClicks, 0)
	if err == nil {
		for _, row := range clicksByBrowserRows {
			if row.Browser.Valid {
				clicksByBrowser = append(clicksByBrowser, BrowserClicks{
					Browser:    row.Browser.String,
					ClickCount: row.ClickCount,
				})
			}
		}
	}

	// Get clicks by OS
	clicksByOSRows, err := queries.GetURLClicksByOS(c.Context(), urlID)
	clicksByOS := make([]OSClicks, 0)
	if err == nil {
		for _, row := range clicksByOSRows {
			if row.Os.Valid {
				clicksByOS = append(clicksByOS, OSClicks{
					OS:         row.Os.String,
					ClickCount: row.ClickCount,
				})
			}
		}
	}

	// Get clicks over time
	clicksOverTimeRows, err := queries.GetURLClicksOverTime(c.Context(), database.GetURLClicksOverTimeParams{
		UrlID: urlID,
		ClickedAt: startDate,
	})
	clicksOverTime := make([]TimeSeriesData, 0)
	if err == nil {
		for _, row := range clicksOverTimeRows {
			if row.Date.Valid {
				clicksOverTime = append(clicksOverTime, TimeSeriesData{
					Date:       row.Date.Time.Format("2006-01-02"),
					ClickCount: row.ClickCount,
				})
			}
		}
	}

	// Get top referrers
	topReferrersRows, err := queries.GetURLReferrers(c.Context(), database.GetURLReferrersParams{
		UrlID: urlID,
		Limit: 10,
	})
	topReferrers := make([]ReferrerData, 0)
	if err == nil {
		for _, row := range topReferrersRows {
			if row.Referer.Valid {
				topReferrers = append(topReferrers, ReferrerData{
					Referer:    row.Referer.String,
					ClickCount: row.ClickCount,
				})
			}
		}
	}

	// Get recent clicks (limited based on tier)
	limit := int32(50)
	if user.SubscriptionTier == "free" {
		limit = 10
	} else if user.SubscriptionTier == "pro" {
		limit = 50
	} else if user.SubscriptionTier == "premium" {
		limit = 100
	}

	recentClicks, err := queries.GetURLAnalyticsWithLimit(c.Context(), database.GetURLAnalyticsWithLimitParams{
		UrlID: urlID,
		Limit: limit,
	})
	if err != nil {
		recentClicks = []database.UrlClick{}
	}

	analytics := URLAnalyticsResponse{
		URL:             url,
		TotalClicks:     totalClicks,
		ClicksByCountry: clicksByCountry,
		ClicksByDevice:  clicksByDevice,
		ClicksByBrowser: clicksByBrowser,
		ClicksByOS:      clicksByOS,
		ClicksOverTime:  clicksOverTime,
		TopReferrers:    topReferrers,
		RecentClicks:    recentClicks,
	}

	return c.JSON(analytics)
}

