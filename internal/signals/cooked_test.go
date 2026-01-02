package signals

import "testing"

func TestCookedDetector_Detect(t *testing.T) {
	detector := NewCookedDetector()

	tests := []struct {
		name       string
		raw        TickerRaw
		pbas       PBASSignal
		wantMin    int
		wantMax    int
		wantCooked bool
	}{
		{
			name: "Clean stock - not cooked",
			raw: TickerRaw{
				Price: PriceData{
					Return52WPct: 15.0,
					Current:      10.0,
					EMA200:       9.5,
				},
				Fundamentals: FundamentalsData{
					RevenueYoYPct:  10.0,
					Dilution12MPct: 2.0,
					OCFToEBITDA:    0.80,
				},
			},
			pbas:       PBASSignal{Score: 0.65},
			wantMin:    0,
			wantMax:    1,
			wantCooked: false,
		},
		{
			name: "Low PBAS only - borderline",
			raw: TickerRaw{
				Price: PriceData{
					Return52WPct: 10.0,
					Current:      10.0,
					EMA200:       9.5,
				},
				Fundamentals: FundamentalsData{
					RevenueYoYPct:  10.0,
					Dilution12MPct: 2.0,
					OCFToEBITDA:    0.80,
				},
			},
			pbas:       PBASSignal{Score: 0.25},
			wantMin:    1,
			wantMax:    1,
			wantCooked: false, // Only 1 trigger, need 2
		},
		{
			name: "Cooked - low PBAS + high dilution",
			raw: TickerRaw{
				Price: PriceData{
					Return52WPct: 20.0,
					Current:      10.0,
					EMA200:       9.5,
				},
				Fundamentals: FundamentalsData{
					RevenueYoYPct:  10.0,
					Dilution12MPct: 15.0, // High dilution
					OCFToEBITDA:    0.80,
				},
			},
			pbas:       PBASSignal{Score: 0.25}, // Low PBAS
			wantMin:    2,
			wantMax:    2,
			wantCooked: true,
		},
		{
			name: "Cooked - multiple triggers",
			raw: TickerRaw{
				Price: PriceData{
					Return52WPct: 50.0, // Price up a lot
					Current:      15.0,
					EMA200:       10.0, // Extended above 200EMA
				},
				Fundamentals: FundamentalsData{
					RevenueYoYPct:  5.0,  // Price >> revenue growth
					Dilution12MPct: 15.0, // High dilution
					OCFToEBITDA:    0.50, // Poor cash conversion
				},
			},
			pbas:       PBASSignal{Score: 0.25},
			wantMin:    3,
			wantMax:    5,
			wantCooked: true,
		},
		{
			name: "Price up but revenue down",
			raw: TickerRaw{
				Price: PriceData{
					Return52WPct: 25.0, // Price up
					Current:      10.0,
					EMA200:       9.5,
				},
				Fundamentals: FundamentalsData{
					RevenueYoYPct:  -5.0, // Revenue down
					Dilution12MPct: 3.0,
					OCFToEBITDA:    0.70,
				},
			},
			pbas:       PBASSignal{Score: 0.40},
			wantMin:    1,
			wantMax:    1,
			wantCooked: false, // Only 1 trigger
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.Detect(tt.raw, tt.pbas)

			if result.IsCooked != tt.wantCooked {
				t.Errorf("IsCooked = %v, want %v (score: %d, reasons: %v)",
					result.IsCooked, tt.wantCooked, result.Score, result.Reasons)
			}
			if result.Score < tt.wantMin || result.Score > tt.wantMax {
				t.Errorf("Score = %v, want between %v and %v (reasons: %v)",
					result.Score, tt.wantMin, tt.wantMax, result.Reasons)
			}
		})
	}
}
