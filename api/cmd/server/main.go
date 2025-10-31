package main

import (
	"log"
	"os"

	_ "github.com/lib/pq"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/joho/godotenv"
	"github.com/xenonnn4w/orangeurl/internal/database"
	"github.com/xenonnn4w/orangeurl/internal/handlers/analytics"
	"github.com/xenonnn4w/orangeurl/internal/handlers/auth"
	"github.com/xenonnn4w/orangeurl/internal/handlers/bio"
	"github.com/xenonnn4w/orangeurl/internal/handlers/dashboard"
	"github.com/xenonnn4w/orangeurl/internal/handlers/payment"
	"github.com/xenonnn4w/orangeurl/internal/handlers/urls"
	"github.com/xenonnn4w/orangeurl/internal/handlers/waitlist"
	"github.com/xenonnn4w/orangeurl/internal/middleware"
	"github.com/xenonnn4w/orangeurl/internal/routes"
	"github.com/xenonnn4w/orangeurl/internal/services/tracking"
	urlService "github.com/xenonnn4w/orangeurl/internal/services/url"
)

func setupRoutes(app *fiber.App) {
	// Health check
	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "OrangeURL API is running"})
	})

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "healthy"})
	})

	// Public routes
	app.Get("/:url", middleware.PasswordProtectionMiddleware, routes.ResolveURL)
	// URL shortening with optional auth (saves to user account if authenticated)
	app.Post("/api/v1", middleware.OptionalAuth(), urlService.ShortenURL)

	// Webhook routes (public - no auth required)
	app.Post("/api/webhooks/clerk", auth.HandleClerkWebhook)
	app.Post("/api/webhooks/dodo", payment.HandleDodoWebhook)

	// Waitlist route (public - no auth required)
	app.Post("/api/v1/api/waitlist", waitlist.JoinWaitlist)

	// Test endpoint for debugging (public for now)
	app.Get("/api/test/users", func(c *fiber.Ctx) error {
		queries := database.GetQueries()
		users, err := queries.ListUsers(c.Context())
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"users": users})
	})

	// Protected routes
	api := app.Group("/api")
	
	// Dashboard routes (with auth)
	api.Get("/dashboard/stats", middleware.RequireAuth(), dashboard.GetDashboardStats)
	api.Get("/dashboard/urls/:shortId/analytics", middleware.RequireAuth(), dashboard.GetURLAnalytics)

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

	// Public unlock endpoint (no auth required)
	app.Post("/api/unlock/:shortId", urls.UnlockURL)

	// Bio Pages Routes
	// Public routes (no auth required)
	api.Get("/bio/:username", bio.GetBioPage)                    // Get published bio page
	api.Post("/bio/:username/view", bio.TrackView)              // Track page view
	api.Post("/bio/:username/click", bio.TrackClick)            // Track link click
	api.Get("/bio/check/:username", bio.CheckUsernameAvailability) // Check username availability

	// Protected routes (require auth)
	api.Post("/bio", middleware.RequireAuth(), bio.CreateBioPage)                             // Create bio page
	api.Get("/dashboard/bio", middleware.RequireAuth(), bio.GetUserBioPages)                  // Get user's bio pages
	api.Get("/dashboard/bio/:username", middleware.RequireAuth(), bio.GetBioPageForEdit)      // Get bio page for editing
	api.Put("/bio/:username", middleware.RequireAuth(), bio.UpdateBioPage)                    // Update bio page
	api.Delete("/bio/:username", middleware.RequireAuth(), bio.DeleteBioPage)                 // Delete bio page
	api.Put("/bio/:username/publish", middleware.RequireAuth(), bio.PublishBioPage)           // Publish bio page
	api.Put("/bio/:username/unpublish", middleware.RequireAuth(), bio.UnpublishBioPage)       // Unpublish bio page
	api.Get("/dashboard/bio/:username/analytics", middleware.RequireAuth(), bio.GetAnalytics) // Get analytics
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Printf("Warning: .env file not found: %v", err)
	}

	// Initialize database
	if err := database.InitPostgres(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.ClosePostgres()

	app := fiber.New()
	app.Use(logger.New())
	app.Use(cors.New())

	setupRoutes(app)

	addr := os.Getenv("APP_PORT")
	if addr == "" {
		addr = ":3000"
	}

	log.Printf("Server starting on port %s", addr)
	log.Fatal(app.Listen(addr))
}
