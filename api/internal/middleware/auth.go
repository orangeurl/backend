package middleware

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

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

// JWKS cache
var (
	jwksCache      *ClerkJWKS
	jwksCacheMutex sync.RWMutex
	jwksCacheTime  time.Time
	jwksCacheTTL   = 1 * time.Hour
)

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
	// Parse the token to get the kid (key ID) from the header
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// Get the kid from token header
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, fmt.Errorf("kid not found in token header")
		}

		// Get the public key for this kid
		publicKey, err := getClerkPublicKey(kid)
		if err != nil {
			return nil, fmt.Errorf("failed to get public key: %w", err)
		}

		return publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to verify token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("token is invalid")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("failed to parse claims")
	}

	// Convert MapClaims to RegisteredClaims
	registeredClaims := &jwt.RegisteredClaims{}

	if sub, ok := claims["sub"].(string); ok {
		registeredClaims.Subject = sub
	} else {
		return nil, fmt.Errorf("token missing subject (clerk user ID)")
	}

	if exp, ok := claims["exp"].(float64); ok {
		registeredClaims.ExpiresAt = jwt.NewNumericDate(time.Unix(int64(exp), 0))
	}

	if iat, ok := claims["iat"].(float64); ok {
		registeredClaims.IssuedAt = jwt.NewNumericDate(time.Unix(int64(iat), 0))
	}

	if nbf, ok := claims["nbf"].(float64); ok {
		registeredClaims.NotBefore = jwt.NewNumericDate(time.Unix(int64(nbf), 0))
	}

	// Validate expiration
	if registeredClaims.ExpiresAt != nil && time.Now().After(registeredClaims.ExpiresAt.Time) {
		return nil, fmt.Errorf("token has expired")
	}

	return registeredClaims, nil
}

func getClerkPublicKey(kid string) (*rsa.PublicKey, error) {
	// Get JWKS from cache or fetch new
	jwks, err := getJWKS()
	if err != nil {
		return nil, fmt.Errorf("failed to get JWKS: %w", err)
	}

	// Find the key with matching kid
	for _, key := range jwks.Keys {
		if key.Kid == kid {
			return jwkToPublicKey(&key)
		}
	}

	return nil, fmt.Errorf("key with kid %s not found in JWKS", kid)
}

func getJWKS() (*ClerkJWKS, error) {
	jwksCacheMutex.RLock()
	if jwksCache != nil && time.Since(jwksCacheTime) < jwksCacheTTL {
		jwksCacheMutex.RUnlock()
		return jwksCache, nil
	}
	jwksCacheMutex.RUnlock()

	// Fetch JWKS from Clerk
	jwksCacheMutex.Lock()
	defer jwksCacheMutex.Unlock()

	// Double-check after acquiring write lock
	if jwksCache != nil && time.Since(jwksCacheTime) < jwksCacheTTL {
		return jwksCache, nil
	}

	// Get Clerk publishable key to construct JWKS URL
	clerkPublishableKey := os.Getenv("CLERK_PUBLISHABLE_KEY")
	if clerkPublishableKey == "" {
		return nil, fmt.Errorf("CLERK_PUBLISHABLE_KEY not set")
	}

	// Extract frontend API from publishable key (format: pk_test_xxxxx or pk_live_xxxxx)
	// The JWKS URL is: https://[clerk-frontend-api]/.well-known/jwks.json
	var jwksURL string
	if strings.HasPrefix(clerkPublishableKey, "pk_test_") {
		// Test environment
		jwksURL = "https://clerk.orangeurl.live/.well-known/jwks.json"
	} else if strings.HasPrefix(clerkPublishableKey, "pk_live_") {
		// Production environment
		jwksURL = "https://clerk.orangeurl.live/.well-known/jwks.json"
	} else {
		return nil, fmt.Errorf("invalid CLERK_PUBLISHABLE_KEY format")
	}

	// Allow override via environment variable
	if customJWKSURL := os.Getenv("CLERK_JWKS_URL"); customJWKSURL != "" {
		jwksURL = customJWKSURL
	}

	log.Printf("[JWKS] Fetching from: %s", jwksURL)

	resp, err := http.Get(jwksURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	var jwks ClerkJWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("failed to decode JWKS: %w", err)
	}

	if len(jwks.Keys) == 0 {
		return nil, fmt.Errorf("JWKS contains no keys")
	}

	jwksCache = &jwks
	jwksCacheTime = time.Now()

	log.Printf("[JWKS] Successfully cached %d keys", len(jwks.Keys))
	return jwksCache, nil
}

func jwkToPublicKey(jwk *ClerkJWK) (*rsa.PublicKey, error) {
	// Decode the modulus (N)
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %w", err)
	}

	// Decode the exponent (E)
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exponent: %w", err)
	}

	// Convert bytes to big.Int
	n := new(big.Int).SetBytes(nBytes)

	// Convert exponent bytes to int
	e := 0
	for _, b := range eBytes {
		e = e*256 + int(b)
	}

	return &rsa.PublicKey{
		N: n,
		E: e,
	}, nil
}

func RequireAuth() fiber.Handler {
	return ClerkAuthMiddleware()
}

// OptionalAuth is middleware that authenticates if token is present, but doesn't fail if not
func OptionalAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		
		// No auth header? Just continue without setting user
		if authHeader == "" {
			return c.Next()
		}

		// Has auth header, try to verify
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader {
			// No Bearer prefix, skip auth
			return c.Next()
		}

		claims, err := verifyClerkJWT(token)
		if err != nil {
			// Invalid token, but don't fail - just continue without user
			log.Printf("[OptionalAuth] Token verification failed: %v", err)
			return c.Next()
		}

		queries := database.GetQueries()
		if queries == nil {
			// Database not ready, continue without user
			return c.Next()
		}

		user, err := queries.GetUserByClerkID(c.Context(), claims.Subject)
		if err != nil {
			// User not found, continue without user
			return c.Next()
		}

		// Store user in context
		c.Locals("user", &user)
		c.Locals("user_id", user.ID)
		c.Locals("clerk_id", user.ClerkID)

		return c.Next()
	}
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

