package user

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

func resolveLocation(tz string) *time.Location {
	tz = strings.TrimSpace(tz)
	if tz == "" {
		loc, err := time.LoadLocation("Asia/Jakarta")
		if err != nil {
			return time.UTC
		}
		return loc
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc, _ = time.LoadLocation("Asia/Jakarta")
		if loc == nil {
			return time.UTC
		}
		return loc
	}
	return loc
}

type service struct {
	repo Repository
}

// NewService creates a new user service
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) GetProfile(ctx context.Context, userID string, timezone string) (*ProfileResponse, error) {
	var (
		profile  *Profile
		metadata ProfileMetadata
	)

	loc := resolveLocation(timezone)
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
		m, err := s.repo.GetProfileMetadata(ctx, userID, loc)
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

func (s *service) UpdateProfile(ctx context.Context, userID string, updates ProfileUpdateInput, timezone string) (*ProfileResponse, error) {
	var (
		updated  *Profile
		metadata ProfileMetadata
	)

	loc := resolveLocation(timezone)
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
		m, err := s.repo.GetProfileMetadata(ctx, userID, loc)
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

func (s *service) ListChallenges(ctx context.Context) ([]ChallengeDefinition, error) {
	return s.repo.ListChallenges(ctx)
}

func (s *service) MigrateChallenges(ctx context.Context) error {
	defs := challengeDefinitions()
	for _, def := range defs {
		if err := s.repo.CreateChallenge(ctx, def); err != nil {
			return fmt.Errorf("failed to migrate challenge %s: %w", def.ID, err)
		}
	}
	return nil
}

func (s *service) GetChallengesMe(ctx context.Context, userID string, timezone string) (*ChallengesMeResponse, error) {
	if userID == "" {
		return nil, fmt.Errorf("missing user id")
	}

	profile, err := s.repo.GetProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	loc := resolveLocation(timezone)
	metadata, err := s.repo.GetProfileMetadata(ctx, userID, loc)
	if err != nil {
		return nil, err
	}
	now := time.Now().In(loc)
	today := truncateToDay(now)

	// Look back enough days to compute streaks safely.
	// For now 45 days is more than enough for the current challenge set.
	start := today.AddDate(0, 0, -45)
	end := today.AddDate(0, 0, 1) // [start, end)

	minsByDate, err := s.repo.GetDailyMinutesByDate(ctx, userID, start, end, loc)
	if err != nil {
		return nil, err
	}

	// Get week start (Monday) for weekly challenges
	weekStart := getWeekStart(today)

	defs, err := s.repo.ListChallenges(ctx)
	if err != nil {
		return nil, err
	}

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
			lastWeekCount, err := s.repo.GetWeeklyShareCount(ctx, userID, weekStart.AddDate(0, 0, -7))
			if err != nil {
				return nil, err
			}
			maxCount := shareCount
			if lastWeekCount > maxCount {
				maxCount = lastWeekCount
			}
			progress = computeWeeklySharesProgress(def, maxCount)
			completed = maxCount >= def.TargetCount

		case ChallengeRuleStreakMilestone:
			progress = computeStreakMilestoneProgress(def, metadata.LongestStreak)
			completed = metadata.LongestStreak >= def.TargetStreak

		case ChallengeRuleCyclesAndMindfulness:
			// Allow today or yesterday (to handle users opening the app after midnight)
			todayCycles, err := s.repo.GetCyclesByDate(ctx, userID, today, loc)
			if err != nil {
				return nil, err
			}
			todayMindfulness, err := s.repo.GetMindfulnessMinutesByDate(ctx, userID, today, loc)
			if err != nil {
				return nil, err
			}

			yesterday := today.AddDate(0, 0, -1)
			yesterdayCycles, err := s.repo.GetCyclesByDate(ctx, userID, yesterday, loc)
			if err != nil {
				return nil, err
			}
			yesterdayMindfulness, err := s.repo.GetMindfulnessMinutesByDate(ctx, userID, yesterday, loc)
			if err != nil {
				return nil, err
			}

			// Maximize progress from either today or yesterday
			progressToday := computeCyclesAndMindfulnessProgress(def, todayCycles, todayMindfulness)
			progressYesterday := computeCyclesAndMindfulnessProgress(def, yesterdayCycles, yesterdayMindfulness)

			if progressYesterday.ProgressPercent > progressToday.ProgressPercent {
				progress = progressYesterday
				completed = yesterdayCycles >= def.TargetCycles && yesterdayMindfulness >= def.TargetMindfulnessMinutes
			} else {
				progress = progressToday
				completed = todayCycles >= def.TargetCycles && todayMindfulness >= def.TargetMindfulnessMinutes
			}

		default:
			// Unknown challenge type; ignore for now.
			continue
		}

		claimedAt, err := s.repo.GetChallengeClaimedAt(ctx, userID, def.ID)
		if err != nil {
			return nil, err
		}
		
		claimed := false
		if !claimedAt.IsZero() {
			switch def.RuleType {
			case ChallengeRuleDailyMinutesStreak, ChallengeRuleStreakMilestone:
				claimed = true
			case ChallengeRuleWeeklyShares:
				if claimedAt.After(weekStart) || claimedAt.Equal(weekStart) {
					claimed = true
				}
			case ChallengeRuleCyclesAndMindfulness:
				if claimedAt.After(today) || claimedAt.Equal(today) {
					claimed = true
				}
			default:
				claimed = true
			}
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

func (s *service) ClaimChallenge(ctx context.Context, userID, challengeID string, timezone string) (*ClaimChallengeResponse, error) {
	if userID == "" {
		return nil, fmt.Errorf("missing user id")
	}
	if challengeID == "" {
		return nil, fmt.Errorf("missing challenge id")
	}

	// Validate challenge exists and fetch its definition.
	var def *ChallengeDefinition
	defs, err := s.repo.ListChallenges(ctx)
	if err != nil {
		return nil, err
	}
	
	for _, c := range defs {
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
	loc := resolveLocation(timezone)
	now := time.Now().In(loc)
	today := truncateToDay(now)

	eligible := false

	switch def.RuleType {
	case ChallengeRuleDailyMinutesStreak:
		start := today.AddDate(0, 0, -45)
		end := today.AddDate(0, 0, 1)
		minsByDate, err := s.repo.GetDailyMinutesByDate(ctx, userID, start, end, loc)
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
		lastWeekCount, err := s.repo.GetWeeklyShareCount(ctx, userID, weekStart.AddDate(0, 0, -7))
		if err != nil {
			return nil, err
		}
		maxCount := shareCount
		if lastWeekCount > maxCount {
			maxCount = lastWeekCount
		}
		eligible = maxCount >= def.TargetCount

	case ChallengeRuleStreakMilestone:
		metadata, err := s.repo.GetProfileMetadata(ctx, userID, loc)
		if err != nil {
			return nil, err
		}
		eligible = metadata.LongestStreak >= def.TargetStreak

	case ChallengeRuleCyclesAndMindfulness:
		// Check today
		todayCycles, err := s.repo.GetCyclesByDate(ctx, userID, today, loc)
		if err != nil {
			return nil, err
		}
		todayMindfulness, err := s.repo.GetMindfulnessMinutesByDate(ctx, userID, today, loc)
		if err != nil {
			return nil, err
		}
		eligible = todayCycles >= def.TargetCycles && todayMindfulness >= def.TargetMindfulnessMinutes

		if !eligible {
			// Check yesterday as backup
			yesterday := today.AddDate(0, 0, -1)
			yesterdayCycles, err := s.repo.GetCyclesByDate(ctx, userID, yesterday, loc)
			if err != nil {
				return nil, err
			}
			yesterdayMindfulness, err := s.repo.GetMindfulnessMinutesByDate(ctx, userID, yesterday, loc)
			if err != nil {
				return nil, err
			}
			eligible = yesterdayCycles >= def.TargetCycles && yesterdayMindfulness >= def.TargetMindfulnessMinutes
		}

	default:
		return nil, fmt.Errorf("unsupported challenge type")
	}

	if eligible {
		// Check if it's already claimed for the current period
		claimedAt, err := s.repo.GetChallengeClaimedAt(ctx, userID, challengeID)
		if err != nil {
			return nil, err
		}
		if !claimedAt.IsZero() {
			already := false
			switch def.RuleType {
			case ChallengeRuleDailyMinutesStreak, ChallengeRuleStreakMilestone:
				already = true
			case ChallengeRuleWeeklyShares:
				weekStart := getWeekStart(today)
				if claimedAt.After(weekStart) || claimedAt.Equal(weekStart) {
					already = true
				}
			case ChallengeRuleCyclesAndMindfulness:
				if claimedAt.After(today) || claimedAt.Equal(today) {
					already = true
				}
			default:
				already = true
			}
			if already {
				profile, _ := s.repo.GetProfile(ctx, userID)
				return &ClaimChallengeResponse{
					ChallengeID:    challengeID,
					Claimed:        false,
					AlreadyClaimed: true,
					PointsAwarded:  0,
					PointsTotal:    func() int { if profile != nil { return profile.PointsTotal }; return 0 }(),
				}, nil
			}
		}
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

	// 1. Calculate maxStreak to see if they completed it
	maxStreak := 0
	tempStreak := 0
	for i := 45; i >= 0; i-- { // iterate chronologically
		day := today.AddDate(0, 0, -i)
		key := day.Format("2006-01-02")
		mins := minsByDate[key]
		if mins >= minPerDay {
			tempStreak++
			if tempStreak > maxStreak {
				maxStreak = tempStreak
			}
		} else {
			tempStreak = 0
		}
	}

	// 2. Calculate activeStreak
	activeStreak := 0
	for i := 0; i <= 45; i++ {
		day := today.AddDate(0, 0, -i)
		key := day.Format("2006-01-02")
		mins := minsByDate[key]
		if mins >= minPerDay {
			activeStreak++
		} else {
			if i == 0 {
				// missed today is fine, the streak is still active from yesterday
				continue
			}
			// missed yesterday (or earlier), streak is broken
			break
		}
	}

	displayedStreak := activeStreak
	if maxStreak >= targetDays {
		displayedStreak = maxStreak
	}

	minsToday := minsByDate[today.Format("2006-01-02")]
	percent := (displayedStreak * 100) / targetDays
	if percent > 100 {
		percent = 100
	}

	return ChallengeProgress{
		CurrentStreakDays: displayedStreak,
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
