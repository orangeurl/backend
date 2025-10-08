package routes

import (
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
		// Return 404 with HTML that redirects to broken-link page
		c.Status(404)
		return c.Type("html").SendString(`
			<!DOCTYPE html>
			<html>
			<head>
				<meta http-equiv="refresh" content="0;url=https://app.orangeurl.live/broken-link">
				<title>Link Not Found</title>
			</head>
			<body>
				<p>Redirecting to error page...</p>
				<script>window.location.href='https://app.orangeurl.live/broken-link';</script>
			</body>
			</html>
		`)
	} else if err != nil {
		// Return 500 with HTML that redirects for database errors
		c.Status(500)
		return c.Type("html").SendString(`
			<!DOCTYPE html>
			<html>
			<head>
				<meta http-equiv="refresh" content="0;url=https://app.orangeurl.live/broken-link">
				<title>Server Error</title>
			</head>
			<body>
				<p>Redirecting to error page...</p>
				<script>window.location.href='https://app.orangeurl.live/broken-link';</script>
			</body>
			</html>
		`)
	}

	rInr := database.CreateClient(1)
	defer rInr.Close()

	_ = rInr.Incr(database.Ctx, "counter")

	return c.Redirect(value, 301)
}

