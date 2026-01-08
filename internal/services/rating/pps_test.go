package rating

import (
	"testing"
	"time"
)

func TestCalculatePPS(t *testing.T) {
	baseDate := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)

	// Helper to generate price bars
	makePrices := func(dates []time.Time, closes []float64) []PriceBar {
		bars := make([]PriceBar, len(dates))
		for i := range dates {
			bars[i] = PriceBar{
				Date:  dates[i],
				Open:  closes[i] - 0.01,
				High:  closes[i] + 0.01,
				Low:   closes[i] - 0.02,
				Close: closes[i],
			}
		}
		return bars
	}

	tests := []struct {
		name          string
		announcements []Announcement
		prices        []PriceBar
		wantScore     float64
		minScore      float64
		maxScore      float64
	}{
		{
			name:          "no announcements - neutral",
			announcements: []Announcement{},
			prices:        makePrices([]time.Time{baseDate}, []float64{1.0}),
			wantScore:     0.5,
			minScore:      0.49,
			maxScore:      0.51,
		},
		{
			name: "no prices - neutral",
			announcements: []Announcement{
				{Date: baseDate, Headline: "Test", IsPriceSensitive: true},
			},
			prices:    []PriceBar{},
			wantScore: 0.5,
			minScore:  0.49,
			maxScore:  0.51,
		},
		{
			name: "price sensitive with good retention",
			announcements: []Announcement{
				{
					Date:             baseDate,
					Headline:         "Positive News",
					IsPriceSensitive: true,
				},
			},
			prices: makePrices(
				[]time.Time{
					baseDate.AddDate(0, 0, -1), // Before: $1.00
					baseDate,                   // Day 0: $1.10 (10% gain)
					baseDate.AddDate(0, 0, 1),
					baseDate.AddDate(0, 0, 2),
					baseDate.AddDate(0, 0, 3),
					baseDate.AddDate(0, 0, 4),
					baseDate.AddDate(0, 0, 5), // After: $1.08 (80% retention)
				},
				[]float64{1.0, 1.10, 1.09, 1.08, 1.09, 1.08, 1.08},
			),
			minScore: 0.7,
			maxScore: 1.0,
		},
		{
			name: "price sensitive with poor retention",
			announcements: []Announcement{
				{
					Date:             baseDate,
					Headline:         "Failed News",
					IsPriceSensitive: true,
				},
			},
			prices: makePrices(
				[]time.Time{
					baseDate.AddDate(0, 0, -1), // Before: $1.00
					baseDate,                   // Day 0: $1.10 (10% gain)
					baseDate.AddDate(0, 0, 1),
					baseDate.AddDate(0, 0, 2),
					baseDate.AddDate(0, 0, 3),
					baseDate.AddDate(0, 0, 4),
					baseDate.AddDate(0, 0, 5), // After: $1.01 (10% retention)
				},
				[]float64{1.0, 1.10, 1.05, 1.03, 1.02, 1.01, 1.01},
			),
			minScore: 0.0,
			maxScore: 0.3,
		},
		{
			name: "non-price-sensitive ignored",
			announcements: []Announcement{
				{
					Date:             baseDate,
					Headline:         "Routine Notice",
					IsPriceSensitive: false, // Should be skipped
				},
			},
			prices: makePrices(
				[]time.Time{
					baseDate.AddDate(0, 0, -1),
					baseDate,
					baseDate.AddDate(0, 0, 5),
				},
				[]float64{1.0, 1.10, 1.00}, // Would be poor retention if counted
			),
			wantScore: 0.5, // Neutral because no PS announcements processed
			minScore:  0.49,
			maxScore:  0.51,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculatePPS(tt.announcements, tt.prices)

			if result.Score < tt.minScore || result.Score > tt.maxScore {
				t.Errorf("CalculatePPS() score = %f, want between %f and %f",
					result.Score, tt.minScore, tt.maxScore)
			}
			if result.Reasoning == "" {
				t.Error("CalculatePPS() reasoning should not be empty")
			}
		})
	}
}

func TestCalculatePPS_SmallMovesIgnored(t *testing.T) {
	baseDate := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)

	// Price moves less than 2% should be ignored
	announcements := []Announcement{
		{
			Date:             baseDate,
			Headline:         "Minor News",
			IsPriceSensitive: true,
		},
	}

	prices := []PriceBar{
		{Date: baseDate.AddDate(0, 0, -1), Close: 1.00},
		{Date: baseDate, Close: 1.01}, // Only 1% move
		{Date: baseDate.AddDate(0, 0, 5), Close: 0.90},
	}

	result := CalculatePPS(announcements, prices)

	// Should be neutral since the move was too small
	if result.Score != 0.5 {
		t.Errorf("CalculatePPS() should return neutral (0.5) for small moves, got %f", result.Score)
	}
}
