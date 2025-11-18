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

func (r *firestoreRepository) GetDailySummaries(ctx context.Context, userID string, startDate, endDate time.Time) ([]*DailySummary, error) {
	// Use [start, end) everywhere for consistency
	iter := r.client.Collection("daily_summaries").
		Where("user_id", "==", userID).
		Where("date", ">=", startDate).
		Where("date", "<", endDate).
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
		// Ensure not persisted; but we expose the doc id to caller
		summary.ID = doc.Ref.ID
		if summary.Categories == nil {
			summary.Categories = map[string]int{}
		}
		summaries = append(summaries, &summary)
	}

	// If no daily summaries found, aggregate from productivities
	if len(summaries) == 0 {
		return r.aggregateFromProductivities(ctx, userID, startDate, endDate)
	}
	return summaries, nil
}

// aggregateFromProductivities reads from productivities collection and creates daily summaries
func (r *firestoreRepository) aggregateFromProductivities(ctx context.Context, userID string, startDate, endDate time.Time) ([]*DailySummary, error) {
	entries, err := r.fetchProductivities(ctx, userID, startDate, endDate)
	if err != nil {
		return nil, err
	}

	dayMap := make(map[string]*DailySummary)
	for _, entry := range entries {
		if entry.StartTime.IsZero() {
			continue
		}
		mins := entry.TimeElapsed / 60
		if mins <= 0 && entry.TimeElapsed > 0 {
			mins = 1
		}
		dayStr := entry.StartTime.Format("2006-01-02")
		if summary, exists := dayMap[dayStr]; exists {
			summary.TotalTime += mins
			summary.Categories[entry.Category] += mins
			summary.Sessions++
		} else {
			dayMap[dayStr] = &DailySummary{
				ID:         dayStr,
				UserID:     userID,
				Date:       entry.StartTime.Truncate(24 * time.Hour),
				TotalTime:  mins,
				Categories: map[string]int{entry.Category: mins},
				Sessions:   1,
				CreatedAt:  time.Now().UTC(),
				UpdatedAt:  time.Now().UTC(),
			}
		}
	}

	summaries := make([]*DailySummary, 0, len(dayMap))
	for _, s := range dayMap {
		summaries = append(summaries, s)
	}

	return summaries, nil
}

func (r *firestoreRepository) ListProductivities(ctx context.Context, userID string, startDate, endDate time.Time) ([]ProductivityEntry, error) {
	return r.fetchProductivities(ctx, userID, startDate, endDate)
}

func (r *firestoreRepository) fetchProductivities(ctx context.Context, userID string, startDate, endDate time.Time) ([]ProductivityEntry, error) {
	iter := r.client.Collection("users").Doc(userID).Collection("productivities").
		Where("anchor", ">=", startDate).
		Where("anchor", "<", endDate).
		Where("deleted", "==", false).
		OrderBy("anchor", firestore.Asc).
		Documents(ctx)

	var entries []ProductivityEntry
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		var payload struct {
			StartTime   time.Time `firestore:"start_time"`
			EndTime     time.Time `firestore:"end_time"`
			TimeElapsed int       `firestore:"time_elapsed"`
			Category    string    `firestore:"category"`
		}
		if err := doc.DataTo(&payload); err != nil {
			continue
		}
		entries = append(entries, ProductivityEntry{
			StartTime:   payload.StartTime,
			EndTime:     payload.EndTime,
			TimeElapsed: payload.TimeElapsed,
			Category:    payload.Category,
		})
	}

	return entries, nil
}

func (r *firestoreRepository) GetProgressStats(ctx context.Context, userID string, startDate, endDate time.Time) (*ProgressStats, error) {
	summaries, err := r.GetDailySummaries(ctx, userID, startDate, endDate)
	if err != nil {
		return nil, err
	}

	stats := &ProgressStats{
		TotalTime:     0,
		TotalSessions: 0,
		Categories:    make(map[string]int),
		Periods:       make(map[string]interface{}),
	}

	for _, s := range summaries {
		stats.TotalTime += s.TotalTime
		stats.TotalSessions += s.Sessions
		for cat, mins := range s.Categories {
			stats.Categories[cat] += mins
		}
	}

	// Caller defines the period semantics via start/end.
	// Here we just mirror totals for convenience buckets.
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
