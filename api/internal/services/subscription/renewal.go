package subscription

import (
	"context"
	"database/sql"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/xenonnn4w/orangeurl/internal/database"
)

// SubscriptionInfo holds subscription details for API responses
type SubscriptionInfo struct {
	UserID                string     `json:"user_id"`
	PlanID                string     `json:"plan_id"`
	Status                string     `json:"status"`
	BillingInterval       string     `json:"billing_interval"`
	CurrentPeriodStart    *time.Time `json:"current_period_start"`
	CurrentPeriodEnd      *time.Time `json:"current_period_end"`
	URLsCreatedThisPeriod int32      `json:"urls_created_this_period"`
	URLUsageResetAt       *time.Time `json:"url_usage_reset_at"`
	URLLimit              int        `json:"url_limit"`
	URLsRemaining         int        `json:"urls_remaining"`
}

// URL limits per plan
var planURLLimits = map[string]int{
	"free":    10,
	"pro":     100,
	"premium": -1, // Unlimited
}

// GetSubscriptionInfo returns subscription info for a user
func GetSubscriptionInfo(ctx context.Context, userID uuid.UUID) (*SubscriptionInfo, error) {
	queries := database.GetQueries()

	sub, err := queries.GetUserSubscription(ctx, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			// Return free tier defaults
			return &SubscriptionInfo{
				UserID:          userID.String(),
				PlanID:          "free",
				Status:          "active",
				BillingInterval: "",
				URLLimit:        planURLLimits["free"],
				URLsRemaining:   planURLLimits["free"],
			}, nil
		}
		return nil, err
	}

	limit := planURLLimits[sub.PlanID]
	remaining := limit
	if limit > 0 {
		remaining = limit - int(sub.UrlsCreatedThisPeriod.Int32)
		if remaining < 0 {
			remaining = 0
		}
	}

	info := &SubscriptionInfo{
		UserID:                sub.UserID.String(),
		PlanID:                sub.PlanID,
		Status:                sub.Status,
		BillingInterval:       sub.BillingInterval.String,
		URLsCreatedThisPeriod: sub.UrlsCreatedThisPeriod.Int32,
		URLLimit:              limit,
		URLsRemaining:         remaining,
	}

	if sub.CurrentPeriodStart.Valid {
		info.CurrentPeriodStart = &sub.CurrentPeriodStart.Time
	}
	if sub.CurrentPeriodEnd.Valid {
		info.CurrentPeriodEnd = &sub.CurrentPeriodEnd.Time
	}
	if sub.UrlUsageResetAt.Valid {
		info.URLUsageResetAt = &sub.UrlUsageResetAt.Time
	}

	return info, nil
}

// CheckURLLimit checks if a user can create more URLs
func CheckURLLimit(ctx context.Context, userID uuid.UUID) (bool, int, error) {
	info, err := GetSubscriptionInfo(ctx, userID)
	if err != nil {
		return false, 0, err
	}

	// Premium users have unlimited URLs
	if info.URLLimit == -1 {
		return true, -1, nil
	}

	return info.URLsRemaining > 0, info.URLsRemaining, nil
}

// IncrementURLUsage increments the URL count for a user
func IncrementURLUsage(ctx context.Context, userID uuid.UUID) error {
	queries := database.GetQueries()
	_, err := queries.IncrementUrlUsage(ctx, userID)
	return err
}

// ResetURLUsageForUser resets the URL count for a specific user
func ResetURLUsageForUser(ctx context.Context, userID uuid.UUID, periodStart, periodEnd time.Time) error {
	queries := database.GetQueries()
	_, err := queries.ResetSubscriptionPeriod(ctx, database.ResetSubscriptionPeriodParams{
		UserID:             userID,
		CurrentPeriodStart: toNullTime(periodStart),
		CurrentPeriodEnd:   toNullTime(periodEnd),
	})
	return err
}

// DowngradeToFree downgrades a user to the free plan
func DowngradeToFree(ctx context.Context, userID uuid.UUID) error {
	queries := database.GetQueries()

	_, err := queries.DowngradeToFree(ctx, userID)
	if err != nil {
		return err
	}

	err = queries.UpdateUserSubscriptionTier(ctx, database.UpdateUserSubscriptionTierParams{
		ID:               userID,
		SubscriptionTier: "free",
	})
	return err
}

// ProcessExpiredSubscriptions finds and processes expired subscriptions
func ProcessExpiredSubscriptions(ctx context.Context) error {
	queries := database.GetQueries()

	// Get subscriptions that are about to expire or have expired
	subs, err := queries.GetSubscriptionsForRenewal(ctx)
	if err != nil {
		return err
	}

	log.Printf("Found %d subscriptions needing renewal check", len(subs))

	for _, sub := range subs {
		// If subscription hasn't been renewed by payment processor, it's expired
		if sub.CurrentPeriodEnd.Valid && sub.CurrentPeriodEnd.Time.Before(time.Now()) {
			log.Printf("Subscription for user %s has expired, downgrading to free", sub.UserID.String())
			if err := DowngradeToFree(ctx, sub.UserID); err != nil {
				log.Printf("Failed to downgrade user %s: %v", sub.UserID.String(), err)
			}
		}
	}

	return nil
}

// StartRenewalCron starts a background goroutine that checks for expired subscriptions
func StartRenewalCron(ctx context.Context) {
	interval := os.Getenv("RENEWAL_CHECK_INTERVAL")
	if interval == "" {
		interval = "1h" // Default to hourly
	}

	duration, err := time.ParseDuration(interval)
	if err != nil {
		log.Printf("Invalid RENEWAL_CHECK_INTERVAL, using 1h: %v", err)
		duration = time.Hour
	}

	go func() {
		ticker := time.NewTicker(duration)
		defer ticker.Stop()

		log.Printf("Subscription renewal cron started (interval: %s)", duration)

		for {
			select {
			case <-ctx.Done():
				log.Println("Subscription renewal cron stopped")
				return
			case <-ticker.C:
				log.Println("Running subscription renewal check...")
				if err := ProcessExpiredSubscriptions(context.Background()); err != nil {
					log.Printf("Error processing expired subscriptions: %v", err)
				}
			}
		}
	}()
}

// Helper function to convert time.Time to sql.NullTime
func toNullTime(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{Valid: false}
	}
	return sql.NullTime{Time: t, Valid: true}
}

// Helper function to convert string to sql.NullString
func toNullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}
