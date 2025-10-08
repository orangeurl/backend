package routes

import (
	"os"
	"path/filepath"

	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/xenonnn4w/orangeurl/internal/database"
)

func ResolveURL(c *fiber.Ctx) error {
	url := c.Params("url")

	r := database.CreateClient(0)
	defer r.Close()

	value, err := r.Get(database.Ctx, url).Result()
	if err == redis.Nil {
		// Serve HTML page for invalid links
		html, err := os.ReadFile(filepath.Join("templates", "404.html"))
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "short not found in the database"})
		}
		c.Set("Content-Type", "text/html")
		return c.Status(fiber.StatusNotFound).SendString(string(html))
	} else if err != nil {
		// For database connection errors, also serve HTML
		html, err := os.ReadFile(filepath.Join("templates", "500.html"))
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "cannot connect to database"})
		}
		c.Set("Content-Type", "text/html")
		return c.Status(fiber.StatusInternalServerError).SendString(string(html))
	}

	rInr := database.CreateClient(1)
	defer rInr.Close()

	_ = rInr.Incr(database.Ctx, "counter")

	return c.Redirect(value, 301)
}
