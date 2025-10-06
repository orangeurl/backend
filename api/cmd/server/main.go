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
	"github.com/xenonnn4w/orangeurl/internal/middleware"
	"github.com/xenonnn4w/orangeurl/internal/routes"
	"github.com/xenonnn4w/orangeurl/internal/services/tracking"
	"github.com/xenonnn4w/orangeurl/internal/services/url"
)

func setupRoutes(app *fiber.App) {
	// Public routes
	app.Get("/:url", routes.ResolveURL)
	app.Post("/api/v1", url.ShortenURL)

	// Webhook routes
	app.Post("/api/webhooks/clerk", auth.HandleClerkWebhook)

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
