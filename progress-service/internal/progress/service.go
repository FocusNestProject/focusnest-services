package progress

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	dateLayout     = "2006-01-02"
	recoveryQuota  = 5
	graceDays       = 3 // grace window = expired_date … expired_date+2 (3 calendar days)
	daysUntilExpiry = 1 // 1 day without activity -> expired next day (last_productive_date + 1)
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

// resolveLocation returns time.Location for the given IANA timezone; defaults to service loc (Asia/Jakarta).
func (s *service) resolveLocation(tz string) *time.Location {
	tz = strings.TrimSpace(tz)
	if tz == "" {
		return s.loc
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return s.loc
	}
	return loc
}

// getLastProductiveDate returns the latest date string (YYYY-MM-DD) in days that has status "done" and is <= today.
func getLastProductiveDate(days []DayStatus, today time.Time) string {
	todayStr := today.Format(dateLayout)
	var last string
	for _, d := range days {
		if d.Status == "done" && d.Date <= todayStr {
			if last == "" || d.Date > last {
				last = d.Date
			}
		}
	}
	return last
}

// streakEndingOn returns the consecutive "done" streak count ending on the given date (inclusive).
// days are in chronological order; we count backward from endDateStr.
func streakEndingOn(days []DayStatus, endDateStr string) int {
	var j int
	for j = len(days) - 1; j >= 0; j-- {
		if days[j].Date == endDateStr {
			break
		}
	}
	if j < 0 || days[j].Status != "done" {
		return 0
	}
	run := 0
	for i := j; i >= 0; i-- {
		if days[i].Status != "done" {
			break
		}
		run++
	}
	return run
}

// GetCurrentStreak returns current running streak (last 30 days window) with status/grace/expired and recovery quota.
// timezone: IANA timezone (e.g. Asia/Jakarta); used for "today". Empty = Asia/Jakarta.
func (s *service) GetCurrentStreak(ctx context.Context, userID string, timezone string) (*StreakData, error) {
	loc := s.resolveLocation(timezone)
	now := time.Now().In(loc)
	today := truncateToDay(now)
	endDate := today
	startDate := endDate.AddDate(0, 0, -30)

	summaries, err := s.repo.GetDailySummaries(ctx, userID, startDate, endDate.AddDate(0, 0, 1))
	if err != nil {
		return nil, fmt.Errorf("failed to get daily summaries: %w", err)
	}

	dayMap := make(map[string]bool)
	for _, summary := range summaries {
		dayStr := summary.Date.In(loc).Format(dateLayout)
		dayMap[dayStr] = true
	}

	days := make([]DayStatus, 0)
	for d := startDate; d.Before(endDate.AddDate(0, 0, 1)); d = d.AddDate(0, 0, 1) {
		dayStr := d.Format(dateLayout)
		dayName := d.Format("Monday")
		var dayStatus string
		if d.After(today) {
			dayStatus = "upcoming"
		} else if dayMap[dayStr] {
			dayStatus = "done"
		} else {
			dayStatus = "skipped"
		}
		days = append(days, DayStatus{Date: dayStr, Day: dayName, Status: dayStatus})
	}

	totalStreak, currentStreak := s.calculateStreaks(days, now)
	lastProd := getLastProductiveDate(days, today)

	resp := &StreakData{
		TotalStreak:           totalStreak,
		CurrentStreak:         currentStreak,
		Days:                  days,
		Status:                StreakStatusActive,
		RecoveryQuotaPerMonth: recoveryQuota,
	}

	// Recovery used this month (always return; 0 for non-premium is fine)
	yearMonth := now.Format("2006-01")
	used, _ := s.repo.GetRecoveryQuota(ctx, userID, yearMonth)
	resp.RecoveryUsedThisMonth = used

	state, err := s.repo.GetStreakState(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get streak state: %w", err)
	}

	// If user has recovered and has override, handle bridged streak across the recovered gap.
	if state != nil && state.OverrideStreakValue > 0 && state.ExpiredAt != "" {
		expiredT, _ := time.ParseInLocation(dateLayout, state.ExpiredAt, loc)
		lastProdT2, errLast := time.ParseInLocation(dateLayout, lastProd, loc)
		if lastProd != "" && errLast == nil && lastProdT2.After(expiredT) {
			// New activity after expired date: bridge the gap
			// bridged = pre-break streak + consecutive days after the break
			postBreakStreak := streakEndingOn(days, lastProd)
			bridgedStreak := state.StreakValueBeforeExpired + postBreakStreak

			// Check for a NEW break based on the actual last productive date
			newExpiredDate := lastProdT2.AddDate(0, 0, daysUntilExpiry)

			if today.Before(newExpiredDate) || today.Equal(newExpiredDate) {
				// Still active: persist bridged value
				state.OverrideStreakValue = bridgedStreak
				_ = s.repo.SetStreakState(ctx, userID, state)
				resp.CurrentStreak = bridgedStreak
				resp.Status = StreakStatusActive
				return resp, nil
			}

			// New break detected after the recovered streak!
			newGraceEnd := newExpiredDate.AddDate(0, 0, graceDays-1)
			state.ExpiredAt = newExpiredDate.Format(dateLayout)
			state.StreakValueBeforeExpired = bridgedStreak
			state.OverrideStreakValue = 0
			_ = s.repo.SetStreakState(ctx, userID, state)

			resp.ExpiredAt = newExpiredDate.Format(dateLayout)

			if today.After(newGraceEnd) {
				resp.CurrentStreak = 0
				resp.Status = StreakStatusExpired
				return resp, nil
			}
			// Within grace for the new break
			resp.Status = StreakStatusGrace
			resp.GraceEndsAt = newGraceEnd.Format(dateLayout)
			resp.CurrentStreak = bridgedStreak
			return resp, nil
		}
		// No new activity after expired date: use override as-is
		resp.CurrentStreak = state.OverrideStreakValue
		resp.Status = StreakStatusActive
		return resp, nil
	}

	if lastProd == "" {
		// No productive day in window -> no streak, no expired logic
		resp.CurrentStreak = 0
		return resp, nil
	}

	lastProdT, err := time.ParseInLocation(dateLayout, lastProd, loc)
	if err != nil {
		return resp, nil
	}
	expiredDate := lastProdT.AddDate(0, 0, daysUntilExpiry)

	if today.Before(expiredDate) || today.Equal(expiredDate) {
		// Still active
		return resp, nil
	}

	// Expired: today > expiredDate
	resp.ExpiredAt = expiredDate.Format(dateLayout)
	if state == nil || state.ExpiredAt != resp.ExpiredAt {
		// First time we see this expiry: persist streak value for recovery
		streakBeforeExpired := streakEndingOn(days, lastProd)
		newState := &StreakState{
			UserID:                  userID,
			ExpiredAt:               resp.ExpiredAt,
			StreakValueBeforeExpired: streakBeforeExpired,
		}
		if state != nil {
			newState.OverrideStreakValue = state.OverrideStreakValue
		}
		_ = s.repo.SetStreakState(ctx, userID, newState)
		state = newState
	}

	graceEnd := expiredDate.AddDate(0, 0, graceDays-1) // expired_date + 2 (last day of grace)
	if today.After(graceEnd) {
		// After grace: reset to 0
		resp.CurrentStreak = 0
		resp.Status = StreakStatusExpired
		return resp, nil
	}

	// Within grace window (expired_date < today <= graceEnd)
	resp.Status = StreakStatusGrace
	resp.GraceEndsAt = graceEnd.Format(dateLayout)
	resp.CurrentStreak = state.StreakValueBeforeExpired
	return resp, nil
}

// RecoverStreak restores the user's streak after expiry (premium only, quota 5/month).
func (s *service) RecoverStreak(ctx context.Context, userID string, isPremium bool, timezone string) (*StreakData, error) {
	if !isPremium {
		return nil, ErrNotPremium
	}

	loc := s.resolveLocation(timezone)
	now := time.Now().In(loc)
	yearMonth := now.Format("2006-01")

	state, err := s.repo.GetStreakState(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get streak state: %w", err)
	}
	if state == nil || state.ExpiredAt == "" {
		return nil, ErrStreakNotRecoverable
	}

	expiredT, _ := time.ParseInLocation(dateLayout, state.ExpiredAt, loc)
	graceEnd := expiredT.AddDate(0, 0, graceDays-1)
	today := truncateToDay(now)
	if today.After(graceEnd) {
		return nil, ErrStreakNotRecoverable
	}

	count, err := s.repo.IncrementRecoveryQuota(ctx, userID, yearMonth)
	if err != nil {
		if err == ErrRecoveryQuotaExceeded {
			return nil, err
		}
		return nil, fmt.Errorf("increment recovery quota: %w", err)
	}

	state.OverrideStreakValue = state.StreakValueBeforeExpired
	// Keep ExpiredAt so GetCurrentStreak knows to use override until new activity after that date
	if err := s.repo.SetStreakState(ctx, userID, state); err != nil {
		return nil, fmt.Errorf("set streak state: %w", err)
	}

	data, err := s.GetCurrentStreak(ctx, userID, timezone)
	if err != nil {
		return nil, err
	}
	data.RecoveryUsedThisMonth = count
	return data, nil
}

// calculateStreaks calculates total (longest) and current streaks from day statuses.
// - totalStreak  : longest run of consecutive "done" days in the window.
// - currentStreak: consecutive "done" days ending on the *last productive day* (<= today),
//                  so streak tetap menunjukkan jumlah hari terakhir yang konsisten,
//                  meskipun hari ini belum ada aktivitas (belum "done").
func (s *service) calculateStreaks(days []DayStatus, now time.Time) (totalStreak, currentStreak int) {
	// Longest consecutive "done" (unchanged)
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

	// Current streak: count consecutive "done" days ending at last productive date <= today.
	today := truncateToDay(now)
	lastProd := getLastProductiveDate(days, today)
	if lastProd == "" {
		currentStreak = 0
		return
	}

	currentStreak = streakEndingOn(days, lastProd)
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
