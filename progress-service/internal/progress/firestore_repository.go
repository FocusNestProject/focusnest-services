package progress

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	streakStateColl        = "streak_state"
	streakRecoveryQuotaColl = "streak_recovery_quota"
	recoveryQuotaLimit     = 5
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

func (r *firestoreRepository) GetStreakState(ctx context.Context, userID string) (*StreakState, error) {
	doc, err := r.client.Collection(streakStateColl).Doc(userID).Get(ctx)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	var state StreakState
	if err := doc.DataTo(&state); err != nil {
		return nil, fmt.Errorf("unmarshal streak_state: %w", err)
	}
	return &state, nil
}

func (r *firestoreRepository) SetStreakState(ctx context.Context, userID string, state *StreakState) error {
	state.UserID = userID
	state.UpdatedAt = time.Now().UTC()
	_, err := r.client.Collection(streakStateColl).Doc(userID).Set(ctx, state)
	return err
}

func (r *firestoreRepository) GetRecoveryQuota(ctx context.Context, userID string, yearMonth string) (int, error) {
	docID := userID + "_" + yearMonth
	doc, err := r.client.Collection(streakRecoveryQuotaColl).Doc(docID).Get(ctx)
	if err != nil {
		if isNotFound(err) {
			return 0, nil
		}
		return 0, err
	}
	var q RecoveryQuota
	if err := doc.DataTo(&q); err != nil {
		return 0, fmt.Errorf("unmarshal recovery_quota: %w", err)
	}
	return q.Count, nil
}

func (r *firestoreRepository) IncrementRecoveryQuota(ctx context.Context, userID string, yearMonth string) (int, error) {
	docID := userID + "_" + yearMonth
	ref := r.client.Collection(streakRecoveryQuotaColl).Doc(docID)
	err := r.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(ref)
		var count int
		if err != nil && !isNotFound(err) {
			return err
		}
		if err == nil {
			var q RecoveryQuota
			if err := doc.DataTo(&q); err != nil {
				return err
			}
			count = q.Count
		}
		if count >= recoveryQuotaLimit {
			return ErrRecoveryQuotaExceeded
		}
		count++
		return tx.Set(ref, &RecoveryQuota{
			UserID:    userID,
			YearMonth: yearMonth,
			Count:     count,
			UpdatedAt: time.Now().UTC(),
		})
	})
	if err != nil {
		return 0, err
	}
	// Return new count
	return r.GetRecoveryQuota(ctx, userID, yearMonth)
}

func isNotFound(err error) bool {
	return err != nil && status.Code(err) == codes.NotFound
}
