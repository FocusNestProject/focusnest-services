package user

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"
)

type service struct {
	repo Repository
}

// NewService creates a new user service
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) GetProfile(ctx context.Context, userID string) (*ProfileResponse, error) {
	var (
		profile  *Profile
		metadata ProfileMetadata
	)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		p, err := s.repo.GetProfile(ctx, userID)
		if err != nil {
			return err
		}
		profile = p
		return nil
	})

	g.Go(func() error {
		m, err := s.repo.GetProfileMetadata(ctx, userID)
		if err != nil {
			return err
		}
		metadata = m
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return buildProfileResponse(profile, metadata), nil
}

func (s *service) UpdateProfile(ctx context.Context, userID string, updates ProfileUpdateInput) (*ProfileResponse, error) {
	var (
		updated  *Profile
		metadata ProfileMetadata
	)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		p, err := s.repo.UpsertProfile(ctx, userID, updates)
		if err != nil {
			return err
		}
		updated = p
		return nil
	})

	g.Go(func() error {
		m, err := s.repo.GetProfileMetadata(ctx, userID)
		if err != nil {
			return err
		}
		metadata = m
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return buildProfileResponse(updated, metadata), nil
}

func (s *service) ListChallenges(_ context.Context) ([]ChallengeDefinition, error) {
	// Static for now; returning a copy keeps callers from mutating.
	defs := challengeDefinitions()
	out := make([]ChallengeDefinition, len(defs))
	copy(out, defs)
	return out, nil
}

func (s *service) GetChallengesMe(ctx context.Context, userID string) (*ChallengesMeResponse, error) {
	if userID == "" {
		return nil, fmt.Errorf("missing user id")
	}

	profile, err := s.repo.GetProfile(ctx, userID)
	if err != nil {
		return nil, err
	}

	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	today := truncateToDay(now)

	// Look back enough days to compute streaks safely.
	// For now 45 days is more than enough for the current challenge set.
	start := today.AddDate(0, 0, -45)
	end := today.AddDate(0, 0, 1) // [start, end)

	minsByDate, err := s.repo.GetDailyMinutesByDate(ctx, userID, start, end)
	if err != nil {
		return nil, err
	}

	// Get week start (Monday) for weekly challenges
	weekStart := getWeekStart(today)

	defs := challengeDefinitions()
	statuses := make([]ChallengeStatus, 0, len(defs))
	for _, def := range defs {
		var progress ChallengeProgress
		var completed bool

		switch def.RuleType {
		case ChallengeRuleDailyMinutesStreak:
			progress = computeDailyMinutesStreakProgress(def, minsByDate, today)
			completed = progress.CurrentStreakDays >= def.ConsecutiveDays

		case ChallengeRuleWeeklyShares:
			shareCount, err := s.repo.GetWeeklyShareCount(ctx, userID, weekStart)
			if err != nil {
				return nil, err
			}
			progress = computeWeeklySharesProgress(def, shareCount)
			completed = shareCount >= def.TargetCount

		case ChallengeRuleStreakMilestone:
			currentStreak, err := s.repo.GetCurrentStreak(ctx, userID)
			if err != nil {
				return nil, err
			}
			progress = computeStreakMilestoneProgress(def, currentStreak)
			completed = currentStreak >= def.TargetStreak

		case ChallengeRuleCyclesAndMindfulness:
			totalCycles, err := s.repo.GetTotalCycles(ctx, userID)
			if err != nil {
				return nil, err
			}
			mindfulnessMinutes, err := s.repo.GetTotalMindfulnessMinutes(ctx, userID)
			if err != nil {
				return nil, err
			}
			progress = computeCyclesAndMindfulnessProgress(def, totalCycles, mindfulnessMinutes)
			completed = totalCycles >= def.TargetCycles && mindfulnessMinutes >= def.TargetMindfulnessMinutes

		default:
			// Unknown challenge type; ignore for now.
			continue
		}

		claimed, err := s.repo.IsChallengeClaimed(ctx, userID, def.ID)
		if err != nil {
			return nil, err
		}

		statuses = append(statuses, ChallengeStatus{
			Challenge: def,
			Progress:  progress,
			Completed: completed,
			Claimed:   claimed,
		})
	}

	return &ChallengesMeResponse{
		PointsTotal: profile.PointsTotal,
		Badges:      badgesForPoints(profile.PointsTotal),
		Challenges:  statuses,
	}, nil
}

func (s *service) ClaimChallenge(ctx context.Context, userID, challengeID string) (*ClaimChallengeResponse, error) {
	if userID == "" {
		return nil, fmt.Errorf("missing user id")
	}
	if challengeID == "" {
		return nil, fmt.Errorf("missing challenge id")
	}

	// Validate challenge exists and fetch its definition.
	var def *ChallengeDefinition
	for _, c := range challengeDefinitions() {
		if c.ID == challengeID {
			copy := c
			def = &copy
			break
		}
	}
	if def == nil {
		return nil, fmt.Errorf("unknown challenge")
	}

	// Ensure it is completed before awarding points.
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	today := truncateToDay(now)

	eligible := false

	switch def.RuleType {
	case ChallengeRuleDailyMinutesStreak:
		start := today.AddDate(0, 0, -45)
		end := today.AddDate(0, 0, 1)
		minsByDate, err := s.repo.GetDailyMinutesByDate(ctx, userID, start, end)
		if err != nil {
			return nil, err
		}
		progress := computeDailyMinutesStreakProgress(*def, minsByDate, today)
		eligible = progress.CurrentStreakDays >= def.ConsecutiveDays

	case ChallengeRuleWeeklyShares:
		weekStart := getWeekStart(today)
		shareCount, err := s.repo.GetWeeklyShareCount(ctx, userID, weekStart)
		if err != nil {
			return nil, err
		}
		eligible = shareCount >= def.TargetCount

	case ChallengeRuleStreakMilestone:
		currentStreak, err := s.repo.GetCurrentStreak(ctx, userID)
		if err != nil {
			return nil, err
		}
		eligible = currentStreak >= def.TargetStreak

	case ChallengeRuleCyclesAndMindfulness:
		totalCycles, err := s.repo.GetTotalCycles(ctx, userID)
		if err != nil {
			return nil, err
		}
		mindfulnessMinutes, err := s.repo.GetTotalMindfulnessMinutes(ctx, userID)
		if err != nil {
			return nil, err
		}
		eligible = totalCycles >= def.TargetCycles && mindfulnessMinutes >= def.TargetMindfulnessMinutes

	default:
		return nil, fmt.Errorf("unsupported challenge type")
	}

	if !eligible {
		// Not eligible yet; return current points for UI.
		profile, _ := s.repo.GetProfile(ctx, userID)
		return &ClaimChallengeResponse{
			ChallengeID:    challengeID,
			Claimed:        false,
			AlreadyClaimed: false,
			PointsAwarded:  0,
			PointsTotal:    func() int { if profile != nil { return profile.PointsTotal }; return 0 }(),
		}, nil
	}

	newTotal, claimedAt, already, err := s.repo.ClaimChallenge(ctx, userID, challengeID, def.RewardPoints)
	if err != nil {
		return nil, err
	}

	resp := &ClaimChallengeResponse{
		ChallengeID:    challengeID,
		Claimed:        !already,
		AlreadyClaimed: already,
		PointsAwarded:  0,
		PointsTotal:    newTotal,
	}
	if already {
		resp.PointsAwarded = 0
	} else {
		resp.PointsAwarded = def.RewardPoints
		resp.ClaimedAt = claimedAt
	}
	return resp, nil
}

func (s *service) RecordShare(ctx context.Context, userID string, shareType string) error {
	if userID == "" {
		return fmt.Errorf("missing user id")
	}
	if shareType == "" {
		shareType = "recap" // default share type
	}
	return s.repo.RecordShare(ctx, userID, shareType)
}

func (s *service) RecordMindfulness(ctx context.Context, userID string, minutes int) error {
	if userID == "" {
		return fmt.Errorf("missing user id")
	}
	if minutes <= 0 {
		return fmt.Errorf("minutes must be positive")
	}
	return s.repo.RecordMindfulness(ctx, userID, minutes)
}

func defaultProfile(userID string) *Profile {
	return &Profile{UserID: userID}
}

func buildProfileResponse(profile *Profile, metadata ProfileMetadata) *ProfileResponse {
	return &ProfileResponse{
		UserID:          profile.UserID,
		Bio:             profile.Bio,
		Birthdate:       profile.Birthdate,
		PointsTotal:     profile.PointsTotal,
		ProfileMetadata: metadata,
		CreatedAt:       profile.CreatedAt,
		UpdatedAt:       profile.UpdatedAt,
	}
}

func truncateToDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func computeDailyMinutesStreakProgress(def ChallengeDefinition, minsByDate map[string]int, today time.Time) ChallengeProgress {
	minPerDay := def.MinMinutesPerDay
	targetDays := def.ConsecutiveDays

	// Current streak ending today (in local time).
	streak := 0
	for i := 0; i < 90; i++ { // safety cap
		day := today.AddDate(0, 0, -i)
		key := day.Format("2006-01-02")
		mins := minsByDate[key]
		if mins >= minPerDay {
			streak++
			continue
		}
		break
	}

	minsToday := minsByDate[today.Format("2006-01-02")]
	percent := (streak * 100) / targetDays
	if percent > 100 {
		percent = 100
	}

	return ChallengeProgress{
		CurrentStreakDays: streak,
		TargetStreakDays:  targetDays,
		MinMinutesPerDay:  minPerDay,
		MinutesToday:      minsToday,
		ProgressPercent:   percent,
	}
}

func computeWeeklySharesProgress(def ChallengeDefinition, shareCount int) ChallengeProgress {
	target := def.TargetCount
	percent := (shareCount * 100) / target
	if percent > 100 {
		percent = 100
	}

	return ChallengeProgress{
		CurrentCount:    shareCount,
		TargetCount:     target,
		ProgressPercent: percent,
	}
}

func computeStreakMilestoneProgress(def ChallengeDefinition, currentStreak int) ChallengeProgress {
	target := def.TargetStreak
	percent := (currentStreak * 100) / target
	if percent > 100 {
		percent = 100
	}

	return ChallengeProgress{
		CurrentStreak:   currentStreak,
		TargetStreak:    target,
		ProgressPercent: percent,
	}
}

// getWeekStart returns the Monday of the week containing the given date.
func getWeekStart(t time.Time) time.Time {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday becomes 7
	}
	// Monday is day 1, so subtract (weekday - 1) days
	return truncateToDay(t.AddDate(0, 0, -(weekday - 1)))
}

func computeCyclesAndMindfulnessProgress(def ChallengeDefinition, currentCycles, currentMindfulness int) ChallengeProgress {
	targetCycles := def.TargetCycles
	targetMindfulness := def.TargetMindfulnessMinutes

	// Calculate combined progress (average of both)
	cyclePercent := 0
	if targetCycles > 0 {
		cyclePercent = (currentCycles * 100) / targetCycles
		if cyclePercent > 100 {
			cyclePercent = 100
		}
	}

	mindfulnessPercent := 0
	if targetMindfulness > 0 {
		mindfulnessPercent = (currentMindfulness * 100) / targetMindfulness
		if mindfulnessPercent > 100 {
			mindfulnessPercent = 100
		}
	}

	// Overall progress is the average of both components
	percent := (cyclePercent + mindfulnessPercent) / 2

	return ChallengeProgress{
		CurrentCycles:             currentCycles,
		TargetCycles:              targetCycles,
		CurrentMindfulnessMinutes: currentMindfulness,
		TargetMindfulnessMinutes:  targetMindfulness,
		ProgressPercent:           percent,
	}
}
