package main

import (
	"context"
	"log"
	"os"

	_ "github.com/lib/pq"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/joho/godotenv"
	"github.com/xenonnn4w/orangeurl/internal/database"
	"github.com/xenonnn4w/orangeurl/internal/handlers/admin"
	"github.com/xenonnn4w/orangeurl/internal/handlers/analytics"
	"github.com/xenonnn4w/orangeurl/internal/handlers/api_keys"
	"github.com/xenonnn4w/orangeurl/internal/handlers/auth"
	"github.com/xenonnn4w/orangeurl/internal/handlers/dashboard"
	"github.com/xenonnn4w/orangeurl/internal/handlers/docs"
	"github.com/xenonnn4w/orangeurl/internal/handlers/payment"
	"github.com/xenonnn4w/orangeurl/internal/handlers/qr"
	"github.com/xenonnn4w/orangeurl/internal/handlers/teams"
	"github.com/xenonnn4w/orangeurl/internal/handlers/urls"
	"github.com/xenonnn4w/orangeurl/internal/handlers/waitlist"
	"github.com/xenonnn4w/orangeurl/internal/handlers/webhooks"
	"github.com/xenonnn4w/orangeurl/internal/jobs"
	"github.com/xenonnn4w/orangeurl/internal/middleware"
	"github.com/xenonnn4w/orangeurl/internal/routes"
	"github.com/xenonnn4w/orangeurl/internal/services/subscription"
	"github.com/xenonnn4w/orangeurl/internal/services/tracking"
	urlService "github.com/xenonnn4w/orangeurl/internal/services/url"
)

// getAPIKeyAccount returns account info for API key owner
func getAPIKeyAccount(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	return c.JSON(fiber.Map{
		"id":                user.ID,
		"email":             user.Email,
		"first_name":        user.FirstName,
		"last_name":         user.LastName,
		"subscription_tier": user.SubscriptionTier,
		"created_at":        user.CreatedAt,
	})
}

func setupRoutes(app *fiber.App) {
	// Health check
	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "OrangeURL API is running"})
	})

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "healthy"})
	})

	// API Documentation
	app.Get("/docs", docs.ServeSwaggerUI)
	app.Get("/docs/openapi.yaml", docs.ServeOpenAPISpec)

	// Public routes
	app.Get("/:url", middleware.PasswordProtectionMiddleware, routes.ResolveURL)
	// URL shortening with optional auth (saves to user account if authenticated)
	app.Post("/api/v1", middleware.OptionalAuth(), urlService.ShortenURL)

	// AI suggestion endpoint (requires auth - Pro/Premium feature)
	app.Post("/api/v1/suggest", middleware.RequireAuth(), urls.SuggestShortID)

	// Webhook routes (public - no auth required, but rate limited)
	app.Post("/api/webhooks/clerk", middleware.WebhookRateLimit(), auth.HandleClerkWebhook)
	app.Post("/api/webhooks/dodo", middleware.WebhookRateLimit(), subscription.HandleDodoWebhook)

	// Legacy dodo webhook handler (keep for backward compatibility)
	app.Post("/api/webhooks/dodo-legacy", middleware.WebhookRateLimit(), payment.HandleDodoWebhook)

	// Waitlist route (public - no auth required)
	app.Post("/api/v1/api/waitlist", waitlist.JoinWaitlist)

	// Protected routes
	api := app.Group("/api")
	
	// Dashboard routes (with auth)
	api.Get("/dashboard/stats", middleware.RequireAuth(), dashboard.GetDashboardStats)
	api.Get("/dashboard/urls/:shortId/analytics", middleware.RequireAuth(), dashboard.GetURLAnalytics)
	api.Get("/dashboard/urls/personal", middleware.RequireAuth(), dashboard.GetPersonalURLs)
	api.Get("/dashboard/urls/team", middleware.RequireAuth(), dashboard.GetTeamURLs)

	// Subscription info routes (with auth)
	api.Get("/subscription/info", middleware.RequireAuth(), subscription.HandleGetSubscriptionInfo)
	api.Get("/subscription/limit", middleware.RequireAuth(), subscription.HandleCheckURLLimit)

	// Analytics routes (with auth)
	api.Get("/analytics", middleware.RequireAuth(), analytics.GetUserAnalytics)
	api.Get("/analytics/urls", middleware.RequireAuth(), analytics.GetAllUserURLs)

	// URL management routes (with auth)
	api.Get("/urls", middleware.RequireAuth(), tracking.GetAllURLs)
	api.Delete("/urls/:id", middleware.RequireAuth(), urls.DeleteURL)
	api.Put("/urls/:id/toggle", middleware.RequireAuth(), urls.ToggleURLStatus)

	// Password protection routes
	api.Post("/urls/:id/lock", middleware.RequireAuth(), urls.SetURLPassword)
	api.Delete("/urls/:id/lock", middleware.RequireAuth(), urls.RemoveURLPassword)
	api.Get("/urls/:shortId/password-stats", middleware.RequireAuth(), urls.GetPasswordStats)

	// Public unlock endpoint (no auth required, but rate limited to prevent brute force)
	app.Post("/api/unlock/:shortId", middleware.AuthRateLimit(), urls.UnlockURL)

	// QR Code tracking (requires auth)
	api.Post("/qr/track", middleware.RequireAuth(), qr.TrackQRCodeGeneration)

	// API Key Management (requires JWT auth)
	api.Post("/api-keys", middleware.RequireAuth(), api_keys.CreateAPIKey)
	api.Get("/api-keys", middleware.RequireAuth(), api_keys.ListAPIKeys)
	api.Put("/api-keys/:id", middleware.RequireAuth(), api_keys.UpdateAPIKey)
	api.Delete("/api-keys/:id", middleware.RequireAuth(), api_keys.DeleteAPIKey)

	// Webhook Management (requires JWT auth)
	api.Post("/webhooks", middleware.RequireAuth(), webhooks.CreateWebhook)
	api.Get("/webhooks", middleware.RequireAuth(), webhooks.ListWebhooks)
	api.Put("/webhooks/:id", middleware.RequireAuth(), webhooks.UpdateWebhook)
	api.Delete("/webhooks/:id", middleware.RequireAuth(), webhooks.DeleteWebhook)
	api.Put("/webhooks/:id/toggle", middleware.RequireAuth(), webhooks.ToggleWebhook)

	// Team Management (requires JWT auth - Premium only)
	api.Post("/teams", middleware.RequireAuth(), teams.CreateTeam)
	api.Get("/teams/my-team", middleware.RequireAuth(), teams.GetMyTeam)
	api.Post("/teams/members", middleware.RequireAuth(), teams.AddTeamMemberHandler)
	api.Delete("/teams/members/:memberID", middleware.RequireAuth(), teams.RemoveTeamMemberHandler)
	api.Delete("/teams", middleware.RequireAuth(), teams.DeleteTeam)

	// Team URL Management (requires JWT auth - Premium only, owner only)
	api.Post("/teams/urls/:urlID/assign", middleware.RequireAuth(), teams.AssignURLToTeam)
	api.Delete("/teams/urls/:urlID/unassign", middleware.RequireAuth(), teams.UnassignURLFromTeam)

	// Admin routes for abuse management (requires JWT auth + admin role)
	api.Post("/admin/block", middleware.RequireAuth(), middleware.AdminAuditLog(), admin.BlockURL)
	api.Delete("/admin/block/:shortId", middleware.RequireAuth(), middleware.AdminAuditLog(), admin.UnblockURL)
	api.Get("/admin/blocked", middleware.RequireAuth(), middleware.AdminAuditLog(), admin.ListBlockedURLs)
	api.Get("/admin/blocked/:shortId", middleware.RequireAuth(), admin.IsURLBlocked)

	// Admin URL management (requires JWT auth + admin role)
	api.Get("/admin/urls", middleware.RequireAuth(), middleware.AdminAuditLog(), admin.AdminListAllURLs)
	api.Delete("/admin/urls/:id", middleware.RequireAuth(), middleware.AdminAuditLog(), admin.AdminHardDeleteURL)

	// Admin domain blocking (requires JWT auth + admin role)
	api.Get("/admin/domains", middleware.RequireAuth(), middleware.AdminAuditLog(), admin.AdminListBlockedDomains)
	api.Post("/admin/domains", middleware.RequireAuth(), middleware.AdminAuditLog(), admin.AdminAddBlockedDomain)
	api.Delete("/admin/domains/:id", middleware.RequireAuth(), middleware.AdminAuditLog(), admin.AdminRemoveBlockedDomain)

	// Admin user management (requires JWT auth + admin role)
	api.Get("/admin/users", middleware.RequireAuth(), middleware.AdminAuditLog(), admin.AdminListUsers)
	api.Post("/admin/users/:id/ban", middleware.RequireAuth(), middleware.AdminAuditLog(), admin.AdminBanUser)
	api.Post("/admin/users/:id/unban", middleware.RequireAuth(), middleware.AdminAuditLog(), admin.AdminUnbanUser)

	// Public API (v1) - Requires API Key + Rate Limiting
	apiV1 := app.Group("/api/v1", middleware.RequireAPIKey(), middleware.APIRateLimit())

	// URL operations with permission checks
	apiV1.Post("/urls", middleware.RequirePermission("urls:write"), urlService.ShortenURL)
	apiV1.Get("/urls", middleware.RequirePermission("urls:read"), urls.GetUserURLs)
	apiV1.Get("/urls/:shortId", middleware.RequirePermission("urls:read"), urls.GetURLDetails)
	apiV1.Delete("/urls/:shortId", middleware.RequirePermission("urls:write"), urls.DeleteURL)
	apiV1.Put("/urls/:shortId", middleware.RequirePermission("urls:write"), urls.UpdateURL)

	// Analytics operations with permission checks
	apiV1.Get("/analytics/urls/:shortId", middleware.RequirePermission("analytics:read"), dashboard.GetURLAnalytics)
	apiV1.Get("/analytics/overview", middleware.RequirePermission("analytics:read"), dashboard.GetDashboardStats)
	apiV1.Get("/analytics/clicks", middleware.RequirePermission("analytics:read"), dashboard.GetRecentClicks)

	// Webhook operations with permission checks (API key-based)
	apiV1.Get("/webhooks", middleware.RequirePermission("webhooks:read"), webhooks.ListWebhooks)
	apiV1.Post("/webhooks", middleware.RequirePermission("webhooks:write"), webhooks.CreateWebhook)
	apiV1.Put("/webhooks/:id", middleware.RequirePermission("webhooks:write"), webhooks.UpdateWebhook)
	apiV1.Delete("/webhooks/:id", middleware.RequirePermission("webhooks:write"), webhooks.DeleteWebhook)
	apiV1.Put("/webhooks/:id/toggle", middleware.RequirePermission("webhooks:write"), webhooks.ToggleWebhook)

	// Account info (no special permission needed, just valid API key)
	apiV1.Get("/account", getAPIKeyAccount)
}

func main() {
	// Try to load .env file, but don't warn if it doesn't exist
	// (in production, environment variables are provided by Docker Compose)
	_ = godotenv.Load()

	// Initialize database
	if err := database.InitPostgres(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.ClosePostgres()

	// Start background cleanup jobs
	jobs.StartCleanupScheduler()

	// Start subscription renewal cron
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	subscription.StartRenewalCron(ctx)

	app := fiber.New(fiber.Config{
		EnableTrustedProxyCheck: true,
		TrustedProxies:          []string{"172.18.0.0/16", "127.0.0.1"}, // Docker network and localhost
		ProxyHeader:             fiber.HeaderXForwardedFor,
	})
	app.Use(logger.New())

	// CORS configuration - PRODUCTION (no localhost)
	app.Use(cors.New(cors.Config{
		AllowOrigins: "https://app.orangeurl.live,https://orangeurl.live",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
		AllowCredentials: true,
	}))

	setupRoutes(app)

	addr := os.Getenv("APP_PORT")
	if addr == "" {
		addr = ":3000"
	}

	log.Printf("Server starting on port %s", addr)
	log.Fatal(app.Listen(addr))
}
