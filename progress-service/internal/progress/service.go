package progress

import (
	"context"
	"fmt"
	"time"
)

type service struct {
	repo Repository
	loc  *time.Location
}

// NewService creates a new progress service with Asia/Jakarta as default location
func NewService(repo Repository) Service {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.UTC
	}
	return &service{repo: repo, loc: loc}
}

// NewServiceWithLocation allows injecting a custom time.Location
func NewServiceWithLocation(repo Repository, loc *time.Location) Service {
	if loc == nil {
		loc = time.UTC
	}
	return &service{repo: repo, loc: loc}
}

func (s *service) GetProgress(ctx context.Context, userID string, startDate, endDate time.Time) (*ProgressStats, error) {
	return s.repo.GetProgressStats(ctx, userID, startDate, endDate)
}

// GetMonthlyStreak returns monthly streak data
func (s *service) GetMonthlyStreak(ctx context.Context, userID string, month, year int) (*MonthlyStreakData, error) {
	// Calculate local month boundaries in service location, then rely on repo using those as-is
	monthStart := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, s.loc)
	monthEnd := monthStart.AddDate(0, 1, 0)

	// For Firestore queries it's common to store UTC; here we assume caller passes UTC boundaries if needed.
	// If you need strict UTC conversion: use monthStart.UTC(), monthEnd.UTC().
	summaries, err := s.repo.GetDailySummaries(ctx, userID, monthStart, monthEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily summaries: %w", err)
	}

	// Create day status map
	dayMap := make(map[string]bool)
	for _, summary := range summaries {
		dayStr := summary.Date.In(s.loc).Format("2006-01-02")
		dayMap[dayStr] = true
	}

	// Generate all days in the month
	days := make([]DayStatus, 0)
	now := time.Now().In(s.loc)

	for d := monthStart; d.Before(monthEnd); d = d.AddDate(0, 0, 1) {
		dayStr := d.Format("2006-01-02")
		dayName := d.Format("Monday")

		var status string
		if d.After(truncateToDay(now)) {
			status = "upcoming"
		} else if dayMap[dayStr] {
			status = "active"
		} else {
			status = "skipped"
		}

		days = append(days, DayStatus{
			Date:   dayStr,
			Day:    dayName,
			Status: status,
		})
	}

	// Calculate streaks
	totalStreak, currentStreak := s.calculateStreaks(days, now)

	return &MonthlyStreakData{
		Month:         month,
		Year:          year,
		TotalStreak:   totalStreak,
		CurrentStreak: currentStreak,
		Days:          days,
	}, nil
}

// GetWeeklyStreak returns weekly streak data (Monday–Sunday)
func (s *service) GetWeeklyStreak(ctx context.Context, userID string, targetDate time.Time) (*WeeklyStreakData, error) {
	td := targetDate.In(s.loc)
	weekStart := truncateToDay(td)
	for weekStart.Weekday() != time.Monday {
		weekStart = weekStart.AddDate(0, 0, -1)
	}
	weekEnd := weekStart.AddDate(0, 0, 7)

	summaries, err := s.repo.GetDailySummaries(ctx, userID, weekStart, weekEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily summaries: %w", err)
	}

	dayMap := make(map[string]bool)
	for _, summary := range summaries {
		dayStr := summary.Date.In(s.loc).Format("2006-01-02")
		dayMap[dayStr] = true
	}

	days := make([]DayStatus, 0)
	now := time.Now().In(s.loc)

	for d := weekStart; d.Before(weekEnd); d = d.AddDate(0, 0, 1) {
		dayStr := d.Format("2006-01-02")
		dayName := d.Format("Monday")

		var status string
		if d.After(truncateToDay(now)) {
			status = "upcoming"
		} else if dayMap[dayStr] {
			status = "active"
		} else {
			status = "skipped"
		}

		days = append(days, DayStatus{
			Date:   dayStr,
			Day:    dayName,
			Status: status,
		})
	}

	// Calculate streaks
	totalStreak, currentStreak := s.calculateStreaks(days, now)

	// Format week as YYYY-WW (ISO)
	year, week := td.ISOWeek()
	weekStr := fmt.Sprintf("%d-%02d", year, week)

	return &WeeklyStreakData{
		Week:          weekStr,
		TotalStreak:   totalStreak,
		CurrentStreak: currentStreak,
		Days:          days,
	}, nil
}

// GetCurrentStreak returns current running streak (last 30 days window)
func (s *service) GetCurrentStreak(ctx context.Context, userID string) (*StreakData, error) {
	now := time.Now().In(s.loc)
	endDate := truncateToDay(now)
	startDate := endDate.AddDate(0, 0, -30)

	summaries, err := s.repo.GetDailySummaries(ctx, userID, startDate, endDate.AddDate(0, 0, 1)) // include today via [start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily summaries: %w", err)
	}

	dayMap := make(map[string]bool)
	for _, summary := range summaries {
		dayStr := summary.Date.In(s.loc).Format("2006-01-02")
		dayMap[dayStr] = true
	}

	days := make([]DayStatus, 0)
	for d := startDate; d.Before(endDate.AddDate(0, 0, 1)); d = d.AddDate(0, 0, 1) {
		dayStr := d.Format("2006-01-02")
		dayName := d.Format("Monday")

		var status string
		if d.After(truncateToDay(now)) {
			status = "upcoming"
		} else if dayMap[dayStr] {
			status = "active"
		} else {
			status = "skipped"
		}

		days = append(days, DayStatus{
			Date:   dayStr,
			Day:    dayName,
			Status: status,
		})
	}

	// Calculate streaks
	totalStreak, currentStreak := s.calculateStreaks(days, now)

	return &StreakData{
		TotalStreak:   totalStreak,
		CurrentStreak: currentStreak,
		Days:          days,
	}, nil
}

// calculateStreaks calculates total (longest) and current streaks from day statuses
func (s *service) calculateStreaks(days []DayStatus, now time.Time) (totalStreak, currentStreak int) {
	// Longest consecutive "active"
	maxStreak := 0
	run := 0
	for _, day := range days {
		if day.Status == "active" {
			run++
			if run > maxStreak {
				maxStreak = run
			}
		} else if day.Status == "skipped" {
			run = 0
		}
	}
	totalStreak = maxStreak

	// Current streak (ending today; tolerate that "today" might be not finished yet)
	run = 0
	today := truncateToDay(now)

	for i := len(days) - 1; i >= 0; i-- {
		day := days[i]
		dayDate, _ := time.Parse("2006-01-02", day.Date)
		// Skip future days just in case
		if dayDate.After(today) {
			continue
		}
		if day.Status == "active" {
			run++
		} else {
			break
		}
	}
	currentStreak = run
	return
}

func truncateToDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
