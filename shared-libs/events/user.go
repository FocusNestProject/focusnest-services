package events

import "time"

// UserSynced describes the payload produced when a Clerk user is synchronized into Firestore.
type UserSynced struct {
	UserID      string    `json:"userId"`
	Email       string    `json:"email"`
	DisplayName string    `json:"displayName"`
	Roles       []string  `json:"roles"`
	SyncedAt    time.Time `json:"syncedAt"`
}

// UserDeleted is emitted when a user is removed from the system.
type UserDeleted struct {
	UserID   string    `json:"userId"`
	DeletedAt time.Time `json:"deletedAt"`
}
