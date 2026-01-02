package signals

import "testing"

func TestAssessmentValidator_Validate(t *testing.T) {
	validator := NewAssessmentValidator()

	tests := []struct {
		name       string
		assessment TickerAssessment
		sig        TickerSignals
		wantValid  bool
	}{
		{
			name: "Valid assessment",
			assessment: TickerAssessment{
				Ticker: "GNP",
				Decision: AssessmentDecision{
					Action:     ActionHold,
					Confidence: ConfidenceMedium,
				},
				Reasoning: AssessmentReasoning{
					Primary: "Test reasoning",
					Evidence: []string{
						"PBAS score of 0.72 exceeds threshold",
						"VLI at 0.45 indicates accumulation",
						"RS rank at 75th percentile",
					},
				},
			},
			sig: TickerSignals{
				PBAS: PBASSignal{Score: 0.72},
			},
			wantValid: true,
		},
		{
			name: "Too few evidence points",
			assessment: TickerAssessment{
				Ticker: "GNP",
				Reasoning: AssessmentReasoning{
					Evidence: []string{
						"PBAS 0.72",
						"VLI 0.45",
					},
				},
			},
			sig:       TickerSignals{},
			wantValid: false,
		},
		{
			name: "Evidence without numbers",
			assessment: TickerAssessment{
				Ticker: "GNP",
				Reasoning: AssessmentReasoning{
					Evidence: []string{
						"Strong fundamentals detected",
						"VLI at 0.45",
						"Good RS rank",
					},
				},
			},
			sig:       TickerSignals{},
			wantValid: false,
		},
		{
			name: "Generic phrase in evidence",
			assessment: TickerAssessment{
				Ticker: "GNP",
				Reasoning: AssessmentReasoning{
					Evidence: []string{
						"PBAS at 0.72 shows solid fundamentals",
						"VLI at 0.45 indicates accumulation",
						"RS at 75 percentile",
					},
				},
			},
			sig:       TickerSignals{},
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(tt.assessment, tt.sig)

			if result.Valid != tt.wantValid {
				t.Errorf("Valid = %v, want %v (errors: %v)",
					result.Valid, tt.wantValid, result.Errors)
			}
		})
	}
}

func TestAssessmentValidator_ActionConsistency(t *testing.T) {
	validator := NewAssessmentValidator()

	tests := []struct {
		name      string
		action    string
		sig       TickerSignals
		wantValid bool
	}{
		{
			name:   "Accumulate with good signals",
			action: ActionAccumulate,
			sig: TickerSignals{
				PBAS:   PBASSignal{Score: 0.70},
				Cooked: CookedSignal{IsCooked: false},
				Regime: RegimeSignal{Classification: string(RegimeTrendUp)},
			},
			wantValid: true,
		},
		{
			name:   "Accumulate with cooked stock - invalid",
			action: ActionAccumulate,
			sig: TickerSignals{
				PBAS:   PBASSignal{Score: 0.70},
				Cooked: CookedSignal{IsCooked: true},
				Regime: RegimeSignal{Classification: string(RegimeTrendUp)},
			},
			wantValid: false,
		},
		{
			name:   "Accumulate in decay regime - invalid",
			action: ActionAccumulate,
			sig: TickerSignals{
				PBAS:   PBASSignal{Score: 0.70},
				Cooked: CookedSignal{IsCooked: false},
				Regime: RegimeSignal{Classification: string(RegimeDecay)},
			},
			wantValid: false,
		},
		{
			name:   "Reduce with bad signals - valid",
			action: ActionReduce,
			sig: TickerSignals{
				PBAS:   PBASSignal{Score: 0.35},
				Cooked: CookedSignal{IsCooked: false},
			},
			wantValid: true,
		},
		{
			name:   "Reduce with cooked stock - valid",
			action: ActionReduce,
			sig: TickerSignals{
				PBAS:   PBASSignal{Score: 0.65},
				Cooked: CookedSignal{IsCooked: true},
			},
			wantValid: true,
		},
		{
			name:   "Hold is always valid",
			action: ActionHold,
			sig: TickerSignals{
				PBAS: PBASSignal{Score: 0.50},
			},
			wantValid: true,
		},
		{
			name:      "Watch is always valid",
			action:    ActionWatch,
			sig:       TickerSignals{},
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			consistent, warning := validator.checkActionConsistency(tt.action, tt.sig)

			if consistent != tt.wantValid {
				t.Errorf("Consistency = %v, want %v (warning: %s)",
					consistent, tt.wantValid, warning)
			}
		})
	}
}

func TestAssessmentValidator_ContainsNumber(t *testing.T) {
	validator := NewAssessmentValidator()

	tests := []struct {
		input     string
		hasNumber bool
	}{
		{"PBAS of 0.72", true},
		{"VLI at 45%", true},
		{"Score is 100", true},
		{"Strong growth", false},
		{"No numbers here", false},
		{"RS rank at 75th percentile", true},
		{"1st quarter results", true},
		{"3.14159", true},
	}

	for _, tt := range tests {
		result := validator.containsNumber(tt.input)
		if result != tt.hasNumber {
			t.Errorf("containsNumber(%q) = %v, want %v", tt.input, result, tt.hasNumber)
		}
	}
}
