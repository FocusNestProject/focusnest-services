package chatbot

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestFormatEnrichmentPrompt_Nil(t *testing.T) {
	result := FormatEnrichmentPrompt(nil, "en")
	if result != "" {
		t.Errorf("expected empty string for nil context, got %q", result)
	}
}

func TestFormatEnrichmentPrompt_EmptyUser(t *testing.T) {
	uctx := &UserProductivityContext{
		TodayByCategory: make(map[string]int),
		WeekByCategory:  make(map[string]int),
	}
	result := FormatEnrichmentPrompt(uctx, "en")
	if result != "" {
		t.Errorf("expected empty string for zero-data user, got %q", result)
	}
}

func TestFormatEnrichmentPrompt_EN(t *testing.T) {
	uctx := &UserProductivityContext{
		TodaySessions:   2,
		TodayMinutes:    45,
		TodayByCategory: map[string]int{"Work": 30, "Study": 15},
		WeekSessions:    8,
		WeekMinutes:     200,
		WeekByCategory:  map[string]int{"Work": 130, "Study": 70},
		CurrentStreak:   5,
		LongestStreak:   12,
		RecentMoods:     []string{"Fokus", "Semangat", "Capek"},
		PreferredTimeMode: "Pomodoro",
		PointsTotal:       150,
		RecentSessions: []SessionSummary{
			{Name: "Math homework", Category: "Study", Minutes: 25},
			{Name: "Code review", Category: "Work", Minutes: 45},
		},
	}

	result := FormatEnrichmentPrompt(uctx, "en")

	checks := []string{
		"[USER PRODUCTIVITY DATA",
		"Today: 2 sessions, 45 min",
		"Work: 30 min",
		"Study: 15 min",
		"This week: 8 sessions, 3h 20min",
		"Top: Work (2h 10min)",
		"Streak: 5 days (best: 12)",
		"Mood trend: Fokus, Semangat, Capek",
		"Preferred mode: Pomodoro",
		"Points: 150",
		"\"Math homework\" (Study, 25min)",
		"\"Code review\" (Work, 45min)",
		"[END USER DATA]",
	}

	for _, check := range checks {
		if !strings.Contains(result, check) {
			t.Errorf("expected output to contain %q, got:\n%s", check, result)
		}
	}
}

func TestFormatEnrichmentPrompt_ID_NoSessions(t *testing.T) {
	uctx := &UserProductivityContext{
		TodayByCategory: make(map[string]int),
		WeekByCategory:  make(map[string]int),
		WeekSessions:    3,
		WeekMinutes:     90,
		CurrentStreak:   2,
		LongestStreak:   2,
	}

	result := FormatEnrichmentPrompt(uctx, "id")
	if !strings.Contains(result, "Hari ini: belum ada sesi") {
		t.Errorf("expected Indonesian 'no sessions' text, got:\n%s", result)
	}
	if !strings.Contains(result, "This week: 3 sessions") {
		t.Errorf("expected week summary, got:\n%s", result)
	}
}

func TestFormatEnrichmentPrompt_OnlyPoints(t *testing.T) {
	uctx := &UserProductivityContext{
		TodayByCategory: make(map[string]int),
		WeekByCategory:  make(map[string]int),
		PointsTotal:     50,
	}

	result := FormatEnrichmentPrompt(uctx, "en")
	if !strings.Contains(result, "Points: 50") {
		t.Errorf("expected points, got:\n%s", result)
	}
	if !strings.Contains(result, "Today: no sessions yet") {
		t.Errorf("expected 'no sessions yet', got:\n%s", result)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		mins int
		want string
	}{
		{0, "0 min"},
		{30, "30 min"},
		{60, "1h"},
		{90, "1h 30min"},
		{200, "3h 20min"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.mins)
		if got != tt.want {
			t.Errorf("formatDuration(%d) = %q, want %q", tt.mins, got, tt.want)
		}
	}
}

func TestCalculateStreakFromDocs_Empty(t *testing.T) {
	current, longest := calculateStreakFromDocs(nil, time.Now().UTC())
	if current != 0 || longest != 0 {
		t.Errorf("expected (0, 0) for empty docs, got (%d, %d)", current, longest)
	}
}

func TestCalculateStreakFromDocs_ConsecutiveDays(t *testing.T) {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	docs := make([]productivityDoc, 0)
	for i := 0; i < 5; i++ {
		docs = append(docs, productivityDoc{
			StartTime: today.AddDate(0, 0, -i).Add(10 * time.Hour),
		})
	}

	current, longest := calculateStreakFromDocs(docs, today)
	if current != 5 {
		t.Errorf("expected current streak 5, got %d", current)
	}
	if longest != 5 {
		t.Errorf("expected longest streak 5, got %d", longest)
	}
}

func TestCalculateStreakFromDocs_GapSkipsToday(t *testing.T) {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// Yesterday and day before, but not today
	docs := []productivityDoc{
		{StartTime: today.AddDate(0, 0, -1).Add(10 * time.Hour)},
		{StartTime: today.AddDate(0, 0, -2).Add(10 * time.Hour)},
		{StartTime: today.AddDate(0, 0, -3).Add(10 * time.Hour)},
	}

	current, longest := calculateStreakFromDocs(docs, today)
	if current != 3 {
		t.Errorf("expected current streak 3 (skipping today), got %d", current)
	}
	if longest != 3 {
		t.Errorf("expected longest streak 3, got %d", longest)
	}
}

func TestCacheTTLBehavior(t *testing.T) {
	p := &firestoreEnrichmentProvider{
		cache: make(map[string]cacheEntry),
	}

	data := &UserProductivityContext{PointsTotal: 100}

	// Simulate a cached entry that hasn't expired
	p.mu.Lock()
	p.cache["user1"] = cacheEntry{data: data, expiresAt: time.Now().Add(5 * time.Minute)}
	p.mu.Unlock()

	p.mu.RLock()
	entry, ok := p.cache["user1"]
	p.mu.RUnlock()

	if !ok || entry.data.PointsTotal != 100 {
		t.Error("expected to find cached data")
	}
	if !time.Now().Before(entry.expiresAt) {
		t.Error("expected entry to not be expired")
	}

	// Simulate an expired entry
	p.mu.Lock()
	p.cache["user2"] = cacheEntry{data: data, expiresAt: time.Now().Add(-1 * time.Minute)}
	p.mu.Unlock()

	p.mu.RLock()
	entry2, ok2 := p.cache["user2"]
	p.mu.RUnlock()

	if !ok2 {
		t.Error("expected cache entry to exist")
	}
	if time.Now().Before(entry2.expiresAt) {
		t.Error("expected entry to be expired")
	}
}

func TestCacheConcurrentAccess(t *testing.T) {
	p := &firestoreEnrichmentProvider{
		cache: make(map[string]cacheEntry),
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := "user"
			data := &UserProductivityContext{PointsTotal: id}

			p.mu.Lock()
			p.cache[key] = cacheEntry{data: data, expiresAt: time.Now().Add(5 * time.Minute)}
			p.mu.Unlock()

			p.mu.RLock()
			_, _ = p.cache[key]
			p.mu.RUnlock()
		}(i)
	}
	wg.Wait()
}

// mockEnrichmentProvider is a test helper for other tests.
type mockEnrichmentProvider struct {
	data *UserProductivityContext
	err  error
}

func (m *mockEnrichmentProvider) GetUserContext(_ context.Context, _ string) (*UserProductivityContext, error) {
	return m.data, m.err
}
