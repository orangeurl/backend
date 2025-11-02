package jobs

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/xenonnn4w/orangeurl/internal/database"
)

// CleanupOldAnalytics removes analytics data older than the retention period based on user tier
func CleanupOldAnalytics() {
	log.Println("[Cleanup] Starting analytics data retention cleanup...")
	
	queries := database.GetQueries()
	if queries == nil {
		log.Println("[Cleanup] Database not initialized, skipping cleanup")
		return
	}

	ctx := context.Background()

	// Get all users
	users, err := queries.ListUsers(ctx)
	if err != nil {
		log.Printf("[Cleanup] Error fetching users: %v", err)
		return
	}

	deletedCount := 0
	for _, user := range users {
		// Get user's subscription to determine retention period
		subscription, subErr := queries.GetUserSubscription(ctx, user.ID)
		retentionDays := 30 // Free tier default
		planTier := "free"

		if subErr == nil {
			planTier = subscription.PlanID
			if subscription.PlanID == "pro" {
				retentionDays = 365 // 1 year
			} else if subscription.PlanID == "premium" {
				// Premium has unlimited retention - skip cleanup
				log.Printf("[Cleanup] Skipping user %s (Premium tier - unlimited retention)", user.Email)
				continue
			}
		}

		cutoffDate := time.Now().AddDate(0, 0, -retentionDays)

		// Delete old analytics for this user's URLs
		err := queries.DeleteOldAnalytics(ctx, database.DeleteOldAnalyticsParams{
			UserID:   user.ID,
			ClickedAt: sql.NullTime{Time: cutoffDate, Valid: true},
		})

		if err != nil {
			log.Printf("[Cleanup] Error deleting analytics for user %s: %v", user.Email, err)
		} else {
			deletedCount++
			log.Printf("[Cleanup] Deleted analytics older than %s for user %s (tier: %s, retention: %d days)",
				cutoffDate.Format("2006-01-02"), user.Email, planTier, retentionDays)
		}
	}

	log.Printf("[Cleanup] Processed %d users, deleted analytics for %d users", len(users), deletedCount)

	log.Println("[Cleanup] Analytics cleanup completed")
}

// StartCleanupScheduler runs cleanup job periodically
func StartCleanupScheduler() {
	ticker := time.NewTicker(24 * time.Hour) // Run daily
	go func() {
		for range ticker.C {
			CleanupOldAnalytics()
		}
	}()
	
	log.Println("[Cleanup] Scheduler started - will run daily")
}



