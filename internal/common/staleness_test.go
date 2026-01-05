package common

import (
	"testing"
	"time"

	"github.com/ternarybob/quaero/internal/eodhd"
)

// Helper to create a time easily
func mustTime(t *testing.T, layout, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(layout, value)
	if err != nil {
		t.Fatalf("failed to parse time %q: %v", value, err)
	}
	return parsed
}

// newTestMetadata creates test metadata with sensible defaults
func newTestMetadata(code, timezone, closeTime string) *eodhd.ExchangeMetadata {
	return &eodhd.ExchangeMetadata{
		Code:             code,
		Timezone:         timezone,
		CloseTime:        closeTime,
		DataDelayMinutes: 180, // 3 hours default
		WorkingDays:      eodhd.DefaultWorkingDays(),
		Holidays:         []time.Time{},
	}
}

func TestIsWorkingDay(t *testing.T) {
	workingDays := eodhd.DefaultWorkingDays() // Mon-Fri

	tests := []struct {
		name        string
		date        string
		holidays    []string
		wantWorking bool
	}{
		{"monday", "2025-01-06", nil, true},
		{"tuesday", "2025-01-07", nil, true},
		{"wednesday", "2025-01-08", nil, true},
		{"thursday", "2025-01-09", nil, true},
		{"friday", "2025-01-10", nil, true},
		{"saturday", "2025-01-11", nil, false},
		{"sunday", "2025-01-12", nil, false},
		{"holiday on monday", "2025-01-06", []string{"2025-01-06"}, false},
		{"holiday on different day", "2025-01-07", []string{"2025-01-06"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			date := mustTime(t, "2006-01-02", tt.date)
			var holidays []time.Time
			for _, h := range tt.holidays {
				holidays = append(holidays, mustTime(t, "2006-01-02", h))
			}

			got := IsWorkingDay(date, workingDays, holidays)
			if got != tt.wantWorking {
				t.Errorf("IsWorkingDay(%s) = %v, want %v", tt.date, got, tt.wantWorking)
			}
		})
	}
}

func TestGetLastTradingDay(t *testing.T) {
	workingDays := eodhd.DefaultWorkingDays()

	tests := []struct {
		name     string
		date     string
		holidays []string
		want     string
	}{
		{"weekday returns same day", "2025-01-08", nil, "2025-01-08"},                                  // Wednesday
		{"saturday returns friday", "2025-01-11", nil, "2025-01-10"},                                   // Saturday -> Friday
		{"sunday returns friday", "2025-01-12", nil, "2025-01-10"},                                     // Sunday -> Friday
		{"monday returns monday", "2025-01-06", nil, "2025-01-06"},                                     // Monday
		{"holiday returns previous day", "2025-01-06", []string{"2025-01-06"}, "2025-01-03"},           // Mon holiday -> Fri
		{"two consecutive holidays", "2025-01-07", []string{"2025-01-06", "2025-01-07"}, "2025-01-03"}, // Mon+Tue holidays -> Fri
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			date := mustTime(t, "2006-01-02", tt.date)
			var holidays []time.Time
			for _, h := range tt.holidays {
				holidays = append(holidays, mustTime(t, "2006-01-02", h))
			}

			got := GetLastTradingDay(date, workingDays, holidays)
			gotStr := got.Format("2006-01-02")
			if gotStr != tt.want {
				t.Errorf("GetLastTradingDay(%s) = %s, want %s", tt.date, gotStr, tt.want)
			}
		})
	}
}

func TestGetNextTradingDay(t *testing.T) {
	workingDays := eodhd.DefaultWorkingDays()

	tests := []struct {
		name     string
		date     string
		holidays []string
		want     string
	}{
		{"monday returns tuesday", "2025-01-06", nil, "2025-01-07"},
		{"friday returns monday", "2025-01-10", nil, "2025-01-13"},
		{"saturday returns monday", "2025-01-11", nil, "2025-01-13"},
		{"with monday holiday", "2025-01-10", []string{"2025-01-13"}, "2025-01-14"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			date := mustTime(t, "2006-01-02", tt.date)
			var holidays []time.Time
			for _, h := range tt.holidays {
				holidays = append(holidays, mustTime(t, "2006-01-02", h))
			}

			got := GetNextTradingDay(date, workingDays, holidays)
			gotStr := got.Format("2006-01-02")
			if gotStr != tt.want {
				t.Errorf("GetNextTradingDay(%s) = %s, want %s", tt.date, gotStr, tt.want)
			}
		})
	}
}

func TestGetDataAvailableTime(t *testing.T) {
	tests := []struct {
		name         string
		tradingDay   string
		closeTime    string
		timezone     string
		delayMinutes int
		wantHour     int // Expected hour in UTC
	}{
		{
			name:         "Sydney 4PM + 3h delay",
			tradingDay:   "2025-01-08",
			closeTime:    "16:00",
			timezone:     "Australia/Sydney",
			delayMinutes: 180,
			wantHour:     8, // 16:00 AEDT (UTC+11) + 3h = 19:00 AEDT = 08:00 UTC
		},
		{
			name:         "New York 4PM + 15min delay",
			tradingDay:   "2025-01-08",
			closeTime:    "16:00",
			timezone:     "America/New_York",
			delayMinutes: 15,
			wantHour:     21, // 16:00 EST (UTC-5) + 15min = 16:15 EST = 21:15 UTC
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tradingDay := mustTime(t, "2006-01-02", tt.tradingDay)

			got, err := GetDataAvailableTime(tradingDay, tt.closeTime, tt.timezone, tt.delayMinutes)
			if err != nil {
				t.Fatalf("GetDataAvailableTime() error = %v", err)
			}

			if got.Hour() != tt.wantHour {
				t.Errorf("GetDataAvailableTime() hour = %d, want %d (full time: %v)", got.Hour(), tt.wantHour, got)
			}
		})
	}
}

func TestGetDataAvailableTime_InvalidTimezone(t *testing.T) {
	tradingDay := mustTime(t, "2006-01-02", "2025-01-08")
	_, err := GetDataAvailableTime(tradingDay, "16:00", "Invalid/Timezone", 180)
	if err == nil {
		t.Error("GetDataAvailableTime() expected error for invalid timezone")
	}
}

func TestCheckTickerStaleness(t *testing.T) {
	// Sydney timezone metadata (16:00 AEDT = 05:00 UTC in summer)
	// Data available at 16:00 + 3h = 19:00 AEDT = 08:00 UTC
	sydneyMeta := newTestMetadata("AU", "Australia/Sydney", "16:00")

	tests := []struct {
		name      string
		docDate   string
		now       string // Format: "2006-01-02 15:04 MST"
		meta      *eodhd.ExchangeMetadata
		wantStale bool
	}{
		{
			name:      "stale - doc older than last trading day, after data available",
			docDate:   "2025-01-07",           // Tuesday
			now:       "2025-01-08 10:00 UTC", // Wednesday 10:00 UTC (after Tuesday data available at 08:00 UTC)
			meta:      sydneyMeta,
			wantStale: true,
		},
		{
			name:      "fresh - doc matches last trading day",
			docDate:   "2025-01-08",           // Wednesday
			now:       "2025-01-08 10:00 UTC", // Wednesday 10:00 UTC (same day)
			meta:      sydneyMeta,
			wantStale: false,
		},
		{
			name:      "fresh - data not yet available",
			docDate:   "2025-01-07",           // Tuesday
			now:       "2025-01-08 06:00 UTC", // Wednesday 6:00 UTC (before Tuesday data available at 08:00 UTC)
			meta:      sydneyMeta,
			wantStale: false,
		},
		{
			name:      "nil metadata - assume stale",
			docDate:   "2025-01-08",
			now:       "2025-01-09 10:00 UTC",
			meta:      nil,
			wantStale: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			docDate := mustTime(t, "2006-01-02", tt.docDate)
			now, err := time.Parse("2006-01-02 15:04 MST", tt.now)
			if err != nil {
				t.Fatalf("failed to parse now time: %v", err)
			}

			result := CheckTickerStaleness(docDate, now, tt.meta)
			if result.IsStale != tt.wantStale {
				t.Errorf("CheckTickerStaleness() IsStale = %v, want %v (reason: %s)", result.IsStale, tt.wantStale, result.Reason)
			}
		})
	}
}

func TestCheckTickerStaleness_NextCheckTime(t *testing.T) {
	meta := newTestMetadata("AU", "Australia/Sydney", "16:00")

	// Fresh data - should have NextCheckTime set
	// Wednesday 10:00 UTC - doc date is Wednesday, same as last trading day
	docDate := mustTime(t, "2006-01-02", "2025-01-08")                 // Wednesday
	now := mustTime(t, "2006-01-02 15:04 MST", "2025-01-08 10:00 UTC") // Wednesday 10:00 UTC

	result := CheckTickerStaleness(docDate, now, meta)
	if result.IsStale {
		t.Errorf("expected fresh data, got stale (reason: %s)", result.Reason)
	}
	if result.NextCheckTime.IsZero() {
		t.Errorf("expected NextCheckTime to be set for fresh data")
	}
}

func TestCheckTickerStaleness_Weekend(t *testing.T) {
	meta := newTestMetadata("AU", "Australia/Sydney", "16:00")

	// Friday doc, checked on Sunday - should still be fresh
	docDate := mustTime(t, "2006-01-02", "2025-01-10")                 // Friday
	now := mustTime(t, "2006-01-02 15:04 MST", "2025-01-12 10:00 UTC") // Sunday

	result := CheckTickerStaleness(docDate, now, meta)
	if result.IsStale {
		t.Errorf("Friday data should still be fresh on Sunday (before Monday data available)")
	}
}

func TestDefaultExchangeMetadata(t *testing.T) {
	tests := []struct {
		code         string
		wantTimezone string
		wantDelay    int
	}{
		{"AU", "Australia/Sydney", 180},
		{"US", "America/New_York", 15},
		{"UNKNOWN", "UTC", 180},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			meta := DefaultExchangeMetadata(tt.code)

			if meta.Timezone != tt.wantTimezone {
				t.Errorf("Timezone = %s, want %s", meta.Timezone, tt.wantTimezone)
			}
			if meta.DataDelayMinutes != tt.wantDelay {
				t.Errorf("DataDelayMinutes = %d, want %d", meta.DataDelayMinutes, tt.wantDelay)
			}
			if len(meta.WorkingDays) != 5 {
				t.Errorf("WorkingDays length = %d, want 5", len(meta.WorkingDays))
			}
		})
	}
}
