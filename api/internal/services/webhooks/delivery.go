package webhooks

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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

// TriggerWebhook sends webhook events to all subscribed endpoints
func TriggerWebhook(eventType string, data map[string]interface{}) {
	go func() {
		ctx := context.Background()
		queries := database.GetQueries()

		// Get all active webhooks subscribed to this event
		webhooks, err := queries.ListActiveWebhooksByEvent(ctx, eventType)
		if err != nil {
			log.Printf("[Webhook] Failed to fetch webhooks for event %s: %v", eventType, err)
			return
		}

		if len(webhooks) == 0 {
			log.Printf("[Webhook] No active webhooks for event: %s", eventType)
			return
		}

		log.Printf("[Webhook] Triggering %d webhooks for event: %s", len(webhooks), eventType)

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

		// Calculate HMAC signature
		signature := calculateSignature(payload, webhook.Secret)

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
				Attempts:     int32(attempt + 1),
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
		Attempts:     int32(maxAttempts),
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
