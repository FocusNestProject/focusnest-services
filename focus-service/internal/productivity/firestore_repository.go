package productivity

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type firestoreRepository struct {
	client *firestore.Client
}

// NewFirestoreRepository instantiates a Firestore-backed repository.
func NewFirestoreRepository(client *firestore.Client) Repository {
	return &firestoreRepository{client: client}
}

const productivitiesCollection = "activities"

func (r *firestoreRepository) userCollection(userID string) *firestore.CollectionRef {
	return r.client.Collection("users").Doc(userID).Collection(productivitiesCollection)
}

func (r *firestoreRepository) Create(ctx context.Context, entry Entry) error {
	data := map[string]any{
		"category":    entry.Category,
		"time_mode":   entry.TimeMode,
		"description": entry.Description,
		"mood":        entry.Mood,
		"cycles":      entry.Cycles,
		"elapsed_ms":  entry.ElapsedMs,
		"start_at":    entry.StartAt,
		"end_at":      entry.EndAt,
		"created_at":  entry.CreatedAt,
		"updated_at":  entry.UpdatedAt,
		"deleted":     false,
		"anchor":      entry.StartAt,
	}

	if entry.Image != nil {
		data["image"] = map[string]any{
			"original_path": entry.Image.OriginalPath,
			"overview_path": entry.Image.OverviewPath,
			"original_url":  entry.Image.OriginalURL,
			"overview_url":  entry.Image.OverviewURL,
		}
	}

	_, err := r.userCollection(entry.UserID).Doc(entry.ID).Create(ctx, data)
	if status.Code(err) == codes.AlreadyExists {
		return ErrConflict
	}
	return err
}

func (r *firestoreRepository) GetByID(ctx context.Context, userID, entryID string) (Entry, error) {
	doc, err := r.userCollection(userID).Doc(entryID).Get(ctx)
	if status.Code(err) == codes.NotFound {
		return Entry{}, ErrNotFound
	}
	if err != nil {
		return Entry{}, err
	}

	if deleted, ok := doc.Data()["deleted"].(bool); ok && deleted {
		return Entry{}, ErrNotFound
	}

	return snapshotToEntry(userID, doc)
}

func (r *firestoreRepository) Delete(ctx context.Context, userID, entryID string, deletedAt time.Time) error {
	ref := r.userCollection(userID).Doc(entryID)
	doc, err := ref.Get(ctx)
	if status.Code(err) == codes.NotFound {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	if deleted, ok := doc.Data()["deleted"].(bool); ok && deleted {
		return ErrNotFound
	}

	_, err = ref.Update(ctx, []firestore.Update{
		{Path: "deleted", Value: true},
		{Path: "updated_at", Value: deletedAt},
		{Path: "deleted_at", Value: deletedAt},
	})
	return err
}

func (r *firestoreRepository) ListByRange(ctx context.Context, userID string, startInclusive, endExclusive time.Time, pagination Pagination) ([]Entry, PageInfo, error) {
	if pagination.Page <= 0 {
		pagination.Page = 1
	}
	if pagination.PageSize <= 0 {
		pagination.PageSize = 20
	}

	collection := r.userCollection(userID)
	baseQuery := collection.
		Where("deleted", "==", false).
		Where("anchor", ">=", startInclusive).
		Where("anchor", "<", endExclusive)

	query := baseQuery.OrderBy("anchor", firestore.Desc).OrderBy("created_at", firestore.Desc)

	offset := (pagination.Page - 1) * pagination.PageSize
	if offset > 0 {
		query = query.Offset(offset)
	}

	iter := query.Limit(pagination.PageSize + 1).Documents(ctx)
	defer iter.Stop()
	entries := make([]Entry, 0)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, PageInfo{}, err
		}

		entry, err := snapshotToEntry(userID, doc)
		if err != nil {
			return nil, PageInfo{}, err
		}
		entries = append(entries, entry)
	}

	hasNext := len(entries) > pagination.PageSize
	if hasNext {
		entries = entries[:pagination.PageSize]
	}

	totalItems, totalPages, err := r.count(ctx, baseQuery, pagination.PageSize)
	if err != nil {
		return nil, PageInfo{}, err
	}

	return entries, PageInfo{
		Page:       pagination.Page,
		PageSize:   pagination.PageSize,
		TotalPages: totalPages,
		TotalItems: totalItems,
		HasNext:    hasNext,
	}, nil
}

func (r *firestoreRepository) count(ctx context.Context, query firestore.Query, pageSize int) (int, int, error) {
	iter := query.Documents(ctx)
	defer iter.Stop()

	total := 0
	for {
		_, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return 0, 0, fmt.Errorf("count query failed: %w", err)
		}
		total++
	}

	items := total
	totalPages := items / pageSize
	if items%pageSize != 0 {
		totalPages++
	}
	if totalPages == 0 {
		totalPages = 1
	}
	return items, totalPages, nil
}

func snapshotToEntry(userID string, doc *firestore.DocumentSnapshot) (Entry, error) {
	var payload struct {
		Category    string         `firestore:"category"`
		TimeMode    string         `firestore:"time_mode"`
		Description string         `firestore:"description"`
		Mood        string         `firestore:"mood"`
		Cycles      int            `firestore:"cycles"`
		ElapsedMs   int            `firestore:"elapsed_ms"`
		StartAt     time.Time      `firestore:"start_at"`
		EndAt       time.Time      `firestore:"end_at"`
		CreatedAt   time.Time      `firestore:"created_at"`
		UpdatedAt   time.Time      `firestore:"updated_at"`
		DeletedAt   time.Time      `firestore:"deleted_at"`
		Image       map[string]any `firestore:"image"`
	}
	if err := doc.DataTo(&payload); err != nil {
		return Entry{}, err
	}

	entry := Entry{
		ID:          doc.Ref.ID,
		UserID:      userID,
		Category:    payload.Category,
		TimeMode:    payload.TimeMode,
		Description: payload.Description,
		Mood:        payload.Mood,
		Cycles:      payload.Cycles,
		ElapsedMs:   payload.ElapsedMs,
		StartAt:     payload.StartAt,
		EndAt:       payload.EndAt,
		CreatedAt:   payload.CreatedAt,
		UpdatedAt:   payload.UpdatedAt,
	}

	if !payload.DeletedAt.IsZero() {
		entry.DeletedAt = &payload.DeletedAt
	}

	// Parse image data if present
	if payload.Image != nil {
		entry.Image = &ImageInfo{
			OriginalPath: getStringFromMap(payload.Image, "original_path"),
			OverviewPath: getStringFromMap(payload.Image, "overview_path"),
			OriginalURL:  getStringFromMap(payload.Image, "original_url"),
			OverviewURL:  getStringFromMap(payload.Image, "overview_url"),
		}
	}

	return entry, nil
}

func getStringFromMap(m map[string]any, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}
