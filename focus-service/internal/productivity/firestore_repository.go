package productivity

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	firestorepb "google.golang.org/genproto/googleapis/firestore/v1"
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

const productivitiesCollection = "productivities"

func (r *firestoreRepository) userCollection(userID string) *firestore.CollectionRef {
	return r.client.Collection("users").Doc(userID).Collection(productivitiesCollection)
}

func (r *firestoreRepository) Create(ctx context.Context, entry Entry) error {
	data := map[string]any{
		"activity_name": entry.ActivityName,
		"time_elapsed":  entry.TimeElapsed,
		"num_cycle":     entry.NumCycle,
		"time_mode":     entry.TimeMode,
		"category":      entry.Category,
		"description":   entry.Description,
		"mood":          entry.Mood,
		"image":         entry.Image,
		"start_time":    entry.StartTime,
		"end_time":      entry.EndTime,
		"created_at":    entry.CreatedAt,
		"updated_at":    entry.UpdatedAt,
		"deleted":       false,
		// anchor is the canonical sort/filter field for time-range queries
		"anchor": entry.StartTime,
	}

	_, err := r.userCollection(entry.UserID).Doc(entry.ID).Create(ctx, data)
	if status.Code(err) == codes.AlreadyExists {
		return ErrConflict
	}
	return err
}

func (r *firestoreRepository) Update(ctx context.Context, entry Entry) error {
	data := map[string]any{
		"activity_name": entry.ActivityName,
		"time_elapsed":  entry.TimeElapsed,
		"num_cycle":     entry.NumCycle,
		"time_mode":     entry.TimeMode,
		"category":      entry.Category,
		"description":   entry.Description,
		"mood":          entry.Mood,
		"image":         entry.Image,
		"start_time":    entry.StartTime,
		"end_time":      entry.EndTime,
		"updated_at":    entry.UpdatedAt,
		"anchor":        entry.StartTime,
	}
	_, err := r.userCollection(entry.UserID).Doc(entry.ID).Set(ctx, data, firestore.MergeAll)
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

func (r *firestoreRepository) ListByRange(
	ctx context.Context,
	userID string,
	startInclusive, endExclusive time.Time,
	pagination Pagination,
) ([]Entry, PageInfo, error) {

	pageSize := pagination.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 1000 {
		pageSize = 1000
	}

	col := r.userCollection(userID)

	// Base query: filter by not-deleted and the time window on "anchor"
	base := col.
		Where("deleted", "==", false).
		Where("anchor", ">=", startInclusive).
		Where("anchor", "<", endExclusive)

	// Index-friendly ordering: anchor desc + __name__ desc (acts as a stable tiebreaker)
	q := base.
		OrderBy("anchor", firestore.Desc).
		OrderBy(firestore.DocumentID, firestore.Desc).
		Limit(pageSize + 1)

	// Apply cursor if present
	if pagination.Token != "" {
		anc, lastID, ok, err := decodePageToken(pagination.Token)
		if err != nil {
			return nil, PageInfo{}, fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
		if ok {
			q = q.StartAfter(anc, lastID)
		}
	}

	it := q.Documents(ctx)
	defer it.Stop()

	entries := make([]Entry, 0, pageSize+1)
	var last *firestore.DocumentSnapshot

	for {
		doc, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, PageInfo{}, err
		}
		e, err := snapshotToEntry(userID, doc)
		if err != nil {
			return nil, PageInfo{}, err
		}
		entries = append(entries, e)
		last = doc
	}

	// Determine next page token
	hasNext := len(entries) > pageSize
	var nextToken string
	if hasNext {
		entries = entries[:pageSize]
		if last != nil {
			// Use the last *returned* doc as cursor
			ld := last
			// However, if we trimmed, ensure ld corresponds to the last element kept.
			if len(entries) > 0 {
				// We need the snapshot for the last kept doc. Re-read its anchor to build a cursor.
				// Since we still have 'last' from the iterator (which is the last fetched, not necessarily the last kept),
				// safest is to fetch the anchor from the last kept entry and pair it with its docID.
				lastKept := entries[len(entries)-1]
				anchor := lastKept.StartTime
				nextToken = encodePageToken(anchor, lastKept.ID)
			} else {
				// degenerate case; fallback to iterator's last
				anc, _ := ld.DataAt("anchor")
				nextToken = encodePageToken(anc.(time.Time), ld.Ref.ID)
			}
		}
	}

	// Count via aggregation (no full scan)
	totalItems, totalPages, err := r.countAgg(ctx, base, pageSize)
	if err != nil {
		return nil, PageInfo{}, err
	}

	return entries, PageInfo{
		PageSize:   pageSize,
		TotalPages: totalPages,
		TotalItems: totalItems,
		HasNext:    hasNext,
		NextToken:  nextToken,
	}, nil
}

// countAgg uses Firestore aggregation queries to avoid scanning documents client-side.
func (r *firestoreRepository) countAgg(ctx context.Context, base firestore.Query, pageSize int) (int, int, error) {
	agg := base.NewAggregationQuery().WithCount("c")
	res, err := agg.Get(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("count query failed: %w", err)
	}
	// Handle Firestore protobuf value - simplified approach
	var count int64
	if val, ok := res["c"].(*firestorepb.Value); ok {
		// Try to get integer value first, then double value
		integerVal := val.GetIntegerValue()
		doubleVal := val.GetDoubleValue()

		if integerVal != 0 {
			count = integerVal
		} else if doubleVal != 0 {
			count = int64(doubleVal)
		} else {
			// Both are 0, which is valid for empty collections
			count = 0
		}
	} else {
		// Fallback for other types
		switch v := res["c"].(type) {
		case int64:
			count = v
		case int:
			count = int64(v)
		case float64:
			count = int64(v)
		default:
			return 0, 0, fmt.Errorf("unexpected count type: %T", v)
		}
	}
	n := int(count)
	pages := n / pageSize
	if n%pageSize != 0 {
		pages++
	}
	if pages == 0 {
		pages = 1
	}
	return n, pages, nil
}

func snapshotToEntry(userID string, doc *firestore.DocumentSnapshot) (Entry, error) {
	var payload struct {
		ActivityName string    `firestore:"activity_name"`
		TimeElapsed  int       `firestore:"time_elapsed"`
		NumCycle     int       `firestore:"num_cycle"`
		TimeMode     string    `firestore:"time_mode"`
		Category     string    `firestore:"category"`
		Description  string    `firestore:"description"`
		Mood         string    `firestore:"mood"`
		Image        string    `firestore:"image"`
		StartTime    time.Time `firestore:"start_time"`
		EndTime      time.Time `firestore:"end_time"`
		CreatedAt    time.Time `firestore:"created_at"`
		UpdatedAt    time.Time `firestore:"updated_at"`
		DeletedAt    time.Time `firestore:"deleted_at"`
	}
	if err := doc.DataTo(&payload); err != nil {
		return Entry{}, err
	}

	entry := Entry{
		ID:           doc.Ref.ID,
		UserID:       userID,
		ActivityName: payload.ActivityName,
		TimeElapsed:  payload.TimeElapsed,
		NumCycle:     payload.NumCycle,
		TimeMode:     payload.TimeMode,
		Category:     payload.Category,
		Description:  payload.Description,
		Mood:         payload.Mood,
		Image:        payload.Image,
		StartTime:    payload.StartTime,
		EndTime:      payload.EndTime,
		CreatedAt:    payload.CreatedAt,
		UpdatedAt:    payload.UpdatedAt,
	}

	if !payload.DeletedAt.IsZero() {
		entry.DeletedAt = &payload.DeletedAt
	}

	return entry, nil
}
