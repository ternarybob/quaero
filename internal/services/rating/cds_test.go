package rating

import (
	"testing"
	"time"
)

func TestCalculateCDS(t *testing.T) {
	now := time.Now()
	threeYearsAgo := int64(100_000_000) // 100M shares

	tests := []struct {
		name          string
		fundamentals  Fundamentals
		announcements []Announcement
		months        int
		wantScore     int
	}{
		{
			name: "strong discipline - low dilution, few halts",
			fundamentals: Fundamentals{
				SharesOutstandingCurrent: 110_000_000, // 10% growth over 3 years (~3% CAGR)
				SharesOutstanding3YAgo:   &threeYearsAgo,
			},
			announcements: []Announcement{
				{Date: now.AddDate(0, -6, 0), Type: TypeTradingHalt},
			},
			months:    36,
			wantScore: 2,
		},
		{
			name: "moderate discipline - moderate CAGR with some halts",
			fundamentals: Fundamentals{
				SharesOutstandingCurrent: 150_000_000, // 50% growth (~14.5% CAGR)
				SharesOutstanding3YAgo:   &threeYearsAgo,
			},
			announcements: []Announcement{
				{Date: now.AddDate(0, -6, 0), Type: TypeTradingHalt},
				{Date: now.AddDate(0, -12, 0), Type: TypeTradingHalt},
				{Date: now.AddDate(0, -18, 0), Type: TypeTradingHalt},
				{Date: now.AddDate(0, -24, 0), Type: TypeTradingHalt}, // 4 halts in 3y = >2/yr
				{Date: now.AddDate(0, -30, 0), Type: TypeTradingHalt},
				{Date: now.AddDate(0, -33, 0), Type: TypeTradingHalt},
				{Date: now.AddDate(0, -35, 0), Type: TypeTradingHalt}, // 7 halts = >2/yr
			},
			months:    36,
			wantScore: 1, // CAGR ~14.5% (<=15% but >2 halts/yr triggers moderate)
		},
		{
			name: "poor discipline - high CAGR exceeds 30%",
			fundamentals: Fundamentals{
				SharesOutstandingCurrent: 250_000_000, // 150% growth (~35.7% CAGR)
				SharesOutstanding3YAgo:   &threeYearsAgo,
			},
			announcements: []Announcement{
				{Date: now.AddDate(0, -3, 0), Type: TypeTradingHalt},
				{Date: now.AddDate(0, -6, 0), Type: TypeTradingHalt},
				{Date: now.AddDate(0, -9, 0), Type: TypeTradingHalt},
				{Date: now.AddDate(0, -12, 0), Type: TypeTradingHalt},
				{Date: now.AddDate(0, -15, 0), Type: TypeTradingHalt},
			},
			months:    36,
			wantScore: 0, // CAGR ~35.7% > 30% triggers poor
		},
		{
			name: "poor discipline - many capital raises",
			fundamentals: Fundamentals{
				SharesOutstandingCurrent: 110_000_000,
				SharesOutstanding3YAgo:   &threeYearsAgo,
			},
			announcements: []Announcement{
				{Date: now.AddDate(0, -3, 0), Type: TypeCapitalRaise},
				{Date: now.AddDate(0, -6, 0), Type: TypeCapitalRaise},
				{Date: now.AddDate(0, -9, 0), Type: TypeCapitalRaise},
				{Date: now.AddDate(0, -12, 0), Type: TypeCapitalRaise},
			},
			months:    12,
			wantScore: 0, // 4 raises per year
		},
		{
			name: "no historical shares data defaults to 0 CAGR",
			fundamentals: Fundamentals{
				SharesOutstandingCurrent: 100_000_000,
				SharesOutstanding3YAgo:   nil,
			},
			announcements: []Announcement{},
			months:        36,
			wantScore:     2, // 0 CAGR = strong
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateCDS(tt.fundamentals, tt.announcements, tt.months)
			if result.Score != tt.wantScore {
				t.Errorf("CalculateCDS() score = %d, want %d (CAGR=%.2f%%, halts/yr=%.1f, raises/yr=%.1f)",
					result.Score, tt.wantScore,
					result.Components.SharesCAGR*100,
					result.Components.TradingHaltsPA,
					result.Components.CapitalRaisesPA)
			}
			if result.Reasoning == "" {
				t.Error("CalculateCDS() reasoning should not be empty")
			}
		})
	}
}

func TestCalculateSharesCAGR(t *testing.T) {
	tests := []struct {
		name      string
		current   int64
		threeYAgo *int64
		wantCAGR  float64
		tolerance float64
	}{
		{
			name:      "0% growth",
			current:   100,
			threeYAgo: ptr(int64(100)),
			wantCAGR:  0,
			tolerance: 0.001,
		},
		{
			name:      "nil historical",
			current:   200,
			threeYAgo: nil,
			wantCAGR:  0,
			tolerance: 0.001,
		},
		{
			name:      "zero historical",
			current:   200,
			threeYAgo: ptr(int64(0)),
			wantCAGR:  0,
			tolerance: 0.001,
		},
		{
			name:      "~26% CAGR (doubled in 3 years)",
			current:   200,
			threeYAgo: ptr(int64(100)),
			wantCAGR:  0.26, // (2)^(1/3) - 1 ≈ 26%
			tolerance: 0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateSharesCAGR(tt.current, tt.threeYAgo)
			if got < tt.wantCAGR-tt.tolerance || got > tt.wantCAGR+tt.tolerance {
				t.Errorf("calculateSharesCAGR() = %f, want %f", got, tt.wantCAGR)
			}
		})
	}
}

func ptr(v int64) *int64 {
	return &v
}
