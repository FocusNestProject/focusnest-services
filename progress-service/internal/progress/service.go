package progress

import (
	"context"
	"fmt"
	"strings"
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

func (s *service) GetSummary(ctx context.Context, userID string, input SummaryInput) (*SummaryResponse, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, ErrMissingUserID
	}
	rng := input.Range
	if rng == "" {
		rng = SummaryRangeWeek
	}
	ref := input.ReferenceDate
	if ref.IsZero() {
		ref = time.Now().In(s.loc)
	} else {
		ref = ref.In(s.loc)
	}
	startLocal, endLocal, err := s.summaryBounds(rng, ref)
	if err != nil {
		return nil, err
	}
	entries, err := s.repo.ListProductivities(ctx, userID, startLocal.UTC(), endLocal.UTC())
	if err != nil {
		return nil, fmt.Errorf("failed to list productivities: %w", err)
	}
	category := strings.TrimSpace(input.Category)
	var (
		totalFrame    int
		totalFiltered int
		totalSessions int
		filtered      []ProductivityEntry
	)
	for _, entry := range entries {
		totalFrame += entry.TimeElapsed
		if category == "" || strings.EqualFold(entry.Category, category) {
			totalFiltered += entry.TimeElapsed
			totalSessions++
			filtered = append(filtered, entry)
		}
	}
	distribution := s.buildDistribution(rng, startLocal, ref, filtered)
	prodStart, prodEnd := s.calculateMostProductiveHour(filtered, ref)
	return &SummaryResponse{
		Range:                   rng,
		ReferenceDate:           ref,
		Category:                category,
		TotalFilteredTime:       totalFiltered,
		TimeDistribution:        distribution,
		TotalSessions:           totalSessions,
		TotalTimeFrame:          totalFrame,
		MostProductiveHourStart: prodStart,
		MostProductiveHourEnd:   prodEnd,
	}, nil
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
			status = "done"
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
			status = "done"
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
			status = "done"
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
	// Longest consecutive "done"
	maxStreak := 0
	run := 0
	for _, day := range days {
		if day.Status == "done" {
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
		if day.Status == "done" {
			run++
		} else {
			break
		}
	}
	currentStreak = run
	return
}

func (s *service) summaryBounds(rng SummaryRange, ref time.Time) (time.Time, time.Time, error) {
	refDay := truncateToDay(ref)
	switch rng {
	case SummaryRangeWeek:
		start := refDay
		for start.Weekday() != time.Monday {
			start = start.AddDate(0, 0, -1)
		}
		return start, start.AddDate(0, 0, 7), nil
	case SummaryRangeMonth:
		start := time.Date(ref.Year(), ref.Month(), 1, 0, 0, 0, 0, ref.Location())
		return start, start.AddDate(0, 1, 0), nil
	case SummaryRangeQuarter:
		monthStart := time.Date(ref.Year(), ref.Month(), 1, 0, 0, 0, 0, ref.Location())
		start := monthStart.AddDate(0, -2, 0)
		return start, monthStart.AddDate(0, 1, 0), nil
	case SummaryRangeYear:
		start := time.Date(ref.Year(), time.January, 1, 0, 0, 0, 0, ref.Location())
		return start, start.AddDate(1, 0, 0), nil
	default:
		return time.Time{}, time.Time{}, ErrInvalidSummaryRange
	}
}

func (s *service) buildDistribution(rng SummaryRange, start, ref time.Time, entries []ProductivityEntry) []SummaryBucket {
	switch rng {
	case SummaryRangeWeek:
		return s.buildWeekDistribution(start, entries)
	case SummaryRangeMonth:
		return s.buildMonthDistribution(entries)
	case SummaryRangeQuarter:
		return s.buildQuarterDistribution(ref, entries)
	case SummaryRangeYear:
		return s.buildYearDistribution(entries)
	default:
		return nil
	}
}

func (s *service) buildWeekDistribution(start time.Time, entries []ProductivityEntry) []SummaryBucket {
	labels := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	buckets := make([]SummaryBucket, len(labels))
	for i, label := range labels {
		buckets[i] = SummaryBucket{Label: label}
	}
	for _, entry := range entries {
		day := truncateToDay(entry.StartTime.In(s.loc))
		delta := int(day.Sub(start).Hours() / 24)
		if delta < 0 || delta >= len(buckets) {
			continue
		}
		buckets[delta].TimeElapsed += entry.TimeElapsed
	}
	return buckets
}

func (s *service) buildMonthDistribution(entries []ProductivityEntry) []SummaryBucket {
	labels := []string{"Week1", "Week2", "Week3", "Week4"}
	buckets := make([]SummaryBucket, len(labels))
	for i, label := range labels {
		buckets[i] = SummaryBucket{Label: label}
	}
	for _, entry := range entries {
		day := entry.StartTime.In(s.loc).Day()
		idx := (day - 1) / 7
		if idx < 0 {
			idx = 0
		}
		if idx >= len(buckets) {
			idx = len(buckets) - 1
		}
		buckets[idx].TimeElapsed += entry.TimeElapsed
	}
	return buckets
}

func (s *service) buildQuarterDistribution(ref time.Time, entries []ProductivityEntry) []SummaryBucket {
	start := time.Date(ref.Year(), ref.Month(), 1, 0, 0, 0, 0, ref.Location()).AddDate(0, -2, 0)
	buckets := []SummaryBucket{{Label: "Month1"}, {Label: "Month2"}, {Label: "Month3"}}
	for _, entry := range entries {
		months := monthsBetween(start, entry.StartTime.In(s.loc))
		if months < 0 || months >= len(buckets) {
			continue
		}
		buckets[months].TimeElapsed += entry.TimeElapsed
	}
	return buckets
}

func (s *service) buildYearDistribution(entries []ProductivityEntry) []SummaryBucket {
	buckets := []SummaryBucket{{Label: "Q1"}, {Label: "Q2"}, {Label: "Q3"}, {Label: "Q4"}}
	for _, entry := range entries {
		month := int(entry.StartTime.In(s.loc).Month())
		idx := (month - 1) / 3
		if idx < 0 || idx >= len(buckets) {
			continue
		}
		buckets[idx].TimeElapsed += entry.TimeElapsed
	}
	return buckets
}

func monthsBetween(start, t time.Time) int {
	startYear, startMonth, _ := start.Date()
	tYear, tMonth, _ := t.Date()
	return (tYear-startYear)*12 + int(tMonth-startMonth)
}

func (s *service) calculateMostProductiveHour(entries []ProductivityEntry, reference time.Time) (*time.Time, *time.Time) {
	if len(entries) == 0 {
		return nil, nil
	}
	totals := make(map[time.Time]int)
	for _, entry := range entries {
		start := entry.StartTime.In(s.loc)
		end := entry.EndTime.In(s.loc)
		if end.IsZero() || !end.After(start) {
			if entry.TimeElapsed > 0 {
				end = start.Add(time.Duration(entry.TimeElapsed) * time.Second)
			} else {
				continue
			}
		}
		current := start
		for current.Before(end) {
			hourStart := time.Date(current.Year(), current.Month(), current.Day(), current.Hour(), 0, 0, 0, s.loc)
			hourEnd := hourStart.Add(time.Hour)
			if hourEnd.After(end) {
				hourEnd = end
			}
			segment := int(hourEnd.Sub(current).Minutes())
			if segment <= 0 && hourEnd.After(current) {
				segment = 1
			}
			totals[hourStart] += segment
			current = hourEnd
		}
	}
	if len(totals) == 0 {
		return nil, nil
	}
	var (
		bestStart time.Time
		bestValue int
		found     bool
	)
	for hourStart, total := range totals {
		if !found || total > bestValue || (total == bestValue && hourStart.Before(bestStart)) {
			bestStart = hourStart
			bestValue = total
			found = true
		}
	}
	if !found {
		return nil, nil
	}
	startUTC := bestStart.UTC()
	endUTC := bestStart.Add(time.Hour).UTC()
	return &startUTC, &endUTC
}

func truncateToDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
