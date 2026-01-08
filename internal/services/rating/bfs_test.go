package rating

import "testing"

func TestCalculateBFS(t *testing.T) {
	tests := []struct {
		name           string
		input          Fundamentals
		wantScore      int
		wantIndicators int
	}{
		{
			name: "strong foundation - all indicators",
			input: Fundamentals{
				RevenueTTM:        15_000_000, // > $10M
				CashBalance:       10_000_000,
				QuarterlyCashBurn: 100_000, // Runway > 18mo
				HasProducingAsset: true,
				IsProfitable:      true,
			},
			wantScore:      2,
			wantIndicators: 4,
		},
		{
			name: "weak foundation - no indicators",
			input: Fundamentals{
				RevenueTTM:        5_000_000, // < $10M
				CashBalance:       100_000,
				QuarterlyCashBurn: 500_000, // Runway < 18mo
				HasProducingAsset: false,
				IsProfitable:      false,
			},
			wantScore:      0,
			wantIndicators: 0,
		},
		{
			name: "developing - one indicator (revenue only)",
			input: Fundamentals{
				RevenueTTM:        15_000_000, // > $10M (1 indicator)
				CashBalance:       100_000,
				QuarterlyCashBurn: 500_000,
				HasProducingAsset: false,
				IsProfitable:      false,
			},
			wantScore:      1,
			wantIndicators: 1,
		},
		{
			name: "strong - two indicators",
			input: Fundamentals{
				RevenueTTM:        15_000_000, // > $10M
				CashBalance:       100_000,
				QuarterlyCashBurn: 500_000,
				HasProducingAsset: false,
				IsProfitable:      true, // 2nd indicator
			},
			wantScore:      2,
			wantIndicators: 2,
		},
		{
			name: "infinite runway when no burn",
			input: Fundamentals{
				RevenueTTM:        5_000_000, // < $10M
				CashBalance:       1_000_000,
				QuarterlyCashBurn: 0, // No burn = infinite runway
				HasProducingAsset: false,
				IsProfitable:      false,
			},
			wantScore:      1, // Cash runway indicator met
			wantIndicators: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateBFS(tt.input)
			if result.Score != tt.wantScore {
				t.Errorf("CalculateBFS() score = %d, want %d", result.Score, tt.wantScore)
			}
			if result.IndicatorCount != tt.wantIndicators {
				t.Errorf("CalculateBFS() indicators = %d, want %d", result.IndicatorCount, tt.wantIndicators)
			}
			if result.Reasoning == "" {
				t.Error("CalculateBFS() reasoning should not be empty")
			}
		})
	}
}
