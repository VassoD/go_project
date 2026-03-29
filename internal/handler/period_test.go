package handler

import (
	"testing"
	"time"
	"vtc-service/internal/model"
)

// parseTime is a helper to create a time.Time from a string in Paris timezone.
func parseTime(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04:05", s, france)
	if err != nil {
		panic("invalid time in test: " + s)
	}
	return t
}

func withCurrentTime(t *testing.T, now time.Time) {
	t.Helper()
	previousNow := currentTime
	currentTime = func() time.Time { return now.In(france) }
	t.Cleanup(func() {
		currentTime = previousNow
	})
}

// TestPeriodStart_Daily verifies daily window boundaries.
func TestPeriodStart_Daily(t *testing.T) {
	tests := []struct {
		name     string
		now      string
		tripTime string
		wantIn   bool
	}{
		{
			name:     "trip at start of day is included",
			now:      "2026-03-28 14:00:00",
			tripTime: "2026-03-28 00:00:00",
			wantIn:   true,
		},
		{
			name:     "trip before now is included",
			now:      "2026-03-28 14:00:00",
			tripTime: "2026-03-28 13:59:59",
			wantIn:   true,
		},
		{
			name:     "future trip same day is excluded",
			now:      "2026-03-28 14:00:00",
			tripTime: "2026-03-28 23:59:59",
			wantIn:   false,
		},
		{
			name:     "trip yesterday at 23:59 is excluded",
			now:      "2026-03-28 14:00:00",
			tripTime: "2026-03-27 23:59:59",
			wantIn:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			now := parseTime(tc.now)
			tripTime := parseTime(tc.tripTime)
			withCurrentTime(t, now)
			got := sumTripsForPeriod([]model.Trip{{DriverID: "d1", Timestamp: tripTime, Amount: 1}}, "daily") > 0
			if got != tc.wantIn {
				t.Errorf("trip at %s with now=%s: expected in=%v, got in=%v", tc.tripTime, tc.now, tc.wantIn, got)
			}
		})
	}
}

// TestPeriodStart_Weekly verifies weekly window boundaries (ISO week: Monday–Sunday).
func TestPeriodStart_Weekly(t *testing.T) {
	tests := []struct {
		name     string
		now      string
		tripTime string
		wantIn   bool
	}{
		{
			name:     "trip on Monday 00:00 is included",
			now:      "2026-03-25 12:00:00", // Wednesday
			tripTime: "2026-03-23 00:00:00", // Monday
			wantIn:   true,
		},
		{
			name:     "future trip later this week is excluded",
			now:      "2026-03-25 12:00:00", // Wednesday
			tripTime: "2026-03-29 23:59:59", // Sunday (future)
			wantIn:   false,
		},
		{
			name:     "trip on Sunday of previous week is excluded",
			now:      "2026-03-25 12:00:00", // Wednesday
			tripTime: "2026-03-22 23:59:59", // previous Sunday
			wantIn:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			now := parseTime(tc.now)
			tripTime := parseTime(tc.tripTime)
			withCurrentTime(t, now)
			got := sumTripsForPeriod([]model.Trip{{DriverID: "d1", Timestamp: tripTime, Amount: 1}}, "weekly") > 0
			if got != tc.wantIn {
				t.Errorf("trip at %s with now=%s: expected in=%v, got in=%v", tc.tripTime, tc.now, tc.wantIn, got)
			}
		})
	}
}

// TestPeriodStart_Monthly verifies monthly window boundaries.
func TestPeriodStart_Monthly(t *testing.T) {
	tests := []struct {
		name     string
		now      string
		tripTime string
		wantIn   bool
	}{
		{
			name:     "trip on 1st of month at 00:00 is included",
			now:      "2026-03-25 12:00:00",
			tripTime: "2026-03-01 00:00:00",
			wantIn:   true,
		},
		{
			name:     "future trip later this month is excluded",
			now:      "2026-03-25 12:00:00",
			tripTime: "2026-03-31 23:59:59",
			wantIn:   false,
		},
		{
			name:     "trip on last day of previous month is excluded",
			now:      "2026-03-25 12:00:00",
			tripTime: "2026-02-28 23:59:59",
			wantIn:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			now := parseTime(tc.now)
			tripTime := parseTime(tc.tripTime)
			withCurrentTime(t, now)
			got := sumTripsForPeriod([]model.Trip{{DriverID: "d1", Timestamp: tripTime, Amount: 1}}, "monthly") > 0
			if got != tc.wantIn {
				t.Errorf("trip at %s with now=%s: expected in=%v, got in=%v", tc.tripTime, tc.now, tc.wantIn, got)
			}
		})
	}
}

// TestPeriodStart_Monthly_EdgeMonths verifies months with different lengths.
func TestPeriodStart_Monthly_EdgeMonths(t *testing.T) {
	tests := []struct {
		name     string
		now      string
		tripTime string
		wantIn   bool
	}{
		{
			name:     "February: trip on Feb 1st included (28-day month)",
			now:      "2026-02-15 12:00:00",
			tripTime: "2026-02-01 00:00:00",
			wantIn:   true,
		},
		{
			name:     "February: trip on Jan 31st excluded (28-day month)",
			now:      "2026-02-15 12:00:00",
			tripTime: "2026-01-31 23:59:59",
			wantIn:   false,
		},
		{
			name:     "February leap year: trip on Feb 29th included",
			now:      "2028-02-29 12:00:00", // 2028 is a leap year
			tripTime: "2028-02-29 06:00:00", // before now, same day
			wantIn:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			now := parseTime(tc.now)
			tripTime := parseTime(tc.tripTime)
			withCurrentTime(t, now)
			got := sumTripsForPeriod([]model.Trip{{DriverID: "d1", Timestamp: tripTime, Amount: 1}}, "monthly") > 0
			if got != tc.wantIn {
				t.Errorf("trip at %s with now=%s: expected in=%v, got in=%v", tc.tripTime, tc.now, tc.wantIn, got)
			}
		})
	}
}
