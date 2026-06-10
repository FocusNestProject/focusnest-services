package progress

import (
	"testing"
	"time"
)

func TestGetLastProductiveDate(t *testing.T) {
	tests := []struct {
		name     string
		days     []DayStatus
		todayStr string
		expected string
	}{
		{
			name: "No productive days",
			days: []DayStatus{
				{Date: "2023-10-01", Status: "skipped"},
				{Date: "2023-10-02", Status: "skipped"},
			},
			todayStr: "2023-10-02",
			expected: "",
		},
		{
			name: "One productive day before today",
			days: []DayStatus{
				{Date: "2023-10-01", Status: "done"},
				{Date: "2023-10-02", Status: "skipped"},
				{Date: "2023-10-03", Status: "upcoming"},
			},
			todayStr: "2023-10-02",
			expected: "2023-10-01",
		},
		{
			name: "Productive day exactly today",
			days: []DayStatus{
				{Date: "2023-10-01", Status: "skipped"},
				{Date: "2023-10-02", Status: "done"},
				{Date: "2023-10-03", Status: "upcoming"},
			},
			todayStr: "2023-10-02",
			expected: "2023-10-02",
		},
		{
			name: "Productive day in the future (ignored)",
			days: []DayStatus{
				{Date: "2023-10-01", Status: "skipped"},
				{Date: "2023-10-02", Status: "skipped"},
				{Date: "2023-10-03", Status: "done"},
			},
			todayStr: "2023-10-02",
			expected: "",
		},
		{
			name: "Multiple productive days",
			days: []DayStatus{
				{Date: "2023-10-01", Status: "done"},
				{Date: "2023-10-02", Status: "done"},
				{Date: "2023-10-03", Status: "skipped"},
			},
			todayStr: "2023-10-03",
			expected: "2023-10-02",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			today, _ := time.Parse(dateLayout, tt.todayStr)
			actual := getLastProductiveDate(tt.days, today)
			if actual != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, actual)
			}
		})
	}
}

func TestStreakEndingOn(t *testing.T) {
	tests := []struct {
		name          string
		days          []DayStatus
		endDate       string
		expectedRun   int
		expectedOvf   bool
	}{
		{
			name: "Target date not found",
			days: []DayStatus{
				{Date: "2023-10-01", Status: "done"},
			},
			endDate:     "2023-10-02",
			expectedRun: 0,
			expectedOvf: false,
		},
		{
			name: "Target date not done",
			days: []DayStatus{
				{Date: "2023-10-01", Status: "done"},
				{Date: "2023-10-02", Status: "skipped"},
			},
			endDate:     "2023-10-02",
			expectedRun: 0,
			expectedOvf: false,
		},
		{
			name: "Short streak ending on target date",
			days: []DayStatus{
				{Date: "2023-10-01", Status: "skipped"},
				{Date: "2023-10-02", Status: "done"},
				{Date: "2023-10-03", Status: "done"},
				{Date: "2023-10-04", Status: "upcoming"},
			},
			endDate:     "2023-10-03",
			expectedRun: 2,
			expectedOvf: false,
		},
		{
			name: "Streak overflows past the start of the array",
			days: []DayStatus{
				{Date: "2023-10-01", Status: "done"},
				{Date: "2023-10-02", Status: "done"},
				{Date: "2023-10-03", Status: "done"},
			},
			endDate:     "2023-10-03",
			expectedRun: 3,
			expectedOvf: true,
		},
		{
			name: "Streak broken before target date",
			days: []DayStatus{
				{Date: "2023-10-01", Status: "done"},
				{Date: "2023-10-02", Status: "skipped"},
				{Date: "2023-10-03", Status: "done"},
				{Date: "2023-10-04", Status: "done"},
			},
			endDate:     "2023-10-04",
			expectedRun: 2,
			expectedOvf: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run, ovf := streakEndingOn(tt.days, tt.endDate)
			if run != tt.expectedRun {
				t.Errorf("expected run %d, got %d", tt.expectedRun, run)
			}
			if ovf != tt.expectedOvf {
				t.Errorf("expected overflow %v, got %v", tt.expectedOvf, ovf)
			}
		})
	}
}

func TestCalculateStreaks(t *testing.T) {
	srv := &service{}

	tests := []struct {
		name                  string
		days                  []DayStatus
		todayStr              string
		expectedTotalStreak   int
		expectedCurrentStreak int
		expectedOverflow      bool
	}{
		{
			name: "All skipped",
			days: []DayStatus{
				{Date: "2023-10-01", Status: "skipped"},
				{Date: "2023-10-02", Status: "skipped"},
				{Date: "2023-10-03", Status: "skipped"},
			},
			todayStr:              "2023-10-03",
			expectedTotalStreak:   0,
			expectedCurrentStreak: 0,
			expectedOverflow:      false,
		},
		{
			name: "Perfect streak (overflows)",
			days: []DayStatus{
				{Date: "2023-10-01", Status: "done"},
				{Date: "2023-10-02", Status: "done"},
				{Date: "2023-10-03", Status: "done"},
			},
			todayStr:              "2023-10-03",
			expectedTotalStreak:   3,
			expectedCurrentStreak: 3,
			expectedOverflow:      true,
		},
		{
			name: "Broken streak in middle",
			days: []DayStatus{
				{Date: "2023-10-01", Status: "done"},
				{Date: "2023-10-02", Status: "done"}, // streak of 2
				{Date: "2023-10-03", Status: "skipped"},
				{Date: "2023-10-04", Status: "done"}, // streak of 1
				{Date: "2023-10-05", Status: "upcoming"},
			},
			todayStr:              "2023-10-04",
			expectedTotalStreak:   2,
			expectedCurrentStreak: 1,
			expectedOverflow:      false,
		},
		{
			name: "Current streak maintained if today is skipped but yesterday was done",
			days: []DayStatus{
				{Date: "2023-10-01", Status: "done"},
				{Date: "2023-10-02", Status: "done"},
				{Date: "2023-10-03", Status: "skipped"}, // today is skipped, last productive is 10-02
			},
			todayStr:              "2023-10-03",
			expectedTotalStreak:   2,
			expectedCurrentStreak: 2,
			expectedOverflow:      true, // It overflows because it goes all the way to 10-01 which is the start
		},
		{
			name: "Multiple streaks, total is max",
			days: []DayStatus{
				{Date: "2023-10-01", Status: "done"},
				{Date: "2023-10-02", Status: "done"},
				{Date: "2023-10-03", Status: "done"}, // length 3
				{Date: "2023-10-04", Status: "skipped"},
				{Date: "2023-10-05", Status: "done"},
				{Date: "2023-10-06", Status: "done"}, // length 2
			},
			todayStr:              "2023-10-06",
			expectedTotalStreak:   3,
			expectedCurrentStreak: 2,
			expectedOverflow:      false,
		},
		{
			name: "Streak separated by month boundaries",
			days: []DayStatus{
				{Date: "2023-09-29", Status: "done"},
				{Date: "2023-09-30", Status: "done"},
				{Date: "2023-10-01", Status: "done"},
				{Date: "2023-10-02", Status: "done"},
				{Date: "2023-10-03", Status: "upcoming"},
			},
			todayStr:              "2023-10-02",
			expectedTotalStreak:   4,
			expectedCurrentStreak: 4,
			expectedOverflow:      true, // Overflows because it goes back to start of slice
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			today, _ := time.Parse(dateLayout, tt.todayStr)
			total, current, ovf := srv.calculateStreaks(tt.days, today)
			if total != tt.expectedTotalStreak {
				t.Errorf("expected total %d, got %d", tt.expectedTotalStreak, total)
			}
			if current != tt.expectedCurrentStreak {
				t.Errorf("expected current %d, got %d", tt.expectedCurrentStreak, current)
			}
			if ovf != tt.expectedOverflow {
				t.Errorf("expected overflow %v, got %v", tt.expectedOverflow, ovf)
			}
		})
	}
}
