package chatbot

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SessionSummary holds a single recent productivity session.
type SessionSummary struct {
	Name     string
	Category string
	Minutes  int
	Mood     string
	TimeMode string
}

// UserProductivityContext holds aggregated user data for enriching the chatbot prompt.
type UserProductivityContext struct {
	// Today
	TodaySessions int
	TodayMinutes  int
	TodayByCategory map[string]int // category -> minutes

	// This week
	WeekSessions int
	WeekMinutes  int
	WeekByCategory map[string]int

	// Streak
	CurrentStreak int
	LongestStreak int

	// Recent moods (last 5 sessions)
	RecentMoods []string

	// Preferred time mode (most common from recent sessions)
	PreferredTimeMode string

	// Last 3 sessions
	RecentSessions []SessionSummary

	// Points
	PointsTotal int
}

// EnrichmentProvider fetches user productivity data for chatbot context.
type EnrichmentProvider interface {
	GetUserContext(ctx context.Context, userID string) (*UserProductivityContext, error)
}

type cacheEntry struct {
	data      *UserProductivityContext
	expiresAt time.Time
}

type firestoreEnrichmentProvider struct {
	client *firestore.Client
	logger *slog.Logger
	mu     sync.RWMutex
	cache  map[string]cacheEntry
}

// NewFirestoreEnrichmentProvider creates an EnrichmentProvider backed by Firestore.
func NewFirestoreEnrichmentProvider(client *firestore.Client, logger *slog.Logger) EnrichmentProvider {
	return &firestoreEnrichmentProvider{
		client: client,
		logger: logger,
		cache:  make(map[string]cacheEntry),
	}
}

const enrichmentCacheTTL = 5 * time.Minute

// GetUserContext fetches productivity data for the given user, using a 5-min in-memory cache.
func (p *firestoreEnrichmentProvider) GetUserContext(ctx context.Context, userID string) (*UserProductivityContext, error) {
	// Check cache
	p.mu.RLock()
	if entry, ok := p.cache[userID]; ok && time.Now().Before(entry.expiresAt) {
		p.mu.RUnlock()
		return entry.data, nil
	}
	p.mu.RUnlock()

	// Cache miss — fetch from Firestore
	uctx, err := p.fetchUserContext(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Store in cache
	p.mu.Lock()
	p.cache[userID] = cacheEntry{data: uctx, expiresAt: time.Now().Add(enrichmentCacheTTL)}
	p.mu.Unlock()

	return uctx, nil
}

// productivityDoc mirrors the Firestore fields we need from users/{uid}/productivities.
type productivityDoc struct {
	ActivityName string    `firestore:"activity_name"`
	TimeElapsed  int       `firestore:"time_elapsed"`
	TimeMode     string    `firestore:"time_mode"`
	Category     string    `firestore:"category"`
	Mood         string    `firestore:"mood"`
	StartTime    time.Time `firestore:"start_time"`
}

// streakStateDoc mirrors the streak_state/{uid} document.
type streakStateDoc struct {
	ExpiredAt               string `firestore:"expired_at"`
	StreakValueBeforeExpired int    `firestore:"streak_value_before_expired"`
	OverrideStreakValue      int    `firestore:"override_streak_value"`
}

// profileDoc mirrors the fields we need from profiles/{uid}.
type profileDoc struct {
	PointsTotal int `firestore:"points_total"`
}

func (p *firestoreEnrichmentProvider) fetchUserContext(ctx context.Context, userID string) (*UserProductivityContext, error) {
	uctx := &UserProductivityContext{
		TodayByCategory: make(map[string]int),
		WeekByCategory:  make(map[string]int),
	}

	now := time.Now().UTC()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	weekStart := todayStart.AddDate(0, 0, -int(todayStart.Weekday()))

	g, gctx := errgroup.WithContext(ctx)

	// 1. Fetch productivities from last 7 days
	var docs []productivityDoc
	g.Go(func() error {
		iter := p.client.Collection("users").Doc(userID).Collection("productivities").
			Where("deleted", "==", false).
			Where("anchor", ">=", weekStart).
			OrderBy("anchor", firestore.Desc).
			Limit(50).
			Documents(gctx)
		defer iter.Stop()

		for {
			doc, err := iter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return fmt.Errorf("fetch productivities: %w", err)
			}
			var d productivityDoc
			if err := doc.DataTo(&d); err != nil {
				p.logger.Warn("skip malformed productivity doc", slog.String("id", doc.Ref.ID), slog.Any("error", err))
				continue
			}
			docs = append(docs, d)
		}
		return nil
	})

	// 2. Fetch streak state
	var streakDoc *streakStateDoc
	g.Go(func() error {
		doc, err := p.client.Collection("streak_state").Doc(userID).Get(gctx)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return nil
			}
			return fmt.Errorf("fetch streak_state: %w", err)
		}
		var sd streakStateDoc
		if err := doc.DataTo(&sd); err != nil {
			return fmt.Errorf("unmarshal streak_state: %w", err)
		}
		streakDoc = &sd
		return nil
	})

	// 3. Fetch profile for points
	var profile *profileDoc
	g.Go(func() error {
		doc, err := p.client.Collection("profiles").Doc(userID).Get(gctx)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return nil
			}
			return fmt.Errorf("fetch profile: %w", err)
		}
		var pd profileDoc
		if err := doc.DataTo(&pd); err != nil {
			return fmt.Errorf("unmarshal profile: %w", err)
		}
		profile = &pd
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Aggregate productivities
	timeModeCount := make(map[string]int)
	moodsSeen := 0

	for _, d := range docs {
		mins := d.TimeElapsed / 60
		if mins <= 0 && d.TimeElapsed > 0 {
			mins = 1
		}

		// Week totals
		uctx.WeekSessions++
		uctx.WeekMinutes += mins
		if d.Category != "" {
			uctx.WeekByCategory[d.Category] += mins
		}

		// Today totals
		if !d.StartTime.Before(todayStart) {
			uctx.TodaySessions++
			uctx.TodayMinutes += mins
			if d.Category != "" {
				uctx.TodayByCategory[d.Category] += mins
			}
		}

		// Moods (last 5 — docs are ordered desc)
		if d.Mood != "" && moodsSeen < 5 {
			uctx.RecentMoods = append(uctx.RecentMoods, d.Mood)
			moodsSeen++
		}

		// Time mode count
		if d.TimeMode != "" {
			timeModeCount[d.TimeMode]++
		}
	}

	// Last 3 sessions (docs are ordered desc, so first 3)
	limit := 3
	if len(docs) < limit {
		limit = len(docs)
	}
	for i := 0; i < limit; i++ {
		d := docs[i]
		mins := d.TimeElapsed / 60
		if mins <= 0 && d.TimeElapsed > 0 {
			mins = 1
		}
		uctx.RecentSessions = append(uctx.RecentSessions, SessionSummary{
			Name:     d.ActivityName,
			Category: d.Category,
			Minutes:  mins,
			Mood:     d.Mood,
			TimeMode: d.TimeMode,
		})
	}

	// Preferred time mode
	if len(timeModeCount) > 0 {
		maxCount := 0
		for mode, count := range timeModeCount {
			if count > maxCount {
				maxCount = count
				uctx.PreferredTimeMode = mode
			}
		}
	}

	// Streak: calculate current streak from productivities and apply streak_state override
	uctx.CurrentStreak, uctx.LongestStreak = calculateStreakFromDocs(docs, todayStart)
	if streakDoc != nil && streakDoc.OverrideStreakValue > uctx.CurrentStreak {
		uctx.CurrentStreak = streakDoc.OverrideStreakValue
	}

	// Points
	if profile != nil {
		uctx.PointsTotal = profile.PointsTotal
	}

	return uctx, nil
}

// calculateStreakFromDocs computes current and longest streak from productivity docs.
func calculateStreakFromDocs(docs []productivityDoc, todayStart time.Time) (current, longest int) {
	if len(docs) == 0 {
		return 0, 0
	}

	// Collect unique active days
	activeDays := make(map[string]bool)
	for _, d := range docs {
		if !d.StartTime.IsZero() {
			day := d.StartTime.UTC().Format("2006-01-02")
			activeDays[day] = true
		}
	}

	// Calculate current streak counting back from today
	for i := 0; i < 60; i++ {
		day := todayStart.AddDate(0, 0, -i).Format("2006-01-02")
		if activeDays[day] {
			current++
		} else if i == 0 {
			// Today might not have activity yet, skip it
			continue
		} else {
			break
		}
	}

	// Longest streak within the data we have
	// Sort days and find longest consecutive run
	dayList := make([]string, 0, len(activeDays))
	for day := range activeDays {
		dayList = append(dayList, day)
	}
	sort.Strings(dayList)

	run := 1
	longest = 1
	for i := 1; i < len(dayList); i++ {
		prev, _ := time.Parse("2006-01-02", dayList[i-1])
		curr, _ := time.Parse("2006-01-02", dayList[i])
		if curr.Sub(prev) == 24*time.Hour {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 1
		}
	}

	if current > longest {
		longest = current
	}

	return current, longest
}

// FormatEnrichmentPrompt builds a compact text block from user productivity data.
// Returns empty string if there is no data (new user).
func FormatEnrichmentPrompt(uctx *UserProductivityContext, lang string) string {
	if uctx == nil {
		return ""
	}
	// If user has zero activity, return nothing
	if uctx.WeekSessions == 0 && uctx.CurrentStreak == 0 && uctx.PointsTotal == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n[USER PRODUCTIVITY DATA - Reference this to personalize your responses]\n")

	// Today
	if uctx.TodaySessions > 0 {
		b.WriteString(fmt.Sprintf("Today: %d sessions, %s", uctx.TodaySessions, formatDuration(uctx.TodayMinutes)))
		if len(uctx.TodayByCategory) > 0 {
			b.WriteString(" (")
			b.WriteString(formatCategoryMap(uctx.TodayByCategory))
			b.WriteString(")")
		}
		b.WriteString("\n")
	} else {
		if lang == languageIndonesian {
			b.WriteString("Hari ini: belum ada sesi\n")
		} else {
			b.WriteString("Today: no sessions yet\n")
		}
	}

	// This week
	if uctx.WeekSessions > 0 {
		topCat, topMins := topCategory(uctx.WeekByCategory)
		b.WriteString(fmt.Sprintf("This week: %d sessions, %s", uctx.WeekSessions, formatDuration(uctx.WeekMinutes)))
		if topCat != "" {
			b.WriteString(fmt.Sprintf(" | Top: %s (%s)", topCat, formatDuration(topMins)))
		}
		b.WriteString("\n")
	}

	// Streak
	if uctx.CurrentStreak > 0 || uctx.LongestStreak > 0 {
		b.WriteString(fmt.Sprintf("Streak: %d days (best: %d)", uctx.CurrentStreak, uctx.LongestStreak))
		if len(uctx.RecentMoods) > 0 {
			b.WriteString(fmt.Sprintf(" | Mood trend: %s", strings.Join(uctx.RecentMoods, ", ")))
		}
		b.WriteString("\n")
	}

	// Preferred mode & points
	var extras []string
	if uctx.PreferredTimeMode != "" {
		extras = append(extras, fmt.Sprintf("Preferred mode: %s", uctx.PreferredTimeMode))
	}
	if uctx.PointsTotal > 0 {
		extras = append(extras, fmt.Sprintf("Points: %d", uctx.PointsTotal))
	}
	if len(extras) > 0 {
		b.WriteString(strings.Join(extras, " | "))
		b.WriteString("\n")
	}

	// Recent sessions
	if len(uctx.RecentSessions) > 0 {
		b.WriteString("Recent: ")
		parts := make([]string, 0, len(uctx.RecentSessions))
		for _, s := range uctx.RecentSessions {
			name := s.Name
			if name == "" {
				name = "Untitled"
			}
			parts = append(parts, fmt.Sprintf("\"%s\" (%s, %dmin)", name, s.Category, s.Minutes))
		}
		b.WriteString(strings.Join(parts, ", "))
		b.WriteString("\n")
	}

	b.WriteString("[END USER DATA]")
	return b.String()
}

func formatDuration(minutes int) string {
	if minutes < 60 {
		return fmt.Sprintf("%d min", minutes)
	}
	h := minutes / 60
	m := minutes % 60
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh %dmin", h, m)
}

func formatCategoryMap(cats map[string]int) string {
	type catMin struct {
		cat string
		min int
	}
	sorted := make([]catMin, 0, len(cats))
	for c, m := range cats {
		sorted = append(sorted, catMin{c, m})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].min > sorted[j].min })

	parts := make([]string, 0, len(sorted))
	for _, cm := range sorted {
		parts = append(parts, fmt.Sprintf("%s: %d min", cm.cat, cm.min))
	}
	return strings.Join(parts, ", ")
}

func topCategory(cats map[string]int) (string, int) {
	var best string
	var bestMin int
	for cat, mins := range cats {
		if mins > bestMin {
			best = cat
			bestMin = mins
		}
	}
	return best, bestMin
}

// WithEnrichment sets the enrichment provider on the service.
func WithEnrichment(ep EnrichmentProvider) func(*service) {
	return func(s *service) { s.enrichment = ep }
}
