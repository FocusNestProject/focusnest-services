package chatbot

import (
	"context"
	"fmt"
	"sort"
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

func (r *firestoreRepository) CreateSession(session *ChatbotSession) error {
	ctx := context.Background()
	_, err := r.client.Collection("chat_sessions").Doc(session.ID).Set(ctx, session)
	return err
}

func (r *firestoreRepository) GetSessions(userID string) ([]*ChatbotSession, error) {
	ctx := context.Background()
	// Note: Firestore requires composite index for multiple orderBy fields
	// For now, we'll sort pinned sessions in memory after fetching
	iter := r.client.Collection("chat_sessions").
		Where("user_id", "==", userID).
		OrderBy("updated_at", firestore.Desc).
		Documents(ctx)

	var sessions []*ChatbotSession
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		var session ChatbotSession
		if err := doc.DataTo(&session); err != nil {
			return nil, fmt.Errorf("unmarshal session: %w", err)
		}
		session.ID = doc.Ref.ID
		// Default pinned to false if not set (for backward compatibility)
		if pinnedVal, ok := doc.Data()["pinned"]; !ok {
			session.Pinned = false
		} else if pinned, ok := pinnedVal.(bool); ok {
			session.Pinned = pinned
		} else {
			session.Pinned = false
		}
		sessions = append(sessions, &session)
	}

	// Sort: pinned sessions first, then by updated_at descending
	// Backend already returns sessions sorted by updated_at desc, so we just need to move pinned to front
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].Pinned != sessions[j].Pinned {
			return sessions[i].Pinned // pinned sessions come first
		}
		// Within same pinned status, maintain updated_at order (already sorted desc)
		return false
	})

	return sessions, nil
}

func (r *firestoreRepository) CreateMessage(message *ChatMessage) error {
	ctx := context.Background()
	_, err := r.client.Collection("chat_messages").Doc(message.ID).Set(ctx, message)
	return err
}

func (r *firestoreRepository) GetSession(sessionID string) (*ChatbotSession, error) {
	ctx := context.Background()
	doc, err := r.client.Collection("chat_sessions").Doc(sessionID).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	var session ChatbotSession
	if err := doc.DataTo(&session); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	session.ID = doc.Ref.ID
	return &session, nil
}

func (r *firestoreRepository) GetMessages(sessionID string) ([]*ChatMessage, error) {
	ctx := context.Background()
	iter := r.client.Collection("chat_messages").
		Where("session_id", "==", sessionID).
		OrderBy("created_at", firestore.Asc).
		Documents(ctx)

	var messages []*ChatMessage
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		var message ChatMessage
		if err := doc.DataTo(&message); err != nil {
			return nil, fmt.Errorf("unmarshal message: %w", err)
		}
		message.ID = doc.Ref.ID
		messages = append(messages, &message)
	}

	return messages, nil
}

func (r *firestoreRepository) UpdateSessionTimestamp(sessionID string, updatedAt time.Time) error {
	ctx := context.Background()
	_, err := r.client.Collection("chat_sessions").Doc(sessionID).Update(ctx, []firestore.Update{
		{Path: "updated_at", Value: updatedAt},
	})
	return err
}

func (r *firestoreRepository) UpdateSessionTitle(sessionID string, title string, updatedAt time.Time) error {
	ctx := context.Background()
	_, err := r.client.Collection("chat_sessions").Doc(sessionID).Update(ctx, []firestore.Update{
		{Path: "title", Value: title},
		{Path: "updated_at", Value: updatedAt},
	})
	return err
}

func (r *firestoreRepository) UpdateSessionPinned(sessionID string, pinned bool, updatedAt time.Time) error {
	ctx := context.Background()
	_, err := r.client.Collection("chat_sessions").Doc(sessionID).Update(ctx, []firestore.Update{
		{Path: "pinned", Value: pinned},
		{Path: "updated_at", Value: updatedAt},
	})
	return err
}

func (r *firestoreRepository) DeleteSession(sessionID string) error {
	ctx := context.Background()
	_, err := r.client.Collection("chat_sessions").Doc(sessionID).Delete(ctx)
	return err
}

func (r *firestoreRepository) DeleteMessages(sessionID string) error {
	ctx := context.Background()
	iter := r.client.Collection("chat_messages").Where("session_id", "==", sessionID).Documents(ctx)
	batch := r.client.Batch()
	count := 0
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		batch.Delete(doc.Ref)
		count++
		if count == 400 {
			if _, err := batch.Commit(ctx); err != nil {
				return err
			}
			batch = r.client.Batch()
			count = 0
		}
	}
	if count > 0 {
		_, err := batch.Commit(ctx)
		return err
	}
	return nil
}

func (r *firestoreRepository) GetRecentMessages(sessionID string, limit int) ([]*ChatMessage, error) {
	ctx := context.Background()
	if limit <= 0 {
		limit = 1
	}
	iter := r.client.Collection("chat_messages").
		Where("session_id", "==", sessionID).
		OrderBy("created_at", firestore.Desc).
		Limit(limit).
		Documents(ctx)

	var reversed []*ChatMessage
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		var message ChatMessage
		if err := doc.DataTo(&message); err != nil {
			return nil, fmt.Errorf("unmarshal message: %w", err)
		}
		message.ID = doc.Ref.ID
		reversed = append(reversed, &message)
	}

	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}
	return reversed, nil
}
