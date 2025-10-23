package middleware

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strings"
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

func ClerkAuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			fmt.Println("❌ [Auth] No Authorization header")
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authorization header required"})
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			fmt.Println("❌ [Auth] No Bearer prefix in token")
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Bearer token required"})
		}

		fmt.Printf("🔐 [Auth] Token received (length: %d)\n", len(tokenString))

		// Verify JWT token
		claims, err := verifyClerkJWT(tokenString)
		if err != nil {
			fmt.Printf("❌ [Auth] Token verification failed: %v\n", err)
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid token",
				"details": err.Error(),
			})
		}

		fmt.Printf("✅ [Auth] Token verified for subject: %s\n", claims.Subject)

		// Get user from database - only allow registered users
		queries := database.GetQueries()
		user, err := queries.GetUserByClerkID(c.Context(), claims.Subject)
		if err != nil {
			// User is not registered in our database
			fmt.Printf("❌ [Auth] User not found in database: %s\n", claims.Subject)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error":   "Access denied",
				"message": "Please sign up first to access this service",
				"code":    "USER_NOT_REGISTERED",
			})
		}

		fmt.Printf("✅ [Auth] User found: %s (%s)\n", user.Email, user.ID)

		// Store user in context
		c.Locals("user", user)
		c.Locals("user_id", user.ID)
		c.Locals("clerk_id", user.ClerkID)

		return c.Next()
	}
}

var jwksCache *ClerkJWKS
var jwksCacheTime time.Time

func verifyClerkJWT(tokenString string) (*jwt.RegisteredClaims, error) {
	// Parse token to get header
	parser := jwt.NewParser()
	token, _, err := parser.ParseUnverified(tokenString, &jwt.RegisteredClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// Get the kid from token header
	kid, ok := token.Header["kid"].(string)
	if !ok {
		return nil, fmt.Errorf("kid not found in token header")
	}

	// Get Clerk's public key
	publicKey, err := getClerkPublicKey(kid)
	if err != nil {
		return nil, fmt.Errorf("failed to get public key: %w", err)
	}

	// Verify the token with the correct public key
	token, err = jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("token verification failed: %w", err)
	}

	if claims, ok := token.Claims.(*jwt.RegisteredClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token claims")
}

func getClerkPublicKey(kid string) (*rsa.PublicKey, error) {
	// Refresh JWKS cache if older than 1 hour or not cached
	if jwksCache == nil || time.Since(jwksCacheTime) > time.Hour {
		clerkDomain := os.Getenv("CLERK_DOMAIN")
		fmt.Printf("🔑 [Auth] CLERK_DOMAIN from env: %s\n", clerkDomain)
		
		if clerkDomain == "" {
			// Try to get from publishable key
			publishableKey := os.Getenv("CLERK_PUBLISHABLE_KEY")
			if publishableKey == "" {
				publishableKey = os.Getenv("NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY")
			}
			fmt.Printf("🔑 [Auth] CLERK_PUBLISHABLE_KEY from env: %s\n", publishableKey)
			
			if publishableKey != "" {
				// Extract domain from publishable key (format: pk_test_xxxxx or pk_live_xxxxx)
				// For Clerk, the JWKS endpoint is at the instance domain
				extracted := extractClerkDomain(publishableKey)
				if extracted != "" {
					clerkDomain = "https://clerk." + extracted
				}
			}
			
			if clerkDomain == "" {
				return nil, fmt.Errorf("CLERK_DOMAIN not set in environment variables")
			}
		}

		jwksURL := clerkDomain + "/.well-known/jwks.json"
		fmt.Printf("🌐 [Auth] Fetching JWKS from: %s\n", jwksURL)
		
		resp, err := http.Get(jwksURL)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch JWKS from %s: %w", jwksURL, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("JWKS endpoint %s returned status %d", jwksURL, resp.StatusCode)
		}

		var jwks ClerkJWKS
		if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
			return nil, fmt.Errorf("failed to decode JWKS: %w", err)
		}

		fmt.Printf("✅ [Auth] JWKS fetched successfully, %d keys found\n", len(jwks.Keys))
		jwksCache = &jwks
		jwksCacheTime = time.Now()
	} else {
		fmt.Printf("♻️  [Auth] Using cached JWKS (%d keys)\n", len(jwksCache.Keys))
	}

	// Find the key with matching kid
	fmt.Printf("🔍 [Auth] Looking for key with kid: %s\n", kid)
	for _, key := range jwksCache.Keys {
		if key.Kid == kid {
			fmt.Printf("✅ [Auth] Found matching key\n")
			return jwkToRSAPublicKey(key)
		}
	}

	return nil, fmt.Errorf("key with kid %s not found in JWKS", kid)
}

func jwkToRSAPublicKey(jwk ClerkJWK) (*rsa.PublicKey, error) {
	// Decode base64url-encoded modulus
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %w", err)
	}

	// Decode base64url-encoded exponent
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exponent: %w", err)
	}

	// Convert bytes to big.Int
	n := new(big.Int).SetBytes(nBytes)
	e := int(new(big.Int).SetBytes(eBytes).Int64())

	return &rsa.PublicKey{
		N: n,
		E: e,
	}, nil
}

func extractClerkDomain(publishableKey string) string {
	// Extract the instance from publishable key
	// Format: pk_test_xxx or pk_live_xxx
	// We can't reliably extract domain from the key alone
	// So we should use the CLERK_DOMAIN env var
	return ""
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
