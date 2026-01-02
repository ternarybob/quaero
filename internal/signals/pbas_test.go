package signals

import "testing"

func TestPBASComputer_Compute(t *testing.T) {
	computer := NewPBASComputer(DefaultPBASConfig())

	tests := []struct {
		name       string
		raw        TickerRaw
		wantMin    float64
		wantMax    float64
		wantInterp string
	}{
		{
			name: "Strong business, moderate price - underpriced",
			raw: TickerRaw{
				HasFundamentals: true,
				Price:           PriceData{Return52WPct: -5.0}, // Negative price momentum
				Fundamentals: FundamentalsData{
					RevenueYoYPct:        50.0, // Very strong revenue
					OCFToEBITDA:          0.95, // Excellent cash
					EBITDAMarginDeltaYoY: 10.0, // Improving margins
					ROICPct:              30.0, // Strong ROIC
					Dilution12MPct:       0.0,  // No dilution
				},
			},
			wantMin:    0.65,
			wantMax:    0.99,
			wantInterp: "underpriced",
		},
		{
			name: "Weak business, strong price - overpriced",
			raw: TickerRaw{
				HasFundamentals: true,
				Price:           PriceData{Return52WPct: 80.0}, // Very strong price
				Fundamentals: FundamentalsData{
					RevenueYoYPct:        -5.0, // Negative revenue growth
					OCFToEBITDA:          0.30, // Poor cash
					EBITDAMarginDeltaYoY: -5.0, // Declining margins
					ROICPct:              0.0,  // No ROIC
					Dilution12MPct:       15.0, // High dilution
				},
			},
			wantMin:    0.01,
			wantMax:    0.25,
			wantInterp: "overpriced",
		},
		{
			name: "Balanced - neutral",
			raw: TickerRaw{
				HasFundamentals: true,
				Price:           PriceData{Return52WPct: 10.0},
				Fundamentals: FundamentalsData{
					RevenueYoYPct:        10.0,
					OCFToEBITDA:          0.75,
					EBITDAMarginDeltaYoY: 0.0,
					ROICPct:              12.0,
					Dilution12MPct:       2.0,
				},
			},
			wantMin:    0.40,
			wantMax:    0.60,
			wantInterp: "neutral",
		},
		{
			name: "No fundamentals",
			raw: TickerRaw{
				HasFundamentals: false,
				Price:           PriceData{Return52WPct: 20.0},
			},
			wantMin:    0.45,
			wantMax:    0.55,
			wantInterp: "neutral",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computer.Compute(tt.raw)

			if result.Score < tt.wantMin || result.Score > tt.wantMax {
				t.Errorf("PBAS = %v, want between %v and %v", result.Score, tt.wantMin, tt.wantMax)
			}
			if result.Interpretation != tt.wantInterp {
				t.Errorf("Interpretation = %v, want %v", result.Interpretation, tt.wantInterp)
			}
		})
	}
}

func TestSigmoid(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
		epsilon  float64
	}{
		{0, 0.5, 0.01},
		{5, 0.99, 0.01},
		{-5, 0.01, 0.01},
		{2, 0.88, 0.05},
		{-2, 0.12, 0.05},
	}

	for _, tt := range tests {
		result := sigmoid(tt.input)
		if result < tt.expected-tt.epsilon || result > tt.expected+tt.epsilon {
			t.Errorf("sigmoid(%v) = %v, want ~%v", tt.input, result, tt.expected)
		}
	}
}
