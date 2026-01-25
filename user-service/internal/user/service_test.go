package user

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeRepo struct {
	getProfileFn                 func(context.Context, string) (*Profile, error)
	upsertProfileFn              func(context.Context, string, ProfileUpdateInput) (*Profile, error)
	getProfileMetaFn             func(context.Context, string) (ProfileMetadata, error)
	getDailyMinutesByDateFn      func(context.Context, string, time.Time, time.Time) (map[string]int, error)
	isChallengeClaimedFn         func(context.Context, string, string) (bool, error)
	claimChallengeFn             func(context.Context, string, string, int) (int, time.Time, bool, error)
	getWeeklyShareCountFn        func(context.Context, string, time.Time) (int, error)
	recordShareFn                func(context.Context, string, string) error
	getCurrentStreakFn           func(context.Context, string) (int, error)
	getTotalCyclesFn             func(context.Context, string) (int, error)
	getTotalMindfulnessMinutesFn func(context.Context, string) (int, error)
	recordMindfulnessFn          func(context.Context, string, int) error
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

func (f *fakeRepo) GetDailyMinutesByDate(ctx context.Context, userID string, startDate, endDate time.Time) (map[string]int, error) {
	if f.getDailyMinutesByDateFn != nil {
		return f.getDailyMinutesByDateFn(ctx, userID, startDate, endDate)
	}
	return map[string]int{}, nil
}

func (f *fakeRepo) IsChallengeClaimed(ctx context.Context, userID, challengeID string) (bool, error) {
	if f.isChallengeClaimedFn != nil {
		return f.isChallengeClaimedFn(ctx, userID, challengeID)
	}
	return false, nil
}

func (f *fakeRepo) ClaimChallenge(ctx context.Context, userID, challengeID string, points int) (int, time.Time, bool, error) {
	if f.claimChallengeFn != nil {
		return f.claimChallengeFn(ctx, userID, challengeID, points)
	}
	return 0, time.Time{}, false, errors.New("claimChallengeFn not provided")
}

func (f *fakeRepo) GetWeeklyShareCount(ctx context.Context, userID string, weekStart time.Time) (int, error) {
	if f.getWeeklyShareCountFn != nil {
		return f.getWeeklyShareCountFn(ctx, userID, weekStart)
	}
	return 0, nil
}

func (f *fakeRepo) RecordShare(ctx context.Context, userID string, shareType string) error {
	if f.recordShareFn != nil {
		return f.recordShareFn(ctx, userID, shareType)
	}
	return nil
}

func (f *fakeRepo) GetCurrentStreak(ctx context.Context, userID string) (int, error) {
	if f.getCurrentStreakFn != nil {
		return f.getCurrentStreakFn(ctx, userID)
	}
	return 0, nil
}

func (f *fakeRepo) GetTodayCycles(ctx context.Context, userID string) (int, error) {
	if f.getTotalCyclesFn != nil {
		return f.getTotalCyclesFn(ctx, userID)
	}
	return 0, nil
}

func (f *fakeRepo) GetTodayMindfulnessMinutes(ctx context.Context, userID string) (int, error) {
	if f.getTotalMindfulnessMinutesFn != nil {
		return f.getTotalMindfulnessMinutesFn(ctx, userID)
	}
	return 0, nil
}

func (f *fakeRepo) RecordMindfulness(ctx context.Context, userID string, minutes int) error {
	if f.recordMindfulnessFn != nil {
		return f.recordMindfulnessFn(ctx, userID, minutes)
	}
	return nil
}

func TestServiceGetProfile_DefaultsWhenMissing(t *testing.T) {
	repo := &fakeRepo{
		getProfileFn: func(ctx context.Context, userID string) (*Profile, error) {
			return defaultProfile(userID), nil
		},
		getProfileMetaFn: func(ctx context.Context, userID string) (ProfileMetadata, error) {
			return ProfileMetadata{LongestStreak: 5, TotalSessions: 20, TotalCycle: 7}, nil
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
	if resp.LongestStreak != 5 || resp.TotalSessions != 20 || resp.TotalCycle != 7 {
		t.Fatalf("metadata not propagated: %+v", resp.ProfileMetadata)
	}
}

func TestServiceUpdateProfile_ConcurrentFetches(t *testing.T) {
	profile := &Profile{UserID: "user-abc", Bio: "Focus Nest"}
	repo := &fakeRepo{
		upsertProfileFn: func(ctx context.Context, userID string, updates ProfileUpdateInput) (*Profile, error) {
			return profile, nil
		},
		getProfileMetaFn: func(ctx context.Context, userID string) (ProfileMetadata, error) {
			return ProfileMetadata{TotalProductivities: 3, TotalCycle: 9}, nil
		},
	}

	svc := NewService(repo)
	resp, err := svc.UpdateProfile(context.Background(), profile.UserID, ProfileUpdateInput{})
	if err != nil {
		t.Fatalf("UpdateProfile returned error: %v", err)
	}

	if resp.Bio != profile.Bio {
		t.Fatalf("expected profile fields to be preserved")
	}
	if resp.TotalProductivities != 3 || resp.TotalCycle != 9 {
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
