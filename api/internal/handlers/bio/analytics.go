package bio

import (
	"database/sql"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/xenonnn4w/orangeurl/internal/database"
)

type TrackViewRequest struct {
	IPAddress string `json:"ip_address"`
	UserAgent string `json:"user_agent"`
	Referer   string `json:"referer"`
	Device    string `json:"device"`
	Browser   string `json:"browser"`
	OS        string `json:"os"`
	Country   string `json:"country"`
	City      string `json:"city"`
}

type TrackClickRequest struct {
	LinkID    string `json:"link_id"`
	IPAddress string `json:"ip_address"`
	UserAgent string `json:"user_agent"`
	Referer   string `json:"referer"`
	Device    string `json:"device"`
	Browser   string `json:"browser"`
	OS        string `json:"os"`
	Country   string `json:"country"`
	City      string `json:"city"`
}

// TrackView - Track a page view (public endpoint)
func TrackView(c *fiber.Ctx) error {
	username := c.Params("username")
	if username == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Username is required",
		})
	}

	var req TrackViewRequest
	if err := c.BodyParser(&req); err != nil {
		// If body parsing fails, extract from headers
		req.IPAddress = c.IP()
		req.UserAgent = c.Get("User-Agent")
		req.Referer = c.Get("Referer")
	}

	queries := database.GetQueries()

	// Get bio page ID
	bioPage, err := queries.GetBioPageByUsername(c.Context(), username)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Bio page not found",
		})
	}

	// Track view
	_, err = queries.TrackBioPageView(c.Context(), database.TrackBioPageViewParams{
		BioPageID: bioPage.ID,
		IpAddress: sql.NullString{String: req.IPAddress, Valid: req.IPAddress != ""},
		UserAgent: sql.NullString{String: req.UserAgent, Valid: req.UserAgent != ""},
		Referer:   sql.NullString{String: req.Referer, Valid: req.Referer != ""},
		Country:   sql.NullString{String: req.Country, Valid: req.Country != ""},
		City:      sql.NullString{String: req.City, Valid: req.City != ""},
		Device:    sql.NullString{String: req.Device, Valid: req.Device != ""},
		Browser:   sql.NullString{String: req.Browser, Valid: req.Browser != ""},
		Os:        sql.NullString{String: req.OS, Valid: req.OS != ""},
		IsBot:     false, // TODO: Implement bot detection
	})

	if err != nil {
		log.Printf("Error tracking view for %s: %v", username, err)
		// Don't return error to user, just log it
	}

	// Increment total views counter
	queries.IncrementBioPageViews(c.Context(), bioPage.ID)

	return c.JSON(fiber.Map{
		"success": true,
	})
}

// TrackClick - Track a link click (public endpoint)
func TrackClick(c *fiber.Ctx) error {
	username := c.Params("username")
	if username == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Username is required",
		})
	}

	var req TrackClickRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.LinkID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Link ID is required",
		})
	}

	queries := database.GetQueries()

	// Get bio page ID
	bioPage, err := queries.GetBioPageByUsername(c.Context(), username)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Bio page not found",
		})
	}

	// Track click
	_, err = queries.TrackBioLinkClick(c.Context(), database.TrackBioLinkClickParams{
		BioPageID: bioPage.ID,
		LinkID:    req.LinkID,
		IpAddress: sql.NullString{String: req.IPAddress, Valid: req.IPAddress != ""},
		UserAgent: sql.NullString{String: req.UserAgent, Valid: req.UserAgent != ""},
		Referer:   sql.NullString{String: req.Referer, Valid: req.Referer != ""},
		Country:   sql.NullString{String: req.Country, Valid: req.Country != ""},
		City:      sql.NullString{String: req.City, Valid: req.City != ""},
		Device:    sql.NullString{String: req.Device, Valid: req.Device != ""},
		Browser:   sql.NullString{String: req.Browser, Valid: req.Browser != ""},
		Os:        sql.NullString{String: req.OS, Valid: req.OS != ""},
		IsBot:     false, // TODO: Implement bot detection
	})

	if err != nil {
		log.Printf("Error tracking click for %s: %v", username, err)
	}

	// Increment total clicks counter
	queries.IncrementBioPageClicks(c.Context(), bioPage.ID)

	return c.JSON(fiber.Map{
		"success": true,
	})
}

// GetAnalytics - Get analytics for a bio page (requires auth)
func GetAnalytics(c *fiber.Ctx) error {
	username := c.Params("username")
	userID := c.Locals("userId").(string)

	if username == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Username is required",
		})
	}

	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	// Parse date range from query params (default: last 30 days)
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	var startDate, endDate time.Time
	var err error

	if startDateStr != "" {
		startDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid start date format (use YYYY-MM-DD)",
			})
		}
	} else {
		startDate = time.Now().AddDate(0, 0, -30) // 30 days ago
	}

	if endDateStr != "" {
		endDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid end date format (use YYYY-MM-DD)",
			})
		}
	} else {
		endDate = time.Now()
	}

	queries := database.GetQueries()

	// Get bio page
	bioPage, err := queries.GetBioPageByUsernameForEdit(c.Context(), database.GetBioPageByUsernameForEditParams{
		Username: username,
		UserID:   userID,
	})
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Bio page not found or you don't have permission to view it",
		})
	}

	// Get overall analytics
	analytics, err := queries.GetBioPageAnalytics(c.Context(), database.GetBioPageAnalyticsParams{
		Username: username,
		UserID:   userID,
	})
	if err != nil {
		log.Printf("Error getting analytics: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get analytics",
		})
	}

	// Get views by date
	viewsByDate, err := queries.GetBioPageViewsByDate(c.Context(), database.GetBioPageViewsByDateParams{
		BioPageID: bioPage.ID,
		ViewedAt:  startDate,
		ViewedAt_2: endDate,
	})
	if err != nil {
		log.Printf("Error getting views by date: %v", err)
	}

	// Get clicks by link
	clicksByLink, err := queries.GetBioLinkClicksByLink(c.Context(), database.GetBioLinkClicksByLinkParams{
		BioPageID: bioPage.ID,
		ClickedAt: startDate,
		ClickedAt_2: endDate,
	})
	if err != nil {
		log.Printf("Error getting clicks by link: %v", err)
	}

	// Get device stats
	deviceStats, err := queries.GetBioPageDeviceStats(c.Context(), bioPage.ID)
	if err != nil {
		log.Printf("Error getting device stats: %v", err)
	}

	// Get country stats
	countryStats, err := queries.GetBioPageCountryStats(c.Context(), bioPage.ID)
	if err != nil {
		log.Printf("Error getting country stats: %v", err)
	}

	// Get browser stats
	browserStats, err := queries.GetBioPageBrowserStats(c.Context(), bioPage.ID)
	if err != nil {
		log.Printf("Error getting browser stats: %v", err)
	}

	return c.JSON(fiber.Map{
		"overview": fiber.Map{
			"total_views":      analytics.TotalViews,
			"total_clicks":     analytics.TotalClicks,
			"unique_countries": analytics.UniqueCountries,
			"unique_devices":   analytics.UniqueDevices,
			"ctr":              float64(analytics.TotalClicks) / float64(analytics.TotalViews) * 100,
		},
		"views_by_date":  viewsByDate,
		"clicks_by_link": clicksByLink,
		"device_stats":   deviceStats,
		"country_stats":  countryStats,
		"browser_stats":  browserStats,
	})
}
