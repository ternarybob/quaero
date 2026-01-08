// -----------------------------------------------------------------------
// SignalAnalysisSchema - Schema definitions for announcement signal analysis
// Provides strongly-typed structures for signal classification output
// -----------------------------------------------------------------------

package processing

import (
	"encoding/json"
	"time"

	"github.com/go-playground/validator/v10"
)

// SignalAnalysisSchema is the root schema for signal analysis documents.
// All fields are validated using go-playground/validator tags.
type SignalAnalysisSchema struct {
	// Document identification
	Ticker       string    `json:"ticker" validate:"required"`
	AnalysisDate time.Time `json:"analysis_date" validate:"required"`
	PeriodStart  time.Time `json:"period_start" validate:"required"`
	PeriodEnd    time.Time `json:"period_end" validate:"required"`

	// Aggregate metrics
	Summary SignalSummary `json:"summary" validate:"required"`

	// Per-announcement classifications
	Classifications []AnnouncementClassification `json:"classifications" validate:"required,dive"`

	// Risk flags derived from metrics
	Flags RiskFlags `json:"flags" validate:"required"`

	// Data quality tracking
	DataGaps []string `json:"data_gaps,omitempty"`

	// Source document references
	DataSource DataSourceMeta `json:"data_source" validate:"required"`
}

// SignalSummary contains aggregate metrics for the analysis period.
type SignalSummary struct {
	// Total count
	TotalAnnouncements int `json:"total_announcements" validate:"gte=0"`

	// Overall scores
	ConvictionScore    int    `json:"conviction_score" validate:"min=1,max=10"`
	CommunicationStyle string `json:"communication_style" validate:"oneof=TRANSPARENT_DATA_DRIVEN PROMOTIONAL_SENTIMENT LEAKY_INSIDER_RISK STANDARD"`

	// Ratio metrics (0-1)
	SignalRatio      float64 `json:"signal_ratio" validate:"gte=0,lte=1"`
	LeakScore        float64 `json:"leak_score" validate:"gte=0,lte=1"`
	CredibilityScore float64 `json:"credibility_score" validate:"gte=0,lte=1"`
	NoiseRatio       float64 `json:"noise_ratio" validate:"gte=0,lte=1"`

	// Classification counts
	CountTrueSignal      int `json:"count_true_signal"`
	CountPricedIn        int `json:"count_priced_in"`
	CountSentimentNoise  int `json:"count_sentiment_noise"`
	CountManagementBluff int `json:"count_management_bluff"`
	CountRoutine         int `json:"count_routine"`
}

// AnnouncementClassification contains the classification for a single announcement.
type AnnouncementClassification struct {
	// Announcement identification
	Date  time.Time `json:"date" validate:"required"`
	Title string    `json:"title" validate:"required"`

	// Optional metadata
	Category string `json:"category,omitempty"`

	// Management's classification
	ManagementSensitive bool `json:"management_sensitive"`

	// Our classification
	Classification string `json:"classification" validate:"required,oneof=TRUE_SIGNAL PRICED_IN SENTIMENT_NOISE MANAGEMENT_BLUFF ROUTINE"`

	// Metrics used for classification
	Metrics ClassificationMetrics `json:"metrics" validate:"required"`
}

// ClassificationMetrics contains the numeric data used to classify an announcement.
type ClassificationMetrics struct {
	// Percentage changes
	DayOfChange float64 `json:"day_of_change"` // % change on announcement day
	PreDrift    float64 `json:"pre_drift"`     // % change T-5 to T-1
	PostDrift   float64 `json:"post_drift"`    // % change T to T+5

	// Volume metrics
	VolumeRatio  float64 `json:"volume_ratio"`   // day_volume / avg_30d_volume
	DayVolume    int64   `json:"day_volume"`     // Volume on announcement day
	AvgVolume30d int64   `json:"avg_volume_30d"` // 30-day average volume

	// Raw prices for transparency
	AnnouncementClose float64 `json:"announcement_close"` // Close on announcement day
	PreviousClose     float64 `json:"previous_close"`     // Close on T-1
}

// RiskFlags contains boolean risk indicators derived from summary metrics.
type RiskFlags struct {
	HighLeakRisk     bool `json:"high_leak_risk"`    // leak_score > 0.3
	SpeculativeBase  bool `json:"speculative_base"`  // noise_ratio > 0.3
	ReliableSignals  bool `json:"reliable_signals"`  // signal_ratio > 0.2 && credibility > 0.8
	InsufficientData bool `json:"insufficient_data"` // < 5 announcements
}

// DataSourceMeta tracks the source documents used for analysis.
type DataSourceMeta struct {
	AnnouncementsDocID string    `json:"announcements_doc_id" validate:"required"`
	EODDocID           string    `json:"eod_doc_id" validate:"required"`
	AnnouncementsDate  time.Time `json:"announcements_date" validate:"required"`
	EODDate            time.Time `json:"eod_date" validate:"required"`
	GeneratedAt        time.Time `json:"generated_at" validate:"required"`
}

// Validate validates the schema using go-playground/validator.
// Returns an error if any required fields are missing or invalid.
func (s *SignalAnalysisSchema) Validate() error {
	validate := validator.New()
	return validate.Struct(s)
}

// ToMap converts the schema to a map for document metadata.
// Uses JSON marshal/unmarshal for clean type conversion.
func (s *SignalAnalysisSchema) ToMap() (map[string]interface{}, error) {
	// Marshal to JSON
	data, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}

	// Unmarshal to map
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// NewSignalAnalysisSchema creates a new schema with default values.
func NewSignalAnalysisSchema(ticker string) *SignalAnalysisSchema {
	now := time.Now()
	return &SignalAnalysisSchema{
		Ticker:          ticker,
		AnalysisDate:    now,
		PeriodStart:     now.AddDate(-1, 0, 0), // Default 1 year
		PeriodEnd:       now,
		Classifications: []AnnouncementClassification{},
		DataGaps:        []string{},
		Summary: SignalSummary{
			ConvictionScore:    5, // Default neutral score
			CommunicationStyle: "STANDARD",
		},
	}
}
