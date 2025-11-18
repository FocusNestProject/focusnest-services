package user

import (
	"context"
	"errors"
	"testing"
)

type fakeRepo struct {
	getProfileFn     func(context.Context, string) (*Profile, error)
	upsertProfileFn  func(context.Context, string, ProfileUpdateInput) (*Profile, error)
	getProfileMetaFn func(context.Context, string) (ProfileMetadata, error)
}

func (f *fakeRepo) GetProfile(ctx context.Context, userID string) (*Profile, error) {
	if f.getProfileFn != nil {
		return f.getProfileFn(ctx, userID)
	}
	return nil, errors.New("getProfileFn not provided")
}

func (f *fakeRepo) UpsertProfile(ctx context.Context, userID string, updates ProfileUpdateInput) (*Profile, error) {
	if f.upsertProfileFn != nil {
		return f.upsertProfileFn(ctx, userID, updates)
	}
	return nil, errors.New("upsertProfileFn not provided")
}

func (f *fakeRepo) GetProfileMetadata(ctx context.Context, userID string) (ProfileMetadata, error) {
	if f.getProfileMetaFn != nil {
		return f.getProfileMetaFn(ctx, userID)
	}
	return ProfileMetadata{}, errors.New("getProfileMetaFn not provided")
}

func TestServiceGetProfile_DefaultsWhenMissing(t *testing.T) {
	repo := &fakeRepo{
		getProfileFn: func(ctx context.Context, userID string) (*Profile, error) {
			return nil, ErrProfileNotFound
		},
		getProfileMetaFn: func(ctx context.Context, userID string) (ProfileMetadata, error) {
			return ProfileMetadata{LongestStreak: 5, TotalSessions: 20}, nil
		},
	}

	svc := NewService(repo)
	resp, err := svc.GetProfile(context.Background(), "user-123")
	if err != nil {
		t.Fatalf("GetProfile returned error: %v", err)
	}

	if resp.UserID != "user-123" {
		t.Fatalf("expected user id to be propagated")
	}
	if resp.Metadata.LongestStreak != 5 || resp.Metadata.TotalSessions != 20 {
		t.Fatalf("metadata not propagated: %+v", resp.Metadata)
	}
}

func TestServiceUpdateProfile_ConcurrentFetches(t *testing.T) {
	profile := &Profile{UserID: "user-abc", FullName: "Focus Nest"}
	repo := &fakeRepo{
		upsertProfileFn: func(ctx context.Context, userID string, updates ProfileUpdateInput) (*Profile, error) {
			return profile, nil
		},
		getProfileMetaFn: func(ctx context.Context, userID string) (ProfileMetadata, error) {
			return ProfileMetadata{TotalProductivities: 3}, nil
		},
	}

	svc := NewService(repo)
	resp, err := svc.UpdateProfile(context.Background(), profile.UserID, ProfileUpdateInput{})
	if err != nil {
		t.Fatalf("UpdateProfile returned error: %v", err)
	}

	if resp.FullName != profile.FullName {
		t.Fatalf("expected profile fields to be preserved")
	}
	if resp.Metadata.TotalProductivities != 3 {
		t.Fatalf("expected metadata to be carried over")
	}
}

func TestServiceUpdateProfile_PropagatesErrors(t *testing.T) {
	wantErr := errors.New("boom")
	repo := &fakeRepo{
		upsertProfileFn: func(ctx context.Context, userID string, updates ProfileUpdateInput) (*Profile, error) {
			return nil, wantErr
		},
		getProfileMetaFn: func(ctx context.Context, userID string) (ProfileMetadata, error) {
			return ProfileMetadata{}, nil
		},
	}

	svc := NewService(repo)
	if _, err := svc.UpdateProfile(context.Background(), "user-err", ProfileUpdateInput{}); !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}
