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
