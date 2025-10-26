package jobs

import (
	"context"
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

	for _, user := range users {
		// Get user's subscription to determine retention period
		subscription, subErr := queries.GetUserSubscription(ctx, user.ID)
		retentionDays := 30 // Free tier default
		
		if subErr == nil {
			if subscription.PlanID == "pro" {
				retentionDays = 365 // 1 year
			} else if subscription.PlanID == "premium" {
				// Premium has unlimited retention - skip cleanup
				continue
			}
		}

		cutoffDate := time.Now().AddDate(0, 0, -retentionDays)
		
		// Delete old analytics for this user's URLs
		// Note: This requires a custom SQL query - marking as TODO for now
		log.Printf("[Cleanup] Would delete analytics older than %s for user %s (tier: %s, retention: %d days)",
			cutoffDate.Format("2006-01-02"), user.Email, subscription.PlanID, retentionDays)
		
		// TODO: Implement actual deletion query
		// DELETE FROM url_clicks WHERE url_id IN (SELECT id FROM urls WHERE user_id = ?) AND clicked_at < ?
	}

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

