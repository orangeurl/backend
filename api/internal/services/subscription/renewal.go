package subscription

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/xenonnn4w/orangeurl/internal/database"
)

// PlanLimits defines URL creation limits per plan
type PlanLimits struct {
	Free    int
	Pro     int
	Premium int
}

var DefaultPlanLimits = PlanLimits{
	Free:    10,
	Pro:     100,
	Premium: 1000,
}

// GetURLLimitForPlan returns the monthly URL limit for a given plan
func GetURLLimitForPlan(plan string) int {
	switch plan {
	case "premium":
		return DefaultPlanLimits.Premium
	case "pro":
		return DefaultPlanLimits.Pro
	case "free":
		fallthrough
	default:
		return DefaultPlanLimits.Free
	}
}

// SubscriptionRenewalService handles subscription renewal logic
type SubscriptionRenewalService struct {
	queries *database.Queries
	redis   *redis.Client
}

// NewSubscriptionRenewalService creates a new renewal service
func NewSubscriptionRenewalService() *SubscriptionRenewalService {
	return &SubscriptionRenewalService{
		queries: database.GetQueries(),
		redis:   database.CreateClient(1), // Use client 1 for rate limiting
	}
}

// ProcessExpiredSubscriptions checks for expired subscriptions and processes them
func (s *SubscriptionRenewalService) ProcessExpiredSubscriptions(ctx context.Context) error {
	log.Println("🔄 [Renewal] Starting subscription renewal check...")

	// Get all subscriptions that have expired and are still active
	subscriptions, err := s.queries.GetSubscriptionsForRenewal(ctx)
	if err != nil {
		return fmt.Errorf("failed to get subscriptions for renewal: %w", err)
	}

	log.Printf("📊 [Renewal] Found %d subscriptions to process", len(subscriptions))

	for _, sub := range subscriptions {
		if err := s.processSubscription(ctx, sub); err != nil {
			log.Printf("❌ [Renewal] Error processing subscription %s: %v", sub.ID, err)
			continue
		}
	}

	log.Println("✅ [Renewal] Subscription renewal check completed")
	return nil
}

// processSubscription handles renewal for a single subscription
func (s *SubscriptionRenewalService) processSubscription(ctx context.Context, sub database.GetSubscriptionsForRenewalRow) error {
	log.Printf("🔍 [Renewal] Processing subscription for user %s (plan: %s)", sub.UserID, sub.PlanID)

	// Check if subscription should auto-renew (payment was successful via webhook)
	// If payment failed, the webhook would have already downgraded the user
	// Here we handle the case where period ended but no webhook was received

	// Get billing interval from subscription
	billingInterval := "monthly"
	if sub.BillingInterval.Valid {
		billingInterval = sub.BillingInterval.String
	}

	// Calculate grace period (3 days after period end)
	gracePeriod := 3 * 24 * time.Hour
	if sub.CurrentPeriodEnd.Valid && time.Since(sub.CurrentPeriodEnd.Time) > gracePeriod {
		// Grace period exceeded without renewal - downgrade to free
		log.Printf("⚠️ [Renewal] Grace period exceeded for user %s, downgrading to free", sub.UserID)
		return s.DowngradeToFree(ctx, sub.ID, sub.UserID)
	}

	// If within grace period, subscription is pending renewal
	// The webhook handler will update the subscription when payment succeeds
	log.Printf("⏳ [Renewal] Subscription %s is within grace period, awaiting payment", sub.ID)

	// Calculate next period dates for when renewal happens
	nextPeriodStart := time.Now()
	var nextPeriodEnd time.Time
	if billingInterval == "annual" {
		nextPeriodEnd = nextPeriodStart.AddDate(1, 0, 0)
	} else {
		nextPeriodEnd = nextPeriodStart.AddDate(0, 1, 0)
	}

	log.Printf("📅 [Renewal] Next period: %s to %s", nextPeriodStart.Format(time.RFC3339), nextPeriodEnd.Format(time.RFC3339))

	return nil
}

// RenewSubscription renews a subscription for a new period
func (s *SubscriptionRenewalService) RenewSubscription(ctx context.Context, subscriptionID uuid.UUID, billingInterval string) error {
	log.Printf("🔄 [Renewal] Renewing subscription %s", subscriptionID)

	now := time.Now()
	var nextPeriodEnd time.Time
	if billingInterval == "annual" {
		nextPeriodEnd = now.AddDate(1, 0, 0)
	} else {
		nextPeriodEnd = now.AddDate(0, 1, 0)
	}

	// Reset subscription period and URL usage
	_, err := s.queries.ResetSubscriptionPeriod(ctx, database.ResetSubscriptionPeriodParams{
		ID:                 subscriptionID,
		CurrentPeriodStart: sql.NullTime{Time: now, Valid: true},
		CurrentPeriodEnd:   sql.NullTime{Time: nextPeriodEnd, Valid: true},
	})

	if err != nil {
		return fmt.Errorf("failed to reset subscription period: %w", err)
	}

	log.Printf("✅ [Renewal] Subscription %s renewed until %s", subscriptionID, nextPeriodEnd.Format(time.RFC3339))
	return nil
}

// DowngradeToFree downgrades a subscription to free tier
func (s *SubscriptionRenewalService) DowngradeToFree(ctx context.Context, subscriptionID uuid.UUID, userID uuid.UUID) error {
	log.Printf("⬇️ [Renewal] Downgrading subscription %s to free", subscriptionID)

	// Update subscription to free
	_, err := s.queries.DowngradeToFree(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("failed to downgrade subscription: %w", err)
	}

	// Update user's subscription tier
	err = s.queries.UpdateUserSubscriptionTier(ctx, database.UpdateUserSubscriptionTierParams{
		ID:               userID,
		SubscriptionTier: "free",
	})
	if err != nil {
		return fmt.Errorf("failed to update user tier: %w", err)
	}

	// Clear Redis rate limit cache for this user
	s.clearUserRateLimitCache(userID)

	log.Printf("✅ [Renewal] User %s downgraded to free tier", userID)
	return nil
}

// ResetURLUsageForUser resets the URL usage counter for a user
func (s *SubscriptionRenewalService) ResetURLUsageForUser(ctx context.Context, userID uuid.UUID) error {
	log.Printf("🔄 [Renewal] Resetting URL usage for user %s", userID)

	// Get user's subscription
	sub, err := s.queries.GetUserSubscription(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user subscription: %w", err)
	}

	// Calculate new period dates
	billingInterval := "monthly"
	if sub.BillingInterval.Valid {
		billingInterval = sub.BillingInterval.String
	}

	now := time.Now()
	var nextPeriodEnd time.Time
	if billingInterval == "annual" {
		nextPeriodEnd = now.AddDate(1, 0, 0)
	} else {
		nextPeriodEnd = now.AddDate(0, 1, 0)
	}

	// Reset the period
	_, err = s.queries.ResetSubscriptionPeriod(ctx, database.ResetSubscriptionPeriodParams{
		ID:                 sub.ID,
		CurrentPeriodStart: sql.NullTime{Time: now, Valid: true},
		CurrentPeriodEnd:   sql.NullTime{Time: nextPeriodEnd, Valid: true},
	})

	if err != nil {
		return fmt.Errorf("failed to reset subscription period: %w", err)
	}

	// Clear Redis cache for rate limiting
	s.clearUserRateLimitCache(userID)

	log.Printf("✅ [Renewal] URL usage reset for user %s", userID)
	return nil
}

// clearUserRateLimitCache clears the Redis rate limit cache for a user
func (s *SubscriptionRenewalService) clearUserRateLimitCache(userID uuid.UUID) {
	if s.redis == nil {
		return
	}

	// Clear any cached rate limit data
	key := fmt.Sprintf("url_limit:%s", userID.String())
	s.redis.Del(database.Ctx, key)
}

// CheckURLLimit checks if a user can create more URLs based on their plan
func (s *SubscriptionRenewalService) CheckURLLimit(ctx context.Context, userID uuid.UUID) (canCreate bool, remaining int, resetDate time.Time, err error) {
	// Get user's subscription
	sub, err := s.queries.GetUserSubscription(ctx, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			// No subscription, use free limits
			return true, DefaultPlanLimits.Free, time.Now().AddDate(0, 1, 0), nil
		}
		return false, 0, time.Time{}, fmt.Errorf("failed to get subscription: %w", err)
	}

	// Get plan limit
	limit := GetURLLimitForPlan(sub.PlanID)

	// Get current usage
	usage, err := s.queries.GetSubscriptionUsage(ctx, userID)
	if err != nil {
		return false, 0, time.Time{}, fmt.Errorf("failed to get usage: %w", err)
	}

	usedCount := int(usage.UrlsCreatedThisPeriod.Int32)
	remaining = limit - usedCount
	canCreate = remaining > 0

	// Get reset date
	if usage.CurrentPeriodEnd.Valid {
		resetDate = usage.CurrentPeriodEnd.Time
	} else {
		resetDate = time.Now().AddDate(0, 1, 0)
	}

	return canCreate, remaining, resetDate, nil
}

// IncrementURLUsage increments the URL usage counter for a user
func (s *SubscriptionRenewalService) IncrementURLUsage(ctx context.Context, userID uuid.UUID) error {
	return s.queries.IncrementUrlUsage(ctx, userID)
}

// GetSubscriptionInfo returns subscription info for dashboard display
func (s *SubscriptionRenewalService) GetSubscriptionInfo(ctx context.Context, userID uuid.UUID) (*SubscriptionInfo, error) {
	sub, err := s.queries.GetUserSubscription(ctx, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			// Return default free subscription info
			return &SubscriptionInfo{
				PlanName:          "free",
				Status:            "active",
				BillingInterval:   "monthly",
				URLLimit:          DefaultPlanLimits.Free,
				URLsUsed:          0,
				URLsRemaining:     DefaultPlanLimits.Free,
				NextBillingDate:   nil,
				URLResetDate:      nil,
				IsCancelled:       false,
			}, nil
		}
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}

	limit := GetURLLimitForPlan(sub.PlanID)
	used := 0
	if sub.UrlsCreatedThisPeriod.Valid {
		used = int(sub.UrlsCreatedThisPeriod.Int32)
	}

	info := &SubscriptionInfo{
		PlanName:        sub.PlanID,
		Status:          sub.Status,
		URLLimit:        limit,
		URLsUsed:        used,
		URLsRemaining:   limit - used,
		IsCancelled:     sub.Status == "cancelled",
	}

	if sub.BillingInterval.Valid {
		info.BillingInterval = sub.BillingInterval.String
	} else {
		info.BillingInterval = "monthly"
	}

	if sub.CurrentPeriodEnd.Valid {
		info.NextBillingDate = &sub.CurrentPeriodEnd.Time
		info.URLResetDate = &sub.CurrentPeriodEnd.Time
	}

	return info, nil
}

// SubscriptionInfo contains subscription details for dashboard display
type SubscriptionInfo struct {
	PlanName        string     `json:"plan_name"`
	Status          string     `json:"status"`
	BillingInterval string     `json:"billing_interval"`
	URLLimit        int        `json:"url_limit"`
	URLsUsed        int        `json:"urls_used"`
	URLsRemaining   int        `json:"urls_remaining"`
	NextBillingDate *time.Time `json:"next_billing_date,omitempty"`
	URLResetDate    *time.Time `json:"url_reset_date,omitempty"`
	IsCancelled     bool       `json:"is_cancelled"`
}

// StartRenewalCron starts a background goroutine that checks for renewals periodically
func StartRenewalCron(ctx context.Context) {
	interval := 1 * time.Hour // Check every hour

	// Allow override via environment variable
	if envInterval := os.Getenv("RENEWAL_CHECK_INTERVAL"); envInterval != "" {
		if parsed, err := time.ParseDuration(envInterval); err == nil {
			interval = parsed
		}
	}

	service := NewSubscriptionRenewalService()

	go func() {
		log.Printf("🚀 [Renewal] Starting renewal cron job (interval: %s)", interval)

		// Run immediately on startup
		if err := service.ProcessExpiredSubscriptions(ctx); err != nil {
			log.Printf("❌ [Renewal] Initial check failed: %v", err)
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := service.ProcessExpiredSubscriptions(ctx); err != nil {
					log.Printf("❌ [Renewal] Scheduled check failed: %v", err)
				}
			case <-ctx.Done():
				log.Println("🛑 [Renewal] Stopping renewal cron job")
				return
			}
		}
	}()
}
