package admin

import (
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/xenonnn4w/orangeurl/internal/database"
	"github.com/xenonnn4w/orangeurl/internal/middleware"
)

type BlockDomainRequest struct {
	Domain            string `json:"domain"`
	IncludeSubdomains bool   `json:"include_subdomains"`
	BlockReason       string `json:"block_reason"`
}

// AdminAddBlockedDomain adds a domain to the blocked list
func AdminAddBlockedDomain(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}
	if !user.IsAdmin.Valid || !user.IsAdmin.Bool {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin access required"})
	}

	var req BlockDomainRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Normalize domain
	req.Domain = strings.ToLower(strings.TrimSpace(req.Domain))
	req.Domain = strings.TrimPrefix(req.Domain, "http://")
	req.Domain = strings.TrimPrefix(req.Domain, "https://")
	req.Domain = strings.TrimPrefix(req.Domain, "www.")
	req.Domain = strings.TrimRight(req.Domain, "/")

	if req.Domain == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "domain is required"})
	}

	if req.BlockReason == "" {
		req.BlockReason = "Blocked by admin"
	}

	queries := database.GetQueries()
	blocked, err := queries.AddBlockedDomain(c.Context(), database.BlockDomainParams{
		Domain:            req.Domain,
		IncludeSubdomains: req.IncludeSubdomains,
		BlockReason:       req.BlockReason,
		BlockedBy:         uuid.NullUUID{UUID: user.ID, Valid: true},
	})
	if err != nil {
		log.Printf("[AdminAddBlockedDomain] Error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to block domain"})
	}

	log.Printf("[AdminAddBlockedDomain] Admin %s blocked domain '%s' (subdomains: %v)", user.Email, req.Domain, req.IncludeSubdomains)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message":            "Domain blocked successfully",
		"id":                 blocked.ID,
		"domain":             blocked.Domain,
		"include_subdomains": blocked.IncludeSubdomains,
		"block_reason":       blocked.BlockReason,
		"created_at":         blocked.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// AdminRemoveBlockedDomain removes a domain from the blocked list
func AdminRemoveBlockedDomain(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}
	if !user.IsAdmin.Valid || !user.IsAdmin.Bool {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin access required"})
	}

	domainID := c.Params("id")
	if domainID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Domain ID required"})
	}

	parsedID, err := uuid.Parse(domainID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid domain ID"})
	}

	queries := database.GetQueries()
	if err := queries.RemoveBlockedDomain(c.Context(), parsedID); err != nil {
		log.Printf("[AdminRemoveBlockedDomain] Error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to unblock domain"})
	}

	log.Printf("[AdminRemoveBlockedDomain] Admin %s unblocked domain ID: %s", user.Email, domainID)

	return c.JSON(fiber.Map{
		"message": "Domain unblocked successfully",
		"id":      domainID,
	})
}

// AdminListBlockedDomains returns all blocked domains
func AdminListBlockedDomains(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}
	if !user.IsAdmin.Valid || !user.IsAdmin.Bool {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin access required"})
	}

	queries := database.GetQueries()
	domains, err := queries.ListBlockedDomains(c.Context())
	if err != nil {
		log.Printf("[AdminListBlockedDomains] Error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch blocked domains"})
	}

	result := make([]fiber.Map, 0, len(domains))
	for _, d := range domains {
		item := fiber.Map{
			"id":                 d.ID,
			"domain":             d.Domain,
			"include_subdomains": d.IncludeSubdomains,
			"block_reason":       d.BlockReason,
			"created_at":         d.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		}
		if d.BlockedBy.Valid {
			item["blocked_by"] = d.BlockedBy.UUID.String()
		}
		result = append(result, item)
	}

	return c.JSON(fiber.Map{
		"blocked_domains": result,
		"count":           len(result),
	})
}
