package progress

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
)

type firestoreRepository struct {
	client *firestore.Client
}

// NewFirestoreRepository creates a new Firestore repository
func NewFirestoreRepository(client *firestore.Client) Repository {
	return &firestoreRepository{client: client}
}

func (r *firestoreRepository) GetDailySummaries(userID string, startDate, endDate time.Time) ([]*DailySummary, error) {
	ctx := context.Background()
	
	// First try to get from daily_summaries collection
	iter := r.client.Collection("daily_summaries").
		Where("user_id", "==", userID).
		Where("date", ">=", startDate).
		Where("date", "<=", endDate).
		OrderBy("date", firestore.Asc).
		Documents(ctx)

	var summaries []*DailySummary
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		var summary DailySummary
		if err := doc.DataTo(&summary); err != nil {
			return nil, fmt.Errorf("unmarshal summary: %w", err)
		}
		summary.ID = doc.Ref.ID
		summaries = append(summaries, &summary)
	}

	// If no daily summaries found, try to aggregate from productivities
	if len(summaries) == 0 {
		return r.aggregateFromProductivities(userID, startDate, endDate)
	}
	
	return summaries, nil
}

// aggregateFromProductivities reads from productivities collection and creates daily summaries
func (r *firestoreRepository) aggregateFromProductivities(userID string, startDate, endDate time.Time) ([]*DailySummary, error) {
	ctx := context.Background()
	
	// Query productivities collection
	iter := r.client.Collection("users").Doc(userID).Collection("productivities").
		Where("start_at", ">=", startDate).
		Where("start_at", "<", endDate).
		Where("deleted", "==", false).
		OrderBy("start_at", firestore.Asc).
		Documents(ctx)

	// Group by day
	dayMap := make(map[string]*DailySummary)
	
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		var entry struct {
			StartAt   time.Time `firestore:"start_at"`
			ElapsedMs int       `firestore:"elapsed_ms"`
			Category  string    `firestore:"category"`
		}
		
		if err := doc.DataTo(&entry); err != nil {
			continue // Skip invalid entries
		}

		// Get day string
		dayStr := entry.StartAt.Format("2006-01-02")
		
		// Create or update daily summary
		if summary, exists := dayMap[dayStr]; exists {
			summary.TotalTime += entry.ElapsedMs / (1000 * 60) // Convert to minutes
			summary.Categories[entry.Category]++
		} else {
			dayMap[dayStr] = &DailySummary{
				ID:         doc.Ref.ID,
				UserID:     userID,
				Date:       entry.StartAt.Truncate(24 * time.Hour),
				TotalTime:  entry.ElapsedMs / (1000 * 60), // Convert to minutes
				Categories: map[string]int{entry.Category: 1},
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}
		}
	}

	// Convert map to slice
	var summaries []*DailySummary
	for _, summary := range dayMap {
		summaries = append(summaries, summary)
	}

	return summaries, nil
}

func (r *firestoreRepository) GetProgressStats(userID string, startDate, endDate time.Time) (*ProgressStats, error) {
	summaries, err := r.GetDailySummaries(userID, startDate, endDate)
	if err != nil {
		return nil, err
	}

	stats := &ProgressStats{
		TotalTime:     0,
		TotalSessions: 0,
		Categories:    make(map[string]int),
		Periods:       make(map[string]interface{}),
	}

	for _, summary := range summaries {
		stats.TotalTime += summary.TotalTime
		stats.TotalSessions++

		for category, time := range summary.Categories {
			stats.Categories[category] += time
		}
	}

	// Calculate period stats
	stats.Periods["week"] = map[string]interface{}{
		"total_time": stats.TotalTime,
		"sessions":   stats.TotalSessions,
	}

	stats.Periods["month"] = map[string]interface{}{
		"total_time": stats.TotalTime,
		"sessions":   stats.TotalSessions,
	}

	return stats, nil
}
