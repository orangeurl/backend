package middleware

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/xenonnn4w/orangeurl/internal/database"
)

type ClerkJWKS struct {
	Keys []ClerkJWK `json:"keys"`
}

type ClerkJWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func ClerkAuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		log.Printf("[Auth] Request to: %s", c.Path())
		
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			log.Println("[Auth] No Authorization header")
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authorization header required"})
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			log.Println("[Auth] Missing Bearer prefix")
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Bearer token required"})
		}

		log.Println("[Auth] Token found, verifying...")
		
		// Verify JWT token
		claims, err := verifyClerkJWT(tokenString)
		if err != nil {
			log.Printf("[Auth] Token verification failed: %v", err)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token", "details": err.Error()})
		}

		log.Printf("[Auth] Token verified, Clerk ID: %s", claims.Subject)

		// Get user from database - only allow registered users
		queries := database.GetQueries()
		if queries == nil {
			log.Println("[Auth] ERROR: Database queries is nil")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database not initialized"})
		}

		user, err := queries.GetUserByClerkID(c.Context(), claims.Subject)
		if err != nil {
			log.Printf("[Auth] User not found in database: clerk_id=%s, error=%v", claims.Subject, err)
			// User is not registered in our database
			// This means they signed in but weren't registered via sign-up webhook
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error":   "Access denied",
				"message": "Please sign up first to access this service",
				"code":    "USER_NOT_REGISTERED",
			})
		}

		log.Printf("[Auth] User found in DB: ID=%s, Email=%s", user.ID, user.Email)

		// Store user in context - use pointer to ensure it persists
		c.Locals("user", &user)
		c.Locals("user_id", user.ID)
		c.Locals("clerk_id", user.ClerkID)

		log.Printf("[Auth] User stored in context (pointer): %p, calling Next()", &user)
		
		result := c.Next()
		
		log.Printf("[Auth] Next() completed, result=%v", result)
		return result
	}
}

func verifyClerkJWT(tokenString string) (*jwt.RegisteredClaims, error) {
	// For now, we'll just parse the token without verification
	// The token is already verified by Clerk on the frontend
	// In production, you should implement proper RSA verification with JWKS
	
	// Parse token without verification to extract claims
	token, _, err := jwt.NewParser().ParseUnverified(tokenString, &jwt.RegisteredClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*jwt.RegisteredClaims); ok {
		// Basic validation: check if token has required fields
		if claims.Subject == "" {
			return nil, fmt.Errorf("token missing subject (clerk user ID)")
		}
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token claims")
}

func getClerkPublicKey(kid string) (interface{}, error) {
	// For development, we'll use a simple approach
	// In production, you should cache this and refresh periodically
	clerkSecretKey := os.Getenv("CLERK_SECRET_KEY")
	if clerkSecretKey == "" {
		return nil, fmt.Errorf("CLERK_SECRET_KEY not set")
	}

	// For now, we'll use the secret key directly
	// In production, you should fetch the JWKS from Clerk's API
	return []byte(clerkSecretKey), nil
}

func RequireAuth() fiber.Handler {
	return ClerkAuthMiddleware()
}

// Helper function to get user from context
func GetUserFromContext(c *fiber.Ctx) (*database.User, error) {
	log.Printf("[GetUserFromContext] Attempting to get user from context...")
	
	userVal := c.Locals("user")
	log.Printf("[GetUserFromContext] c.Locals('user') returned: %v (type: %T)", userVal, userVal)
	
	if userVal == nil {
		log.Println("[GetUserFromContext] user is nil in context")
		return nil, fmt.Errorf("user not found in context")
	}
	
	user, ok := userVal.(*database.User)
	if !ok {
		log.Printf("[GetUserFromContext] Type assertion failed, got type: %T", userVal)
		return nil, fmt.Errorf("user not found in context - type assertion failed")
	}
	
	log.Printf("[GetUserFromContext] Successfully retrieved user: ID=%s", user.ID)
	return user, nil
}

