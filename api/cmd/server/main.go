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
	"github.com/xenonnn4w/orangeurl/internal/handlers/auth"
	"github.com/xenonnn4w/orangeurl/internal/handlers/waitlist"
	"github.com/xenonnn4w/orangeurl/internal/middleware"
	"github.com/xenonnn4w/orangeurl/internal/routes"
	"github.com/xenonnn4w/orangeurl/internal/services/tracking"
	"github.com/xenonnn4w/orangeurl/internal/services/url"
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
	app.Get("/:url", routes.ResolveURL)
	app.Post("/api/v1", url.ShortenURL)

	// Webhook routes (public - no auth required)
	app.Post("/api/webhooks/clerk", auth.HandleClerkWebhook)

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
	api.Use(middleware.RequireAuth()) // Add auth middleware to all /api routes

	api.Get("/urls", tracking.GetAllURLs)

	api.Get("/dashboard/stats", func(c *fiber.Ctx) error {
		user, err := middleware.GetUserFromContext(c)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "User not found"})
		}
		return c.JSON(fiber.Map{
			"message": "Dashboard stats",
			"user_id": user.ID,
			"email":   user.Email,
		})
	})
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
