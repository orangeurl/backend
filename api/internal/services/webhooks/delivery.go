package webhooks

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xenonnn4w/orangeurl/internal/database"
	"golang.org/x/net/context"
)

// WebhookEvent represents a webhook event payload
type WebhookEvent struct {
	Event     string                 `json:"event"`
	Timestamp string                 `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// TriggerWebhook sends webhook events to subscribed endpoints for a specific user
func TriggerWebhook(eventType string, userID uuid.UUID, data map[string]interface{}) {
	go func() {
		ctx := context.Background()
		queries := database.GetQueries()

		// Check webhook delivery rate limit
		r := database.CreateClient(0)
		defer r.Close()

		rateLimitKey := fmt.Sprintf("webhook_delivery_ratelimit:%s", userID)

		// Get current count in last hour
		currentCount, err := r.Get(ctx, rateLimitKey).Int64()
		if err != nil && err.Error() != "redis: nil" {
			log.Printf("[Webhook] Rate limit check failed: %v", err)
		}

		// Get user's subscription tier to determine rate limit
		user, err := queries.GetUserByID(ctx, userID)
		var maxDeliveriesPerHour int64
		if err == nil {
			switch user.SubscriptionTier {
			case "pro":
				maxDeliveriesPerHour = 10000
			case "premium":
				maxDeliveriesPerHour = 50000
			default:
				maxDeliveriesPerHour = 1000
			}
		} else {
			maxDeliveriesPerHour = 1000 // Default to free tier limit
		}

		if currentCount >= maxDeliveriesPerHour {
			log.Printf("[Webhook] Rate limit exceeded for user %s: %d/%d", userID, currentCount, maxDeliveriesPerHour)
			return
		}

		// Increment counter
		pipe := r.Pipeline()
		pipe.Incr(ctx, rateLimitKey)
		pipe.Expire(ctx, rateLimitKey, 1*time.Hour)
		_, err = pipe.Exec(ctx)
		if err != nil {
			log.Printf("[Webhook] Failed to update rate limit counter: %v", err)
		}

		// Get all active webhooks subscribed to this event for the specific user
		webhooks, err := queries.ListActiveWebhooksByEventAndUser(ctx, database.ListActiveWebhooksByEventAndUserParams{
			Events: []string{eventType},
			UserID: userID,
		})
		if err != nil {
			log.Printf("[Webhook] Failed to fetch webhooks for event %s and user %s: %v", eventType, userID, err)
			return
		}

		if len(webhooks) == 0 {
			log.Printf("[Webhook] No active webhooks for event %s and user %s", eventType, userID)
			return
		}

		log.Printf("[Webhook] Triggering %d webhooks for event %s and user %s", len(webhooks), eventType, userID)

		// Create webhook event payload
		event := WebhookEvent{
			Event:     eventType,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Data:      data,
		}

		payloadBytes, err := json.Marshal(event)
		if err != nil {
			log.Printf("[Webhook] Failed to marshal event payload: %v", err)
			return
		}

		// Send to each webhook endpoint
		for _, webhook := range webhooks {
			// Create delivery record
			delivery, err := queries.CreateWebhookDelivery(ctx, database.CreateWebhookDeliveryParams{
				WebhookID: webhook.ID,
				EventType: eventType,
				Payload:   payloadBytes,
				Status:    "pending",
			})
			if err != nil {
				log.Printf("[Webhook] Failed to create delivery record: %v", err)
				continue
			}

			// Attempt delivery
			go deliverWebhook(webhook, delivery.ID, payloadBytes)
		}
	}()
}

// deliverWebhook attempts to deliver a webhook with retries
func deliverWebhook(webhook database.Webhook, deliveryID uuid.UUID, payload []byte) {
	ctx := context.Background()
	queries := database.GetQueries()

	maxAttempts := 3
	backoffDurations := []time.Duration{
		0,              // Immediate
		5 * time.Second,
		30 * time.Second,
	}

	var lastErr error
	var responseCode int
	var responseBody string

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			log.Printf("[Webhook] Retrying delivery %s (attempt %d/%d)", deliveryID, attempt+1, maxAttempts)
			time.Sleep(backoffDurations[attempt])
		}

		// Re-validate webhook URL before each attempt to prevent DNS rebinding attacks
		if err := validateWebhookURL(webhook.Url); err != nil {
			lastErr = fmt.Errorf("URL validation failed: %w", err)
			log.Printf("[Webhook] Delivery %s blocked: %v", deliveryID, lastErr)
			break // Don't retry if URL becomes invalid
		}

		// Generate timestamp for replay attack protection
		timestamp := time.Now().Unix()
		timestampStr := fmt.Sprintf("%d", timestamp)

		// Create signed payload: timestamp + "." + payload
		signedContent := timestampStr + "." + string(payload)

		// Calculate HMAC signature over timestamped payload
		signature := calculateSignature([]byte(signedContent), webhook.Secret)

		// Create HTTP request
		req, err := http.NewRequest("POST", webhook.Url, bytes.NewBuffer(payload))
		if err != nil {
			lastErr = err
			log.Printf("[Webhook] Failed to create request for delivery %s: %v", deliveryID, err)
			continue
		}

		// Set headers
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "OrangeURL-Webhooks/1.0")
		req.Header.Set("X-OrangeURL-Signature", signature)
		req.Header.Set("X-OrangeURL-Timestamp", timestampStr)
		req.Header.Set("X-OrangeURL-Signature-Version", "2")
		req.Header.Set("X-OrangeURL-Delivery-ID", deliveryID.String())
		req.Header.Set("X-OrangeURL-Event", webhook.Events[0]) // Event type

		// Send request with timeout
		client := &http.Client{
			Timeout: 10 * time.Second,
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			log.Printf("[Webhook] Delivery %s failed (attempt %d): %v", deliveryID, attempt+1, err)
			continue
		}
		defer resp.Body.Close()

		// Read response
		bodyBytes, _ := io.ReadAll(resp.Body)
		responseCode = resp.StatusCode
		responseBody = string(bodyBytes)

		// Check if successful (2xx status code)
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			// Success!
			log.Printf("[Webhook] Delivery %s succeeded (status: %d)", deliveryID, resp.StatusCode)

			if err := queries.UpdateWebhookDelivery(ctx, database.UpdateWebhookDeliveryParams{
				ID:           deliveryID,
				Status:       "success",
				Attempts:     sql.NullInt32{Int32: int32(attempt + 1), Valid: true},
				ResponseCode: database.NewNullInt32(int32(responseCode)),
				ResponseBody: database.NewNullString(truncateString(responseBody, 500)),
			}); err != nil {
				log.Printf("[Webhook] Failed to update delivery record: %v", err)
			}
			return
		}

		lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateString(responseBody, 200))
		log.Printf("[Webhook] Delivery %s received non-2xx status (attempt %d): %d", deliveryID, attempt+1, resp.StatusCode)
	}

	// All attempts failed
	log.Printf("[Webhook] Delivery %s failed after %d attempts: %v", deliveryID, maxAttempts, lastErr)

	if err := queries.UpdateWebhookDelivery(ctx, database.UpdateWebhookDeliveryParams{
		ID:           deliveryID,
		Status:       "failed",
		Attempts:     sql.NullInt32{Int32: int32(maxAttempts), Valid: true},
		ResponseCode: database.NewNullInt32(int32(responseCode)),
		ResponseBody: database.NewNullString(truncateString(responseBody, 500)),
	}); err != nil {
		log.Printf("[Webhook] Failed to update delivery record: %v", err)
	}
}

// calculateSignature generates HMAC-SHA256 signature for webhook payload
func calculateSignature(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// truncateString truncates a string to maxLength
func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength] + "..."
}

// validateWebhookURL validates a webhook URL to prevent SSRF and DNS rebinding attacks
func validateWebhookURL(rawURL string) error {
	// Parse URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Only allow HTTP and HTTPS schemes
	scheme := strings.ToLower(parsedURL.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("only HTTP and HTTPS protocols are allowed")
	}

	// Extract hostname
	hostname := parsedURL.Hostname()
	if hostname == "" {
		return fmt.Errorf("URL must contain a hostname")
	}

	// Resolve hostname to IP addresses
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return fmt.Errorf("failed to resolve hostname: %w", err)
	}

	if len(ips) == 0 {
		return fmt.Errorf("hostname does not resolve to any IP address")
	}

	// Check each resolved IP for private/internal addresses
	for _, ip := range ips {
		if err := validateIP(ip); err != nil {
			return err
		}
	}

	// Check for cloud metadata endpoints
	if isMetadataEndpoint(hostname) {
		return fmt.Errorf("requests to cloud metadata endpoints are not allowed")
	}

	return nil
}

// validateIP checks if an IP address is private, loopback, or otherwise restricted
func validateIP(ip net.IP) error {
	// Reject loopback addresses
	if ip.IsLoopback() {
		return fmt.Errorf("loopback addresses are not allowed")
	}

	// Reject private IP ranges
	if ip.IsPrivate() {
		return fmt.Errorf("private IP addresses are not allowed")
	}

	// Reject link-local addresses
	if ip.IsLinkLocalUnicast() {
		return fmt.Errorf("link-local addresses are not allowed")
	}

	// Reject multicast addresses
	if ip.IsMulticast() {
		return fmt.Errorf("multicast addresses are not allowed")
	}

	// Reject unspecified addresses
	if ip.IsUnspecified() {
		return fmt.Errorf("unspecified addresses are not allowed")
	}

	return nil
}

// isMetadataEndpoint checks for well-known cloud metadata endpoints
func isMetadataEndpoint(hostname string) bool {
	hostname = strings.ToLower(hostname)
	metadataEndpoints := []string{
		"169.254.169.254",
		"metadata.google.internal",
		"metadata",
		"instance-data",
		"fd00:ec2::254",
	}

	for _, endpoint := range metadataEndpoints {
		if hostname == endpoint {
			return true
		}
	}

	return false
}
