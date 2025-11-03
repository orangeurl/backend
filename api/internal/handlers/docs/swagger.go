package docs

import (
	"github.com/gofiber/fiber/v2"
)

// ServeSwaggerUI serves the Swagger UI and OpenAPI spec
func ServeSwaggerUI(c *fiber.Ctx) error {
	return c.SendFile("./internal/handlers/docs/static/index.html")
}

// ServeOpenAPISpec serves the OpenAPI YAML specification
func ServeOpenAPISpec(c *fiber.Ctx) error {
	c.Set("Content-Type", "application/yaml")
	return c.SendFile("./docs/openapi.yaml")
}
