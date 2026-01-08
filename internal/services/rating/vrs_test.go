package rating

import (
	"testing"
	"time"
)

func TestCalculateVRS(t *testing.T) {
	baseDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	// Helper to generate price bars with specified volatility pattern
	makeStablePrices := func(days int, basePrice float64) []PriceBar {
		bars := make([]PriceBar, days)
		for i := 0; i < days; i++ {
			// Small random-ish variation (stable volatility)
			variation := 0.005 * float64(i%5) // 0.5% max variation
			close := basePrice * (1 + variation)
			bars[i] = PriceBar{
				Date:  baseDate.AddDate(0, 0, i),
				Open:  close - 0.01,
				High:  close + 0.01,
				Low:   close - 0.01,
				Close: close,
			}
		}
		return bars
	}

	makeVolatilePrices := func(days int, basePrice float64) []PriceBar {
		bars := make([]PriceBar, days)
		for i := 0; i < days; i++ {
			// Large random variation (high volatility with frequent regime changes)
			// Use different patterns throughout to create regime transitions
			var variation float64
			if i < 20 {
				variation = 0.30 * float64(i%5) / 5 // High volatility at start
			} else if i < 40 {
				variation = 0.05 * float64(i%3) / 3 // Low volatility
			} else if i < 60 {
				variation = 0.40 * float64(i%7) / 7 // Very high volatility
			} else if i < 80 {
				variation = 0.02 * float64(i%2) // Very low
			} else {
				variation = 0.50 * float64(i%10) / 10 // Extremely high
			}
			if i%2 == 0 {
				variation = -variation
			}
			close := basePrice * (1 + variation)
			bars[i] = PriceBar{
				Date:  baseDate.AddDate(0, 0, i),
				Open:  close - 0.05,
				High:  close + 0.10,
				Low:   close - 0.10,
				Close: close,
			}
		}
		return bars
	}

	tests := []struct {
		name     string
		prices   []PriceBar
		minScore float64
		maxScore float64
	}{
		{
			name:     "insufficient data - neutral",
			prices:   makeStablePrices(20, 1.0), // Less than 40 bars
			minScore: 0.49,
			maxScore: 0.51,
		},
		{
			name:     "stable volatility - high score",
			prices:   makeStablePrices(100, 1.0),
			minScore: 0.6,
			maxScore: 1.0,
		},
		{
			name:     "volatile - lower score",
			prices:   makeVolatilePrices(100, 1.0),
			minScore: 0.0,
			maxScore: 0.7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateVRS(tt.prices)

			if result.Score < tt.minScore || result.Score > tt.maxScore {
				t.Errorf("CalculateVRS() score = %f, want between %f and %f",
					result.Score, tt.minScore, tt.maxScore)
			}
			if result.Reasoning == "" {
				t.Error("CalculateVRS() reasoning should not be empty")
			}
		})
	}
}

func TestClassifyRegimes(t *testing.T) {
	tests := []struct {
		name       string
		rollingVol []float64
		wantLen    int
	}{
		{
			name:       "empty",
			rollingVol: []float64{},
			wantLen:    0,
		},
		{
			name:       "three values - one of each regime",
			rollingVol: []float64{0.01, 0.05, 0.10},
			wantLen:    3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regimes := classifyRegimes(tt.rollingVol)
			if len(regimes) != tt.wantLen {
				t.Errorf("classifyRegimes() len = %d, want %d", len(regimes), tt.wantLen)
			}
		})
	}
}

func TestCountTransitions(t *testing.T) {
	tests := []struct {
		name    string
		regimes []string
		want    int
	}{
		{
			name:    "empty",
			regimes: []string{},
			want:    0,
		},
		{
			name:    "single",
			regimes: []string{"low"},
			want:    0,
		},
		{
			name:    "no transitions",
			regimes: []string{"low", "low", "low"},
			want:    0,
		},
		{
			name:    "two transitions",
			regimes: []string{"low", "medium", "high"},
			want:    2,
		},
		{
			name:    "back and forth",
			regimes: []string{"low", "high", "low", "high"},
			want:    3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countTransitions(tt.regimes)
			if got != tt.want {
				t.Errorf("countTransitions() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestDeterminePattern(t *testing.T) {
	tests := []struct {
		name    string
		regimes []string
		want    string
	}{
		{
			name:    "empty",
			regimes: []string{},
			want:    "unknown",
		},
		{
			name:    "predominantly low",
			regimes: []string{"low", "low", "low", "medium"},
			want:    "predominantly_low",
		},
		{
			name:    "predominantly high",
			regimes: []string{"high", "high", "high", "low"},
			want:    "predominantly_high",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determinePattern(tt.regimes)
			if got != tt.want {
				t.Errorf("determinePattern() = %s, want %s", got, tt.want)
			}
		})
	}
}
