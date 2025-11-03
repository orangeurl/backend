package middleware

import (
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
)

// AuditLog logs admin actions to a structured audit trail
type AuditLog struct {
	Timestamp string `json:"timestamp"`
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	Action    string `json:"action"`
	Resource  string `json:"resource"`
	IPAddress string `json:"ip_address"`
	UserAgent string `json:"user_agent"`
	Status    int    `json:"status"`
	Details   string `json:"details,omitempty"`
}

// AdminAuditLog middleware logs all admin actions
// This should be applied to all admin-only routes
func AdminAuditLog() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get user from context (requires RequireAuth middleware first)
		user, err := GetUserFromContext(c)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
		}

		// Verify user is admin
		if !user.IsAdmin.Valid || !user.IsAdmin.Bool {
			log.Printf("[Audit] ⚠️  Non-admin user %s attempted to access admin endpoint: %s %s",
				user.Email, c.Method(), c.Path())
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Forbidden - Admin access required",
			})
		}

		// Record start time
		startTime := time.Now()

		// Execute the request
		err = c.Next()

		// Log the admin action
		audit := AuditLog{
			Timestamp: startTime.Format(time.RFC3339),
			UserID:    user.ID.String(),
			Email:     user.Email,
			Action:    c.Method(),
			Resource:  c.Path(),
			IPAddress: c.IP(),
			UserAgent: c.Get("User-Agent"),
			Status:    c.Response().StatusCode(),
			Details:   fmt.Sprintf("Duration: %dms", time.Since(startTime).Milliseconds()),
		}

		// Structured logging for audit trail
		// In production, this should be sent to a secure audit log service
		log.Printf("[AUDIT] Admin Action: user=%s email=%s action=%s resource=%s ip=%s status=%d duration=%dms",
			audit.UserID,
			audit.Email,
			audit.Action,
			audit.Resource,
			audit.IPAddress,
			audit.Status,
			time.Since(startTime).Milliseconds(),
		)

		// TODO: In production, send to dedicated audit log storage:
		// - AWS CloudWatch Logs
		// - Elasticsearch/Kibana
		// - Splunk
		// - Datadog
		// - Or dedicated database table for compliance requirements

		return err
	}
}

// RequireAdmin is a combined middleware that enforces admin access and logs actions
func RequireAdmin() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get user from context (requires RequireAuth middleware first)
		user, err := GetUserFromContext(c)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
		}

		// Verify user is admin
		if !user.IsAdmin.Valid || !user.IsAdmin.Bool {
			log.Printf("[Auth] ⚠️  Non-admin user %s attempted to access admin endpoint: %s %s",
				user.Email, c.Method(), c.Path())
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Forbidden - Admin access required",
			})
		}

		return c.Next()
	}
}
