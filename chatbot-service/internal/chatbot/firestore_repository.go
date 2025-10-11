package chatbot

import (
	"context"
	"fmt"

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

func (r *firestoreRepository) CreateSession(session *ChatbotSession) error {
	ctx := context.Background()
	_, err := r.client.Collection("chat_sessions").Doc(session.ID).Set(ctx, session)
	return err
}

func (r *firestoreRepository) GetSessions(userID string) ([]*ChatbotSession, error) {
	ctx := context.Background()
	iter := r.client.Collection("chat_sessions").
		Where("user_id", "==", userID).
		OrderBy("created_at", firestore.Desc).
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
		sessions = append(sessions, &session)
	}

	return sessions, nil
}

func (r *firestoreRepository) CreateMessage(message *ChatMessage) error {
	ctx := context.Background()
	_, err := r.client.Collection("chat_messages").Doc(message.ID).Set(ctx, message)
	return err
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
