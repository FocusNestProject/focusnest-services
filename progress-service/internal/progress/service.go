package progress

import (
	"context"
	"fmt"
	"time"
)

type service struct {
	repo Repository
}

// NewService creates a new progress service
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) GetProgress(userID string, startDate, endDate time.Time) (*ProgressStats, error) {
	return s.repo.GetProgressStats(userID, startDate, endDate)
}

// GetMonthlyStreak returns monthly streak data
func (s *service) GetMonthlyStreak(ctx context.Context, userID string, month, year int) (*MonthlyStreakData, error) {
	// Calculate month boundaries
	monthStart := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0)

	// Get daily summaries for the month
	summaries, err := s.repo.GetDailySummaries(userID, monthStart, monthEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily summaries: %w", err)
	}

	// Create day status map
	dayMap := make(map[string]bool)
	for _, summary := range summaries {
		dayStr := summary.Date.Format("2006-01-02")
		dayMap[dayStr] = true
	}

	// Generate all days in the month
	days := make([]DayStatus, 0)
	now := time.Now().UTC()

	for d := monthStart; d.Before(monthEnd); d = d.AddDate(0, 0, 1) {
		dayStr := d.Format("2006-01-02")
		dayName := d.Format("Monday")

		var status string
		if d.After(now.Truncate(24 * time.Hour)) {
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

// GetWeeklyStreak returns weekly streak data
func (s *service) GetWeeklyStreak(ctx context.Context, userID string, targetDate time.Time) (*WeeklyStreakData, error) {
	// Calculate week boundaries (Monday to Sunday)
	weekStart := targetDate.Truncate(24 * time.Hour)
	for weekStart.Weekday() != time.Monday {
		weekStart = weekStart.AddDate(0, 0, -1)
	}
	weekEnd := weekStart.AddDate(0, 0, 7)

	// Get daily summaries for the week
	summaries, err := s.repo.GetDailySummaries(userID, weekStart, weekEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily summaries: %w", err)
	}

	// Create day status map
	dayMap := make(map[string]bool)
	for _, summary := range summaries {
		dayStr := summary.Date.Format("2006-01-02")
		dayMap[dayStr] = true
	}

	// Generate all days in the week
	days := make([]DayStatus, 0)
	now := time.Now().UTC()

	for d := weekStart; d.Before(weekEnd); d = d.AddDate(0, 0, 1) {
		dayStr := d.Format("2006-01-02")
		dayName := d.Format("Monday")

		var status string
		if d.After(now.Truncate(24 * time.Hour)) {
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

	// Format week as YYYY-WW
	year, week := targetDate.ISOWeek()
	weekStr := fmt.Sprintf("%d-%02d", year, week)

	return &WeeklyStreakData{
		Week:          weekStr,
		TotalStreak:   totalStreak,
		CurrentStreak: currentStreak,
		Days:          days,
	}, nil
}

// GetCurrentStreak returns current running streak
func (s *service) GetCurrentStreak(ctx context.Context, userID string) (*StreakData, error) {
	// Get last 30 days of data
	endDate := time.Now().UTC()
	startDate := endDate.AddDate(0, 0, -30)

	summaries, err := s.repo.GetDailySummaries(userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily summaries: %w", err)
	}

	// Create day status map
	dayMap := make(map[string]bool)
	for _, summary := range summaries {
		dayStr := summary.Date.Format("2006-01-02")
		dayMap[dayStr] = true
	}

	// Generate last 30 days
	days := make([]DayStatus, 0)
	now := time.Now().UTC()

	for d := startDate; d.Before(endDate.AddDate(0, 0, 1)); d = d.AddDate(0, 0, 1) {
		dayStr := d.Format("2006-01-02")
		dayName := d.Format("Monday")

		var status string
		if d.After(now.Truncate(24 * time.Hour)) {
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

// calculateStreaks calculates total and current streaks from day statuses
func (s *service) calculateStreaks(days []DayStatus, now time.Time) (totalStreak, currentStreak int) {
	// Calculate total streak (longest consecutive active days)
	maxStreak := 0
	currentRun := 0

	for _, day := range days {
		if day.Status == "active" {
			currentRun++
			if currentRun > maxStreak {
				maxStreak = currentRun
			}
		} else {
			currentRun = 0
		}
	}
	totalStreak = maxStreak

	// Calculate current streak (consecutive active days ending today or yesterday)
	currentRun = 0
	today := now.Truncate(24 * time.Hour)

	// Go backwards from today to find current streak
	for i := len(days) - 1; i >= 0; i-- {
		day := days[i]
		dayDate, _ := time.Parse("2006-01-02", day.Date)

		if dayDate.After(today) {
			continue // Skip future days
		}

		if day.Status == "active" {
			currentRun++
		} else {
			break // Streak broken
		}
	}

	currentStreak = currentRun
	return
}
