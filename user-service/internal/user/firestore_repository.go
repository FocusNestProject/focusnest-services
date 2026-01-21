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
