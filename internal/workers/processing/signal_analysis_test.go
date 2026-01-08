// -----------------------------------------------------------------------
// Signal Analysis Tests - Unit tests for schema validation and classification
// -----------------------------------------------------------------------

package processing

import (
	"testing"
	"time"
)

// TestSignalAnalysisSchemaValidation tests schema validation
func TestSignalAnalysisSchemaValidation(t *testing.T) {
	tests := []struct {
		name        string
		schema      *SignalAnalysisSchema
		expectValid bool
	}{
		{
			name: "valid schema with all required fields",
			schema: &SignalAnalysisSchema{
				Ticker:       "CBA.AU",
				AnalysisDate: time.Now(),
				PeriodStart:  time.Now().AddDate(-1, 0, 0),
				PeriodEnd:    time.Now(),
				Summary: SignalSummary{
					TotalAnnouncements: 10,
					ConvictionScore:    5,
					CommunicationStyle: StyleStandard,
					SignalRatio:        0.2,
					LeakScore:          0.1,
					CredibilityScore:   0.8,
					NoiseRatio:         0.1,
				},
				Classifications: []AnnouncementClassification{
					{
						Date:           time.Now(),
						Title:          "Test Announcement",
						Classification: ClassificationRoutine,
						Metrics:        ClassificationMetrics{},
					},
				},
				Flags: RiskFlags{},
				DataSource: DataSourceMeta{
					AnnouncementsDocID: "doc-123",
					EODDocID:           "doc-456",
					AnnouncementsDate:  time.Now(),
					EODDate:            time.Now(),
					GeneratedAt:        time.Now(),
				},
			},
			expectValid: true,
		},
		{
			name: "invalid - missing ticker",
			schema: &SignalAnalysisSchema{
				Ticker:       "", // Missing
				AnalysisDate: time.Now(),
				PeriodStart:  time.Now().AddDate(-1, 0, 0),
				PeriodEnd:    time.Now(),
				Summary: SignalSummary{
					ConvictionScore:    5,
					CommunicationStyle: StyleStandard,
				},
				Classifications: []AnnouncementClassification{},
				Flags:           RiskFlags{},
				DataSource: DataSourceMeta{
					AnnouncementsDocID: "doc-123",
					EODDocID:           "doc-456",
					AnnouncementsDate:  time.Now(),
					EODDate:            time.Now(),
					GeneratedAt:        time.Now(),
				},
			},
			expectValid: false,
		},
		{
			name: "invalid - conviction score out of range",
			schema: &SignalAnalysisSchema{
				Ticker:       "CBA.AU",
				AnalysisDate: time.Now(),
				PeriodStart:  time.Now().AddDate(-1, 0, 0),
				PeriodEnd:    time.Now(),
				Summary: SignalSummary{
					ConvictionScore:    15, // Out of range (max 10)
					CommunicationStyle: StyleStandard,
				},
				Classifications: []AnnouncementClassification{},
				Flags:           RiskFlags{},
				DataSource: DataSourceMeta{
					AnnouncementsDocID: "doc-123",
					EODDocID:           "doc-456",
					AnnouncementsDate:  time.Now(),
					EODDate:            time.Now(),
					GeneratedAt:        time.Now(),
				},
			},
			expectValid: false,
		},
		{
			name: "invalid - bad communication style",
			schema: &SignalAnalysisSchema{
				Ticker:       "CBA.AU",
				AnalysisDate: time.Now(),
				PeriodStart:  time.Now().AddDate(-1, 0, 0),
				PeriodEnd:    time.Now(),
				Summary: SignalSummary{
					ConvictionScore:    5,
					CommunicationStyle: "INVALID_STYLE", // Not in enum
				},
				Classifications: []AnnouncementClassification{},
				Flags:           RiskFlags{},
				DataSource: DataSourceMeta{
					AnnouncementsDocID: "doc-123",
					EODDocID:           "doc-456",
					AnnouncementsDate:  time.Now(),
					EODDate:            time.Now(),
					GeneratedAt:        time.Now(),
				},
			},
			expectValid: false,
		},
		{
			name: "invalid - signal ratio out of range",
			schema: &SignalAnalysisSchema{
				Ticker:       "CBA.AU",
				AnalysisDate: time.Now(),
				PeriodStart:  time.Now().AddDate(-1, 0, 0),
				PeriodEnd:    time.Now(),
				Summary: SignalSummary{
					ConvictionScore:    5,
					CommunicationStyle: StyleStandard,
					SignalRatio:        1.5, // Out of range (max 1.0)
				},
				Classifications: []AnnouncementClassification{},
				Flags:           RiskFlags{},
				DataSource: DataSourceMeta{
					AnnouncementsDocID: "doc-123",
					EODDocID:           "doc-456",
					AnnouncementsDate:  time.Now(),
					EODDate:            time.Now(),
					GeneratedAt:        time.Now(),
				},
			},
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.schema.Validate()
			if tt.expectValid && err != nil {
				t.Errorf("expected valid schema, got error: %v", err)
			}
			if !tt.expectValid && err == nil {
				t.Error("expected invalid schema, got no error")
			}
		})
	}
}

// TestClassifyAnnouncement tests the classification logic
func TestClassifyAnnouncement(t *testing.T) {
	tests := []struct {
		name           string
		metrics        ClassificationMetrics
		priceSensitive bool
		category       string
		expected       string
	}{
		{
			name: "TRUE_SIGNAL - high volume, high change, low pre-drift",
			metrics: ClassificationMetrics{
				VolumeRatio: 2.5,
				DayOfChange: 4.0,
				PreDrift:    0.5,
			},
			priceSensitive: true,
			expected:       ClassificationTrueSignal,
		},
		{
			name: "PRICED_IN - high pre-drift, low day change",
			metrics: ClassificationMetrics{
				VolumeRatio: 1.0,
				DayOfChange: 0.5,
				PreDrift:    3.0,
			},
			priceSensitive: true,
			expected:       ClassificationPricedIn,
		},
		{
			name: "SENTIMENT_NOISE - routine category with volume reaction",
			metrics: ClassificationMetrics{
				VolumeRatio: 2.0,
				DayOfChange: 1.0,
				PreDrift:    0.0,
			},
			priceSensitive: false,
			category:       "Appendix 3Y - Director Interest",
			expected:       ClassificationSentimentNoise,
		},
		{
			name: "MANAGEMENT_BLUFF - price sensitive with no market impact",
			metrics: ClassificationMetrics{
				VolumeRatio: 0.6,
				DayOfChange: 0.2,
				PreDrift:    0.0,
			},
			priceSensitive: true,
			expected:       ClassificationManagementBluff,
		},
		{
			name: "ROUTINE - default case",
			metrics: ClassificationMetrics{
				VolumeRatio: 1.0,
				DayOfChange: 0.5,
				PreDrift:    0.5,
			},
			priceSensitive: false,
			expected:       ClassificationRoutine,
		},
		{
			name: "TRUE_SIGNAL takes precedence over PRICED_IN",
			metrics: ClassificationMetrics{
				VolumeRatio: 3.0, // High volume
				DayOfChange: 5.0, // High change
				PreDrift:    1.5, // Still under 2%
			},
			priceSensitive: true,
			expected:       ClassificationTrueSignal,
		},
		{
			name: "SENTIMENT_NOISE - routine with price reaction",
			metrics: ClassificationMetrics{
				VolumeRatio: 1.2,
				DayOfChange: 2.5, // > 2%
				PreDrift:    0.0,
			},
			priceSensitive: false,
			category:       "Daily Share Buy-Back Notice",
			expected:       ClassificationSentimentNoise,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyAnnouncement(tt.metrics, tt.priceSensitive, tt.category)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestCalculateConvictionScore tests conviction scoring
func TestCalculateConvictionScore(t *testing.T) {
	tests := []struct {
		name            string
		summary         SignalSummary
		classifications []AnnouncementClassification
		expectedMin     int
		expectedMax     int
	}{
		{
			name: "baseline score of 5",
			summary: SignalSummary{
				CountPricedIn:        0,
				CountManagementBluff: 0,
				NoiseRatio:           0.1,
			},
			classifications: []AnnouncementClassification{},
			expectedMin:     5,
			expectedMax:     5,
		},
		{
			name: "penalized for PRICED_IN",
			summary: SignalSummary{
				CountPricedIn:        3,
				CountManagementBluff: 0,
				NoiseRatio:           0.1,
			},
			classifications: []AnnouncementClassification{},
			expectedMin:     1, // 5 - 3 = 2, but could be clamped
			expectedMax:     2,
		},
		{
			name: "penalized for MANAGEMENT_BLUFF",
			summary: SignalSummary{
				CountPricedIn:        0,
				CountManagementBluff: 2,
				NoiseRatio:           0.1,
			},
			classifications: []AnnouncementClassification{},
			expectedMin:     3,
			expectedMax:     3,
		},
		{
			name: "penalized for high noise ratio",
			summary: SignalSummary{
				CountPricedIn:        0,
				CountManagementBluff: 0,
				NoiseRatio:           0.4, // > 0.3
			},
			classifications: []AnnouncementClassification{},
			expectedMin:     4, // 5 - 0.5 = 4.5 → 5 when rounded
			expectedMax:     5,
		},
		{
			name: "boosted for high-volume TRUE_SIGNAL",
			summary: SignalSummary{
				CountPricedIn:        0,
				CountManagementBluff: 0,
				NoiseRatio:           0.1,
			},
			classifications: []AnnouncementClassification{
				{Classification: ClassificationTrueSignal, Metrics: ClassificationMetrics{VolumeRatio: 3.5}},
				{Classification: ClassificationTrueSignal, Metrics: ClassificationMetrics{VolumeRatio: 4.0}},
			},
			expectedMin: 7,
			expectedMax: 7,
		},
		{
			name: "clamped to minimum 1",
			summary: SignalSummary{
				CountPricedIn:        5,
				CountManagementBluff: 3,
				NoiseRatio:           0.5,
			},
			classifications: []AnnouncementClassification{},
			expectedMin:     1,
			expectedMax:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateConvictionScore(tt.summary, tt.classifications)
			if result < tt.expectedMin || result > tt.expectedMax {
				t.Errorf("expected score between %d-%d, got %d", tt.expectedMin, tt.expectedMax, result)
			}
		})
	}
}

// TestDetermineCommunicationStyle tests style classification
func TestDetermineCommunicationStyle(t *testing.T) {
	tests := []struct {
		name     string
		summary  SignalSummary
		expected string
	}{
		{
			name: "TRANSPARENT_DATA_DRIVEN - high credibility, low leak",
			summary: SignalSummary{
				CredibilityScore: 0.9,
				LeakScore:        0.05,
				NoiseRatio:       0.1,
			},
			expected: StyleTransparent,
		},
		{
			name: "LEAKY_INSIDER_RISK - high leak score",
			summary: SignalSummary{
				CredibilityScore: 0.9,
				LeakScore:        0.4, // > 0.3
				NoiseRatio:       0.1,
			},
			expected: StyleLeaky,
		},
		{
			name: "PROMOTIONAL_SENTIMENT - high noise ratio",
			summary: SignalSummary{
				CredibilityScore: 0.7,
				LeakScore:        0.1,
				NoiseRatio:       0.4, // > 0.3
			},
			expected: StylePromotional,
		},
		{
			name: "PROMOTIONAL_SENTIMENT - low credibility",
			summary: SignalSummary{
				CredibilityScore: 0.4, // < 0.5
				LeakScore:        0.1,
				NoiseRatio:       0.1,
			},
			expected: StylePromotional,
		},
		{
			name: "STANDARD - default case",
			summary: SignalSummary{
				CredibilityScore: 0.7,
				LeakScore:        0.15,
				NoiseRatio:       0.2,
			},
			expected: StyleStandard,
		},
		{
			name: "LEAKY takes precedence over TRANSPARENT",
			summary: SignalSummary{
				CredibilityScore: 0.9,
				LeakScore:        0.35, // > 0.3 triggers LEAKY first
				NoiseRatio:       0.05,
			},
			expected: StyleLeaky,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetermineCommunicationStyle(tt.summary)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestDeriveRiskFlags tests risk flag derivation
func TestDeriveRiskFlags(t *testing.T) {
	tests := []struct {
		name     string
		summary  SignalSummary
		expected RiskFlags
	}{
		{
			name: "all flags false",
			summary: SignalSummary{
				LeakScore:          0.1,
				NoiseRatio:         0.1,
				SignalRatio:        0.1,
				CredibilityScore:   0.7,
				TotalAnnouncements: 10,
			},
			expected: RiskFlags{
				HighLeakRisk:     false,
				SpeculativeBase:  false,
				ReliableSignals:  false,
				InsufficientData: false,
			},
		},
		{
			name: "high leak risk",
			summary: SignalSummary{
				LeakScore:          0.35, // > 0.3
				NoiseRatio:         0.1,
				SignalRatio:        0.1,
				CredibilityScore:   0.7,
				TotalAnnouncements: 10,
			},
			expected: RiskFlags{
				HighLeakRisk:     true,
				SpeculativeBase:  false,
				ReliableSignals:  false,
				InsufficientData: false,
			},
		},
		{
			name: "speculative base",
			summary: SignalSummary{
				LeakScore:          0.1,
				NoiseRatio:         0.35, // > 0.3
				SignalRatio:        0.1,
				CredibilityScore:   0.7,
				TotalAnnouncements: 10,
			},
			expected: RiskFlags{
				HighLeakRisk:     false,
				SpeculativeBase:  true,
				ReliableSignals:  false,
				InsufficientData: false,
			},
		},
		{
			name: "reliable signals",
			summary: SignalSummary{
				LeakScore:          0.1,
				NoiseRatio:         0.1,
				SignalRatio:        0.25, // > 0.2
				CredibilityScore:   0.85, // > 0.8
				TotalAnnouncements: 10,
			},
			expected: RiskFlags{
				HighLeakRisk:     false,
				SpeculativeBase:  false,
				ReliableSignals:  true,
				InsufficientData: false,
			},
		},
		{
			name: "insufficient data",
			summary: SignalSummary{
				LeakScore:          0.1,
				NoiseRatio:         0.1,
				SignalRatio:        0.1,
				CredibilityScore:   0.7,
				TotalAnnouncements: 3, // < 5
			},
			expected: RiskFlags{
				HighLeakRisk:     false,
				SpeculativeBase:  false,
				ReliableSignals:  false,
				InsufficientData: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DeriveRiskFlags(tt.summary)
			if result != tt.expected {
				t.Errorf("expected %+v, got %+v", tt.expected, result)
			}
		})
	}
}

// TestCalculateAggregates tests aggregate calculation
func TestCalculateAggregates(t *testing.T) {
	t.Run("empty classifications", func(t *testing.T) {
		result := CalculateAggregates([]AnnouncementClassification{})
		if result.TotalAnnouncements != 0 {
			t.Errorf("expected 0 total, got %d", result.TotalAnnouncements)
		}
		if result.ConvictionScore != 5 {
			t.Errorf("expected conviction score 5, got %d", result.ConvictionScore)
		}
		if result.CommunicationStyle != StyleStandard {
			t.Errorf("expected STANDARD style, got %s", result.CommunicationStyle)
		}
	})

	t.Run("mixed classifications", func(t *testing.T) {
		classifications := []AnnouncementClassification{
			{Classification: ClassificationTrueSignal, ManagementSensitive: true, Metrics: ClassificationMetrics{VolumeRatio: 2.5}},
			{Classification: ClassificationTrueSignal, ManagementSensitive: true, Metrics: ClassificationMetrics{VolumeRatio: 3.5}},
			{Classification: ClassificationPricedIn, ManagementSensitive: true},
			{Classification: ClassificationRoutine, ManagementSensitive: false},
			{Classification: ClassificationRoutine, ManagementSensitive: false},
		}

		result := CalculateAggregates(classifications)

		if result.TotalAnnouncements != 5 {
			t.Errorf("expected 5 total, got %d", result.TotalAnnouncements)
		}
		if result.CountTrueSignal != 2 {
			t.Errorf("expected 2 TRUE_SIGNAL, got %d", result.CountTrueSignal)
		}
		if result.CountPricedIn != 1 {
			t.Errorf("expected 1 PRICED_IN, got %d", result.CountPricedIn)
		}
		if result.CountRoutine != 2 {
			t.Errorf("expected 2 ROUTINE, got %d", result.CountRoutine)
		}

		// Check signal ratio: 2/5 = 0.4
		if result.SignalRatio < 0.39 || result.SignalRatio > 0.41 {
			t.Errorf("expected signal ratio ~0.4, got %f", result.SignalRatio)
		}

		// Check leak score: 1/3 price-sensitive = ~0.33
		if result.LeakScore < 0.32 || result.LeakScore > 0.34 {
			t.Errorf("expected leak score ~0.33, got %f", result.LeakScore)
		}
	})
}

// TestSchemaToMap tests ToMap conversion
func TestSchemaToMap(t *testing.T) {
	schema := &SignalAnalysisSchema{
		Ticker:       "CBA.AU",
		AnalysisDate: time.Now(),
		PeriodStart:  time.Now().AddDate(-1, 0, 0),
		PeriodEnd:    time.Now(),
		Summary: SignalSummary{
			TotalAnnouncements: 10,
			ConvictionScore:    7,
			CommunicationStyle: StyleTransparent,
		},
		Classifications: []AnnouncementClassification{},
		Flags:           RiskFlags{},
		DataSource: DataSourceMeta{
			AnnouncementsDocID: "doc-123",
			EODDocID:           "doc-456",
			AnnouncementsDate:  time.Now(),
			EODDate:            time.Now(),
			GeneratedAt:        time.Now(),
		},
	}

	result, err := schema.ToMap()
	if err != nil {
		t.Fatalf("ToMap failed: %v", err)
	}

	if result["ticker"] != "CBA.AU" {
		t.Errorf("expected ticker CBA.AU, got %v", result["ticker"])
	}

	summary, ok := result["summary"].(map[string]interface{})
	if !ok {
		t.Fatal("summary should be a map")
	}

	if summary["conviction_score"] != float64(7) {
		t.Errorf("expected conviction_score 7, got %v", summary["conviction_score"])
	}
}

// TestIsRoutineCategory tests routine category detection
func TestIsRoutineCategory(t *testing.T) {
	routineCategories := []string{
		"Appendix 3B - New Issue",
		"Appendix 3Y - Director Interest",
		"Daily Share Buy-Back Notice",
		"Change of Company Secretary",
		"APPENDIX 3X",
	}

	for _, category := range routineCategories {
		if !isRoutineCategory(category) {
			t.Errorf("expected %q to be routine", category)
		}
	}

	nonRoutineCategories := []string{
		"Quarterly Activities Report",
		"Annual Report",
		"Dividend Announcement",
		"Trading Update",
	}

	for _, category := range nonRoutineCategories {
		if isRoutineCategory(category) {
			t.Errorf("expected %q to NOT be routine", category)
		}
	}
}
