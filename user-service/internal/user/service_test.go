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
	getProfileMetaFn             func(context.Context, string, *time.Location) (ProfileMetadata, error)
	getDailyMinutesByDateFn      func(context.Context, string, time.Time, time.Time, *time.Location) (map[string]int, error)
	listChallengesFn             func(context.Context) ([]ChallengeDefinition, error)
	createChallengeFn            func(context.Context, ChallengeDefinition) error
	isChallengeClaimedFn         func(context.Context, string, string) (bool, error)
	claimChallengeFn             func(context.Context, string, string, int) (int, time.Time, bool, error)
	getWeeklyShareCountFn        func(context.Context, string, time.Time) (int, error)
	recordShareFn                func(context.Context, string, string) error
	getCurrentStreakFn           func(context.Context, string, *time.Location) (int, error)
	getCyclesByDateFn            func(context.Context, string, time.Time, *time.Location) (int, error)
	getMindfulnessMinutesByDateFn func(context.Context, string, time.Time, *time.Location) (int, error)
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

func (f *fakeRepo) GetProfileMetadata(ctx context.Context, userID string, loc *time.Location) (ProfileMetadata, error) {
	if f.getProfileMetaFn != nil {
		return f.getProfileMetaFn(ctx, userID, loc)
	}
	return ProfileMetadata{}, errors.New("getProfileMetaFn not provided")
}

func (f *fakeRepo) GetDailyMinutesByDate(ctx context.Context, userID string, startDate, endDate time.Time, loc *time.Location) (map[string]int, error) {
	if f.getDailyMinutesByDateFn != nil {
		return f.getDailyMinutesByDateFn(ctx, userID, startDate, endDate, loc)
	}
	return map[string]int{}, nil
}

func (f *fakeRepo) ListChallenges(ctx context.Context) ([]ChallengeDefinition, error) {
	if f.listChallengesFn != nil {
		return f.listChallengesFn(ctx)
	}
	return []ChallengeDefinition{}, nil
}

func (f *fakeRepo) CreateChallenge(ctx context.Context, def ChallengeDefinition) error {
	if f.createChallengeFn != nil {
		return f.createChallengeFn(ctx, def)
	}
	return nil
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

func (f *fakeRepo) GetCurrentStreak(ctx context.Context, userID string, loc *time.Location) (int, error) {
	if f.getCurrentStreakFn != nil {
		return f.getCurrentStreakFn(ctx, userID, loc)
	}
	return 0, nil
}

func (f *fakeRepo) GetCyclesByDate(ctx context.Context, userID string, date time.Time, loc *time.Location) (int, error) {
	if f.getCyclesByDateFn != nil {
		return f.getCyclesByDateFn(ctx, userID, date, loc)
	}
	return 0, nil
}

func (f *fakeRepo) GetMindfulnessMinutesByDate(ctx context.Context, userID string, date time.Time, loc *time.Location) (int, error) {
	if f.getMindfulnessMinutesByDateFn != nil {
		return f.getMindfulnessMinutesByDateFn(ctx, userID, date, loc)
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
		getProfileMetaFn: func(ctx context.Context, userID string, loc *time.Location) (ProfileMetadata, error) {
			return ProfileMetadata{LongestStreak: 5, TotalSessions: 20, TotalCycle: 7}, nil
		},
	}

	svc := NewService(repo)
	resp, err := svc.GetProfile(context.Background(), "user-123", "UTC")
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
		getProfileMetaFn: func(ctx context.Context, userID string, loc *time.Location) (ProfileMetadata, error) {
			return ProfileMetadata{TotalProductivities: 3, TotalCycle: 9}, nil
		},
	}

	svc := NewService(repo)
	resp, err := svc.UpdateProfile(context.Background(), profile.UserID, ProfileUpdateInput{}, "UTC")
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

func TestServiceGetChallengesMe_ProgressLogic(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")
	now := time.Now().In(loc)
	todayStr := now.Format("2006-01-02")
	yesterdayStr := now.AddDate(0, 0, -1).Format("2006-01-02")
	dayBeforeStr := now.AddDate(0, 0, -2).Format("2006-01-02")

	repo := &fakeRepo{
		getProfileFn: func(ctx context.Context, userID string) (*Profile, error) {
			return &Profile{UserID: userID, PointsTotal: 100}, nil
		},
		getProfileMetaFn: func(ctx context.Context, userID string, loc *time.Location) (ProfileMetadata, error) {
			return ProfileMetadata{LongestStreak: 12}, nil
		},
		getDailyMinutesByDateFn: func(ctx context.Context, userID string, start, end time.Time, loc *time.Location) (map[string]int, error) {
			return map[string]int{
				todayStr:     130,
				yesterdayStr: 120,
				dayBeforeStr: 125,
			}, nil
		},
		listChallengesFn: func(ctx context.Context) ([]ChallengeDefinition, error) {
			return []ChallengeDefinition{
				{
					ID:               "focus_2h_3days",
					RuleType:         ChallengeRuleDailyMinutesStreak,
					MinMinutesPerDay: 120,
					ConsecutiveDays:  3,
					RewardPoints:     50,
				},
				{
					ID:           "streak_10_days",
					RuleType:     ChallengeRuleStreakMilestone,
					TargetStreak: 10,
					RewardPoints: 50,
				},
			}, nil
		},
		isChallengeClaimedFn: func(ctx context.Context, userID, challengeID string) (bool, error) {
			return false, nil
		},
		claimChallengeFn: func(ctx context.Context, userID, challengeID string, points int) (int, time.Time, bool, error) {
			return 150, time.Now(), false, nil
		},
	}

	svc := NewService(repo)
	resp, err := svc.GetChallengesMe(context.Background(), "user-1", "UTC")
	if err != nil {
		t.Fatalf("GetChallengesMe failed: %v", err)
	}

	// Check focus_2h_3days (should be completed based on the 3 days of 120+ mins)
	foundFocus := false
	for _, c := range resp.Challenges {
		if c.Challenge.ID == "focus_2h_3days" {
			foundFocus = true
			if !c.Completed {
				t.Errorf("expected focus_2h_3days to be completed")
			}
			if c.Progress.CurrentStreakDays < 3 {
				t.Errorf("expected streak to be at least 3, got %d", c.Progress.CurrentStreakDays)
			}
		}
		if c.Challenge.ID == "streak_10_days" {
			if !c.Completed {
				t.Errorf("expected streak_10_days to be completed (longest streak 12 >= 10)")
			}
		}
	}
	if !foundFocus {
		t.Fatal("focus_2h_3days challenge not found in response")
	}
}

func TestServiceGetChallengesMe_ComplexRules(t *testing.T) {
	repo := &fakeRepo{
		getProfileFn: func(ctx context.Context, userID string) (*Profile, error) {
			return &Profile{UserID: userID, PointsTotal: 100}, nil
		},
		getProfileMetaFn: func(ctx context.Context, userID string, loc *time.Location) (ProfileMetadata, error) {
			return ProfileMetadata{}, nil
		},
		getDailyMinutesByDateFn: func(ctx context.Context, userID string, start, end time.Time, loc *time.Location) (map[string]int, error) {
			return map[string]int{}, nil
		},
		getWeeklyShareCountFn: func(ctx context.Context, userID string, weekStart time.Time) (int, error) {
			return 3, nil // Meets target
		},
		getCyclesByDateFn: func(ctx context.Context, userID string, date time.Time, loc *time.Location) (int, error) {
			return 4, nil // Meets target
		},
		getMindfulnessMinutesByDateFn: func(ctx context.Context, userID string, date time.Time, loc *time.Location) (int, error) {
			return 2, nil // Meets target
		},
		listChallengesFn: func(ctx context.Context) ([]ChallengeDefinition, error) {
			return []ChallengeDefinition{
				{
					ID:           "share_recap_3x_weekly",
					RuleType:     ChallengeRuleWeeklyShares,
					TargetCount:  3,
					RewardPoints: 50,
				},
				{
					ID:                       "cycles_and_mindfulness",
					RuleType:                 ChallengeRuleCyclesAndMindfulness,
					TargetCycles:             4,
					TargetMindfulnessMinutes: 2,
					RewardPoints:             50,
				},
			}, nil
		},
		isChallengeClaimedFn: func(ctx context.Context, userID, challengeID string) (bool, error) {
			return false, nil
		},
		claimChallengeFn: func(ctx context.Context, userID, challengeID string, points int) (int, time.Time, bool, error) {
			return 150, time.Now(), false, nil
		},
	}

	svc := NewService(repo)
	resp, err := svc.GetChallengesMe(context.Background(), "user-1", "UTC")
	if err != nil {
		t.Fatalf("GetChallengesMe failed: %v", err)
	}

	for _, c := range resp.Challenges {
		if c.Challenge.ID == "share_recap_3x_weekly" {
			if !c.Completed {
				t.Errorf("expected share_recap_3x_weekly to be completed")
			}
			if c.Progress.CurrentCount != 3 {
				t.Errorf("expected count 3, got %d", c.Progress.CurrentCount)
			}
		}
		if c.Challenge.ID == "cycles_and_mindfulness" {
			if !c.Completed {
				t.Errorf("expected cycles_and_mindfulness to be completed")
			}
			if c.Progress.CurrentCycles != 4 || c.Progress.CurrentMindfulnessMinutes != 2 {
				t.Errorf("expected 4 cycles and 2 mins, got %d and %d", c.Progress.CurrentCycles, c.Progress.CurrentMindfulnessMinutes)
			}
		}
	}
}
