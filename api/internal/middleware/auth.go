package middleware

import (
	"fmt"
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
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authorization header required"})
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Bearer token required"})
		}

		// Verify JWT token
		claims, err := verifyClerkJWT(tokenString)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token"})
		}

		// Get user from database - only allow registered users
		queries := database.GetQueries()
		user, err := queries.GetUserByClerkID(c.Context(), claims.Subject)
		if err != nil {
			// User is not registered in our database
			// This means they signed in but weren't registered via sign-up webhook
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error":   "Access denied",
				"message": "Please sign up first to access this service",
				"code":    "USER_NOT_REGISTERED",
			})
		}

		// Store user in context
		c.Locals("user", user)
		c.Locals("user_id", user.ID)
		c.Locals("clerk_id", user.ClerkID)

		return c.Next()
	}
}

func verifyClerkJWT(tokenString string) (*jwt.RegisteredClaims, error) {
	// Parse token without verification first to get the kid
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return nil, nil // We'll verify later
	})

	if err != nil {
		return nil, err
	}

	// Get the kid from token header
	kid, ok := token.Header["kid"].(string)
	if !ok {
		return nil, fmt.Errorf("kid not found in token header")
	}

	// Get Clerk's public key
	publicKey, err := getClerkPublicKey(kid)
	if err != nil {
		return nil, err
	}

	// Now verify the token with the correct public key
	token, err = jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return publicKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*jwt.RegisteredClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
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
	user, ok := c.Locals("user").(*database.User)
	if !ok {
		return nil, fmt.Errorf("user not found in context")
	}
	return user, nil
}
