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

	// CORS configuration
	app.Use(cors.New(cors.Config{
		AllowOrigins: "https://app.orangeurl.live,https://orangeurl.live,http://localhost:3001,http://localhost:3000",
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
