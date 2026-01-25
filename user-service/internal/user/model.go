package user

import (
	"context"
	"time"
)

// Profile represents the persisted profile document stored in Firestore.
type Profile struct {
	UserID    string     `json:"user_id" firestore:"user_id"`
	Bio       string     `json:"bio" firestore:"bio"`
	Birthdate *time.Time `json:"birthdate" firestore:"birthdate"`
	PointsTotal int      `json:"points_total" firestore:"points_total"`
	CreatedAt time.Time  `json:"created_at" firestore:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" firestore:"updated_at"`
}

// ProfileMetadata captures derived counters that accompany a profile response.
type ProfileMetadata struct {
	LongestStreak       int `json:"longest_streak"`
	TotalProductivities int `json:"total_productivities"`
	TotalSessions       int `json:"total_sessions"`
	TotalCycle          int `json:"total_cycle"`
}

// Badge represents a milestone earned from points.
type Badge struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	MinPts int    `json:"min_points"`
}

// ChallengeRuleType identifies how progress is computed.
type ChallengeRuleType string

const (
	ChallengeRuleDailyMinutesStreak    ChallengeRuleType = "daily_minutes_streak"
	ChallengeRuleWeeklyShares          ChallengeRuleType = "weekly_shares"
	ChallengeRuleStreakMilestone       ChallengeRuleType = "streak_milestone"
	ChallengeRuleCyclesAndMindfulness  ChallengeRuleType = "cycles_and_mindfulness"
)

// ChallengeDefinition is a static challenge template.
type ChallengeDefinition struct {
	ID           string            `json:"id"`
	Title        string            `json:"title"`
	Description  string            `json:"description"`
	RewardPoints int               `json:"reward_points"`
	RuleType     ChallengeRuleType `json:"rule_type"`

	// Rule params (interpreted based on RuleType)
	MinMinutesPerDay int `json:"min_minutes_per_day,omitempty"`
	ConsecutiveDays  int `json:"consecutive_days,omitempty"`

	// For count-based challenges (e.g., weekly shares)
	TargetCount int `json:"target_count,omitempty"`

	// For streak milestone challenges
	TargetStreak int `json:"target_streak,omitempty"`

	// For cycles and mindfulness challenge
	TargetCycles            int `json:"target_cycles,omitempty"`
	TargetMindfulnessMinutes int `json:"target_mindfulness_minutes,omitempty"`
}

// ChallengeProgress captures computed progress for a challenge.
type ChallengeProgress struct {
	// For daily minutes streak challenges
	CurrentStreakDays int `json:"current_streak_days,omitempty"`
	TargetStreakDays  int `json:"target_streak_days,omitempty"`
	MinMinutesPerDay  int `json:"min_minutes_per_day,omitempty"`
	MinutesToday      int `json:"minutes_today,omitempty"`

	// For count-based challenges (shares, etc.)
	CurrentCount int `json:"current_count,omitempty"`
	TargetCount  int `json:"target_count,omitempty"`

	// For streak milestone challenges
	CurrentStreak int `json:"current_streak,omitempty"`
	TargetStreak  int `json:"target_streak,omitempty"`

	// For cycles and mindfulness challenge
	CurrentCycles            int `json:"current_cycles,omitempty"`
	TargetCycles             int `json:"target_cycles,omitempty"`
	CurrentMindfulnessMinutes int `json:"current_mindfulness_minutes,omitempty"`
	TargetMindfulnessMinutes  int `json:"target_mindfulness_minutes,omitempty"`

	// Generic progress percentage (0-100) for UI
	ProgressPercent int `json:"progress_percent"`
}

// ChallengeStatus is the per-user state for a challenge.
type ChallengeStatus struct {
	Challenge ChallengeDefinition `json:"challenge"`
	Progress  ChallengeProgress   `json:"progress"`
	Completed bool                `json:"completed"`
	Claimed   bool                `json:"claimed"`
}

// ChallengesMeResponse is returned by GET /v1/challenges/me.
type ChallengesMeResponse struct {
	PointsTotal int               `json:"points_total"`
	Badges      []Badge           `json:"badges"`
	Challenges  []ChallengeStatus `json:"challenges"`
}

// ClaimChallengeResponse is returned by POST /v1/challenges/{id}/claim.
type ClaimChallengeResponse struct {
	ChallengeID   string    `json:"challenge_id"`
	Claimed       bool      `json:"claimed"`
	AlreadyClaimed bool     `json:"already_claimed"`
	PointsAwarded int       `json:"points_awarded"`
	PointsTotal   int       `json:"points_total"`
	ClaimedAt     time.Time `json:"claimed_at,omitempty"`
}

// ProfileResponse combines persisted profile fields with derived metadata.
type ProfileResponse struct {
	UserID    string     `json:"user_id"`
	Bio       string     `json:"bio"`
	Birthdate *time.Time `json:"birthdate"`
	PointsTotal int      `json:"points_total"`
	ProfileMetadata
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// ProfileUpdateInput describes the allowed fields during a PATCH request.
type ProfileUpdateInput struct {
	Bio       *string
	Birthdate *BirthdatePatch
}

// BirthdatePatch differentiates between omitted and explicit null updates.
type BirthdatePatch struct {
	IsSet bool
	Value *time.Time
}

// Repository defines the interface for user data access.
type Repository interface {
	GetProfile(ctx context.Context, userID string) (*Profile, error)
	UpsertProfile(ctx context.Context, userID string, updates ProfileUpdateInput) (*Profile, error)
	GetProfileMetadata(ctx context.Context, userID string) (ProfileMetadata, error)
	GetDailyMinutesByDate(ctx context.Context, userID string, startDate, endDate time.Time) (map[string]int, error)
	IsChallengeClaimed(ctx context.Context, userID, challengeID string) (bool, error)
	ClaimChallenge(ctx context.Context, userID, challengeID string, points int) (newTotal int, claimedAt time.Time, alreadyClaimed bool, err error)

	// For weekly shares challenge
	GetWeeklyShareCount(ctx context.Context, userID string, weekStart time.Time) (int, error)
	RecordShare(ctx context.Context, userID string, shareType string) error

	// For streak milestone challenge
	GetCurrentStreak(ctx context.Context, userID string) (int, error)

	// For cycles and mindfulness challenge
	GetTotalCycles(ctx context.Context, userID string) (int, error)
	GetTotalMindfulnessMinutes(ctx context.Context, userID string) (int, error)
	RecordMindfulness(ctx context.Context, userID string, minutes int) error
}

// Service defines the user service interface.
type Service interface {
	GetProfile(ctx context.Context, userID string) (*ProfileResponse, error)
	UpdateProfile(ctx context.Context, userID string, updates ProfileUpdateInput) (*ProfileResponse, error)
	ListChallenges(ctx context.Context) ([]ChallengeDefinition, error)
	GetChallengesMe(ctx context.Context, userID string) (*ChallengesMeResponse, error)
	ClaimChallenge(ctx context.Context, userID, challengeID string) (*ClaimChallengeResponse, error)
	RecordShare(ctx context.Context, userID string, shareType string) error
	RecordMindfulness(ctx context.Context, userID string, minutes int) error
}
