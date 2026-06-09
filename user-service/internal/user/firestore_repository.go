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

type firestoreRepository struct {
	client *firestore.Client
}

// NewFirestoreRepository creates a new Firestore repository
func NewFirestoreRepository(client *firestore.Client) Repository {
	return &firestoreRepository{client: client}
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

func (r *firestoreRepository) GetProfileMetadata(ctx context.Context, userID string, loc *time.Location) (ProfileMetadata, error) {
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
		cycle := snapshot.NumCycle
		if cycle <= 0 {
			cycle = 1
		}
		metrics.TotalCycle += cycle

		if snapshot.StartTime.IsZero() {
			continue
		}
		localStart := snapshot.StartTime.In(loc)
		day := time.Date(localStart.Year(), localStart.Month(), localStart.Day(), 0, 0, 0, 0, loc)
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

func (r *firestoreRepository) GetDailyMinutesByDate(ctx context.Context, userID string, startDate, endDate time.Time, loc *time.Location) (map[string]int, error) {
	// Always aggregate from productivities (single source of truth).
	// daily_summaries is not maintained by the app and may contain stale simulation data.
	minsByDate := make(map[string]int)
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
		key := e.StartTime.In(loc).Format("2006-01-02")
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

func (r *firestoreRepository) ListChallenges(ctx context.Context) ([]ChallengeDefinition, error) {
	iter := r.client.Collection("challenges").Documents(ctx)
	defer iter.Stop()

	var challenges []ChallengeDefinition
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to fetch challenges: %w", err)
		}
		var def ChallengeDefinition
		if err := doc.DataTo(&def); err != nil {
			continue
		}
		challenges = append(challenges, def)
	}
	return challenges, nil
}

func (r *firestoreRepository) CreateChallenge(ctx context.Context, def ChallengeDefinition) error {
	_, err := r.client.Collection("challenges").Doc(def.ID).Set(ctx, def)
	return err
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
func (r *firestoreRepository) GetCurrentStreak(ctx context.Context, userID string, loc *time.Location) (int, error) {
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

		day := snapshot.StartTime.In(loc).Format("2006-01-02")
		activeDays[day] = true
	}

	if len(activeDays) == 0 {
		return 0, nil
	}

	// Calculate current streak from today backwards
	today := time.Now().In(loc)
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

// GetCyclesByDate returns the total number of work cycles completed by the user on a specific date.
func (r *firestoreRepository) GetCyclesByDate(ctx context.Context, userID string, date time.Time, loc *time.Location) (int, error) {
	if userID == "" {
		return 0, nil
	}

	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)
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

// GetMindfulnessMinutesByDate returns the total mindfulness minutes for the user on a specific date.
func (r *firestoreRepository) GetMindfulnessMinutesByDate(ctx context.Context, userID string, date time.Time, loc *time.Location) (int, error) {
	if userID == "" {
		return 0, nil
	}

	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, loc)
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
