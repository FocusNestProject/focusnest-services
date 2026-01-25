package user

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	profileLocation = loadProfileLocation()
)

type firestoreRepository struct {
	client *firestore.Client
}

// NewFirestoreRepository creates a new Firestore repository
func NewFirestoreRepository(client *firestore.Client) Repository {
	return &firestoreRepository{client: client}
}

func loadProfileLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.UTC
	}
	return loc
}

func (r *firestoreRepository) GetProfile(ctx context.Context, userID string) (*Profile, error) {
	doc, err := r.client.Collection("profiles").Doc(userID).Get(ctx)
	if status.Code(err) == codes.NotFound {
		return defaultProfile(userID), nil
	}
	if err != nil {
		return nil, err
	}

	var profile Profile
	if err := doc.DataTo(&profile); err != nil {
		return nil, fmt.Errorf("unmarshal profile: %w", err)
	}
	profile.UserID = userID
	return &profile, nil
}

func (r *firestoreRepository) UpsertProfile(ctx context.Context, userID string, updates ProfileUpdateInput) (*Profile, error) {
	docRef := r.client.Collection("profiles").Doc(userID)
	now := time.Now().UTC()

	err := r.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		data := map[string]interface{}{
			"user_id":    userID,
			"updated_at": now,
		}

		if updates.Bio != nil {
			data["bio"] = strings.TrimSpace(*updates.Bio)
		}
		if updates.Birthdate != nil && updates.Birthdate.IsSet {
			data["birthdate"] = updates.Birthdate.Value
		}

		if _, err := tx.Get(docRef); status.Code(err) == codes.NotFound {
			data["created_at"] = now
			// Ensure the field exists for new profiles.
			data["points_total"] = 0
		} else if err != nil {
			return err
		}

		return tx.Set(docRef, data, firestore.MergeAll)
	})
	if err != nil {
		return nil, err
	}

	return r.GetProfile(ctx, userID)
}

func (r *firestoreRepository) GetProfileMetadata(ctx context.Context, userID string) (ProfileMetadata, error) {
	metrics := ProfileMetadata{}
	query := r.productivitiesQuery(userID).
		Select("start_time", "deleted", "num_cycle", "category").
		OrderBy("start_time", firestore.Asc)
	iter := query.Documents(ctx)
	defer iter.Stop()

	var prevDay time.Time
	var lastProcessedDay time.Time
	current := 0
	longest := 0
	
	// Track unique categories for TotalProductivities
	uniqueCategories := make(map[string]bool)

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return metrics, err
		}

		var snapshot struct {
			StartTime time.Time `firestore:"start_time"`
			Deleted   bool      `firestore:"deleted"`
			NumCycle  int       `firestore:"num_cycle"`
			Category  string    `firestore:"category"`
		}
		if err := doc.DataTo(&snapshot); err != nil {
			return metrics, fmt.Errorf("decode productivity snapshot: %w", err)
		}
		if snapshot.Deleted {
			continue
		}
		
		// Track unique categories (only count non-empty categories)
		// Trim and check category - empty string means field doesn't exist or is empty
		category := strings.TrimSpace(snapshot.Category)
		if category != "" {
			uniqueCategories[category] = true
		}
		
		metrics.TotalSessions++
		metrics.TotalCycle += snapshot.NumCycle

		if snapshot.StartTime.IsZero() {
			continue
		}
		day := snapshot.StartTime.In(profileLocation).Truncate(24 * time.Hour)
		if !lastProcessedDay.IsZero() && day.Equal(lastProcessedDay) {
			continue
		}

		if prevDay.IsZero() {
			current = 1
		} else if day.Equal(prevDay.AddDate(0, 0, 1)) {
			current++
		} else if day.After(prevDay) {
			current = 1
		}

		if current > longest {
			longest = current
		}
		prevDay = day
		lastProcessedDay = day
	}

	// Set TotalProductivities to count of unique categories
	metrics.TotalProductivities = len(uniqueCategories)
	metrics.LongestStreak = longest
	return metrics, nil
}

func (r *firestoreRepository) productivitiesQuery(userID string) firestore.Query {
	return r.client.Collection("users").Doc(userID).Collection("productivities").Query
}

func (r *firestoreRepository) GetDailyMinutesByDate(ctx context.Context, userID string, startDate, endDate time.Time) (map[string]int, error) {
	// Primary source: daily_summaries (same schema as progress-service).
	iter := r.client.Collection("daily_summaries").
		Where("user_id", "==", userID).
		Where("date", ">=", startDate).
		Where("date", "<", endDate).
		OrderBy("date", firestore.Asc).
		Documents(ctx)

	defer iter.Stop()

	minsByDate := make(map[string]int)
	found := 0
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		var summary struct {
			Date      time.Time `firestore:"date"`
			TotalTime int       `firestore:"total_time"` // minutes
		}
		if err := doc.DataTo(&summary); err != nil {
			continue
		}
		key := summary.Date.In(profileLocation).Format("2006-01-02")
		minsByDate[key] = summary.TotalTime
		found++
	}

	if found > 0 {
		return minsByDate, nil
	}

	// Fallback: aggregate from productivities (only within the requested window).
	entries, err := r.fetchProductivitiesForWindow(ctx, userID, startDate, endDate)
	if err != nil {
		return nil, err
	}

	for _, e := range entries {
		if e.StartTime.IsZero() {
			continue
		}
		mins := e.TimeElapsed / 60
		if mins <= 0 && e.TimeElapsed > 0 {
			mins = 1
		}
		key := e.StartTime.In(profileLocation).Format("2006-01-02")
		minsByDate[key] += mins
	}

	return minsByDate, nil
}

func (r *firestoreRepository) fetchProductivitiesForWindow(ctx context.Context, userID string, startDate, endDate time.Time) ([]ProductivityEntry, error) {
	iter := r.client.Collection("users").Doc(userID).Collection("productivities").
		Where("anchor", ">=", startDate).
		Where("anchor", "<", endDate).
		Where("deleted", "==", false).
		OrderBy("anchor", firestore.Asc).
		Documents(ctx)
	defer iter.Stop()

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
			TimeElapsed int       `firestore:"time_elapsed"`
		}
		if err := doc.DataTo(&payload); err != nil {
			continue
		}
		entries = append(entries, ProductivityEntry{
			StartTime:   payload.StartTime,
			TimeElapsed: payload.TimeElapsed,
		})
	}
	return entries, nil
}

type ProductivityEntry struct {
	StartTime   time.Time
	TimeElapsed int
}

func (r *firestoreRepository) IsChallengeClaimed(ctx context.Context, userID, challengeID string) (bool, error) {
	if userID == "" || challengeID == "" {
		return false, nil
	}
	ref := r.client.Collection("profiles").Doc(userID).Collection("challenge_claims").Doc(challengeID)
	_, err := ref.Get(ctx)
	if status.Code(err) == codes.NotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *firestoreRepository) ClaimChallenge(ctx context.Context, userID, challengeID string, points int) (newTotal int, claimedAt time.Time, alreadyClaimed bool, err error) {
	if userID == "" || challengeID == "" {
		return 0, time.Time{}, false, fmt.Errorf("missing identifiers")
	}
	if points <= 0 {
		return 0, time.Time{}, false, fmt.Errorf("invalid points")
	}

	profileRef := r.client.Collection("profiles").Doc(userID)
	claimRef := profileRef.Collection("challenge_claims").Doc(challengeID)
	now := time.Now().UTC()

	err = r.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// If claim doc exists, do nothing (idempotent).
		_, getErr := tx.Get(claimRef)
		if getErr == nil {
			alreadyClaimed = true
			// Still return current points_total.
			doc, err := tx.Get(profileRef)
			if err != nil {
				return err
			}
			var p Profile
			if err := doc.DataTo(&p); err != nil {
				return err
			}
			newTotal = p.PointsTotal
			return nil
		}
		if status.Code(getErr) != codes.NotFound {
			return getErr
		}

		// Ensure profile exists; if missing, create baseline.
		if _, err := tx.Get(profileRef); status.Code(err) == codes.NotFound {
			if err := tx.Set(profileRef, map[string]any{
				"user_id":       userID,
				"points_total":  0,
				"created_at":    now,
				"updated_at":    now,
				"bio":           "",
				"birthdate":     nil,
			}, firestore.MergeAll); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

		// Increment points atomically.
		if err := tx.Update(profileRef, []firestore.Update{
			{Path: "points_total", Value: firestore.Increment(int64(points))},
			{Path: "updated_at", Value: now},
		}); err != nil {
			return err
		}

		// Create claim doc.
		if err := tx.Create(claimRef, map[string]any{
			"challenge_id":   challengeID,
			"points_awarded": points,
			"claimed_at":     now,
		}); err != nil {
			// If race created it, treat as already claimed.
			if status.Code(err) == codes.AlreadyExists {
				alreadyClaimed = true
				return nil
			}
			return err
		}

		claimedAt = now
		return nil
	})
	if err != nil {
		return 0, time.Time{}, false, err
	}

	// If we awarded, fetch the updated total. (We could read in-tx, but Increment is easier to resolve after.)
	if !alreadyClaimed {
		p, err2 := r.GetProfile(ctx, userID)
		if err2 != nil {
			return 0, time.Time{}, false, err2
		}
		newTotal = p.PointsTotal
	}

	return newTotal, claimedAt, alreadyClaimed, nil
}

// GetWeeklyShareCount returns the number of shares for a user in the given week.
// weekStart should be the Monday of the week (truncated to day).
func (r *firestoreRepository) GetWeeklyShareCount(ctx context.Context, userID string, weekStart time.Time) (int, error) {
	if userID == "" {
		return 0, nil
	}

	weekEnd := weekStart.AddDate(0, 0, 7) // Monday + 7 days

	iter := r.client.Collection("profiles").Doc(userID).Collection("shares").
		Where("shared_at", ">=", weekStart).
		Where("shared_at", "<", weekEnd).
		Documents(ctx)
	defer iter.Stop()

	count := 0
	for {
		_, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return 0, err
		}
		count++
	}

	return count, nil
}

// RecordShare records a share event for the user.
// shareType can be "recap", "achievement", etc.
func (r *firestoreRepository) RecordShare(ctx context.Context, userID string, shareType string) error {
	if userID == "" {
		return fmt.Errorf("missing user id")
	}

	now := time.Now().UTC()
	sharesRef := r.client.Collection("profiles").Doc(userID).Collection("shares")

	_, _, err := sharesRef.Add(ctx, map[string]any{
		"share_type": shareType,
		"shared_at":  now,
	})

	return err
}

// GetCurrentStreak returns the current consecutive active days streak for the user.
// An "active" day is any day with at least one productivity session.
func (r *firestoreRepository) GetCurrentStreak(ctx context.Context, userID string) (int, error) {
	if userID == "" {
		return 0, nil
	}

	// Get productivities ordered by start_time descending
	query := r.productivitiesQuery(userID).
		Select("start_time", "deleted").
		OrderBy("start_time", firestore.Desc)
	iter := query.Documents(ctx)
	defer iter.Stop()

	// Collect unique active days
	activeDays := make(map[string]bool)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return 0, err
		}

		var snapshot struct {
			StartTime time.Time `firestore:"start_time"`
			Deleted   bool      `firestore:"deleted"`
		}
		if err := doc.DataTo(&snapshot); err != nil {
			continue
		}
		if snapshot.Deleted || snapshot.StartTime.IsZero() {
			continue
		}

		day := snapshot.StartTime.In(profileLocation).Format("2006-01-02")
		activeDays[day] = true
	}

	if len(activeDays) == 0 {
		return 0, nil
	}

	// Calculate current streak from today backwards
	today := time.Now().In(profileLocation)
	streak := 0

	for i := 0; i < 365; i++ { // Safety cap at 1 year
		day := today.AddDate(0, 0, -i)
		key := day.Format("2006-01-02")
		if activeDays[key] {
			streak++
		} else if i == 0 {
			// Today might not have activity yet, check yesterday
			continue
		} else {
			break
		}
	}

	return streak, nil
}

// GetTodayCycles returns the total number of work cycles completed by the user today.
func (r *firestoreRepository) GetTodayCycles(ctx context.Context, userID string) (int, error) {
	if userID == "" {
		return 0, nil
	}

	// Get start and end of today in user's timezone (Asia/Jakarta)
	now := time.Now().In(profileLocation)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, profileLocation)
	endOfDay := startOfDay.Add(24 * time.Hour)

	query := r.productivitiesQuery(userID).
		Where("start_time", ">=", startOfDay).
		Where("start_time", "<", endOfDay).
		Select("num_cycle", "deleted", "start_time")
	iter := query.Documents(ctx)
	defer iter.Stop()

	totalCycles := 0
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return 0, err
		}

		var snapshot struct {
			NumCycle  int       `firestore:"num_cycle"`
			Deleted   bool      `firestore:"deleted"`
			StartTime time.Time `firestore:"start_time"`
		}
		if err := doc.DataTo(&snapshot); err != nil {
			continue
		}
		if snapshot.Deleted {
			continue
		}
		totalCycles += snapshot.NumCycle
	}

	return totalCycles, nil
}

// GetTodayMindfulnessMinutes returns the total mindfulness minutes for the user today.
func (r *firestoreRepository) GetTodayMindfulnessMinutes(ctx context.Context, userID string) (int, error) {
	if userID == "" {
		return 0, nil
	}

	// Get start and end of today in user's timezone (Asia/Jakarta)
	now := time.Now().In(profileLocation)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, profileLocation)
	endOfDay := startOfDay.Add(24 * time.Hour)

	iter := r.client.Collection("profiles").Doc(userID).Collection("mindfulness").
		Where("completed_at", ">=", startOfDay).
		Where("completed_at", "<", endOfDay).
		Documents(ctx)
	defer iter.Stop()

	totalMinutes := 0
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return 0, err
		}

		var snapshot struct {
			Minutes     int       `firestore:"minutes"`
			CompletedAt time.Time `firestore:"completed_at"`
		}
		if err := doc.DataTo(&snapshot); err != nil {
			continue
		}
		totalMinutes += snapshot.Minutes
	}

	return totalMinutes, nil
}

// RecordMindfulness records a mindfulness session for the user.
func (r *firestoreRepository) RecordMindfulness(ctx context.Context, userID string, minutes int) error {
	if userID == "" {
		return fmt.Errorf("missing user id")
	}
	if minutes <= 0 {
		return fmt.Errorf("invalid minutes")
	}

	now := time.Now().UTC()
	mindfulnessRef := r.client.Collection("profiles").Doc(userID).Collection("mindfulness")

	_, _, err := mindfulnessRef.Add(ctx, map[string]any{
		"minutes":      minutes,
		"completed_at": now,
	})

	return err
}
