// Package announcements provides pure functions for announcement classification
// and signal-to-noise analysis. No I/O or document awareness.
package announcements

import (
	"encoding/gob"
	"time"
)

func init() {
	// Register types with gob for BadgerDB serialization
	gob.Register([]ProcessedAnnouncement{})
	gob.Register(ProcessedAnnouncement{})
	gob.Register(RawAnnouncement{})
	gob.Register([]RawAnnouncement{})
	gob.Register(PriceImpactData{})
	gob.Register(PriceBar{})
	gob.Register([]PriceBar{})
	gob.Register(ProcessingSummary{})
	gob.Register(SignalNoiseRating(""))
	gob.Register(MQSScores{})
	gob.Register(DeduplicationStats{})
	gob.Register(DeduplicationGroup{})
	gob.Register([]DeduplicationGroup{})
	gob.Register(SignalNoiseResult{})
}

// RawAnnouncement represents an unprocessed ASX announcement
type RawAnnouncement struct {
	Date           time.Time `json:"date"`
	Headline       string    `json:"headline"`
	Type           string    `json:"type"`
	PDFURL         string    `json:"pdf_url"`
	DocumentKey    string    `json:"document_key"`
	PriceSensitive bool      `json:"price_sensitive"`
}

// ProcessedAnnouncement represents an announcement with analysis applied
type ProcessedAnnouncement struct {
	// Raw data
	Date           time.Time `json:"date"`
	Headline       string    `json:"headline"`
	Type           string    `json:"type"`
	PDFURL         string    `json:"pdf_url"`
	DocumentKey    string    `json:"document_key"`
	PriceSensitive bool      `json:"price_sensitive"`

	// Relevance classification (keyword-based)
	RelevanceCategory string `json:"relevance_category"` // HIGH, MEDIUM, LOW, NOISE
	RelevanceReason   string `json:"relevance_reason"`

	// Signal-to-noise analysis (market impact based)
	SignalNoiseRating    SignalNoiseRating `json:"signal_noise_rating"`
	SignalNoiseRationale string            `json:"signal_noise_rationale"`

	// Price impact data
	PriceImpact *PriceImpactData `json:"price_impact,omitempty"`

	// Detection flags
	IsTradingHalt          bool   `json:"is_trading_halt"`
	IsReinstatement        bool   `json:"is_reinstatement"`
	IsDividendAnnouncement bool   `json:"is_dividend_announcement"`
	IsRoutine              bool   `json:"is_routine"`
	RoutineType            string `json:"routine_type,omitempty"`

	// Anomaly detection
	IsAnomaly   bool   `json:"is_anomaly"`
	AnomalyType string `json:"anomaly_type,omitempty"` // "NO_REACTION", "UNEXPECTED_REACTION"

	// Critical review classification
	SignalClassification string `json:"signal_classification,omitempty"` // TRUE_SIGNAL, PRICED_IN, SENTIMENT_NOISE, MANAGEMENT_BLUFF, ROUTINE
}

// SignalNoiseRating represents the signal quality of an announcement based on market impact
type SignalNoiseRating string

const (
	// SignalNoiseHigh indicates significant market impact - high signal, low noise
	// Criteria: Price change >=3% OR volume ratio >=2x, typically with price-sensitive flag
	SignalNoiseHigh SignalNoiseRating = "HIGH_SIGNAL"

	// SignalNoiseModerate indicates notable market reaction
	// Criteria: Price change >=1.5% OR volume ratio >=1.5x
	SignalNoiseModerate SignalNoiseRating = "MODERATE_SIGNAL"

	// SignalNoiseLow indicates minimal market reaction
	// Criteria: Price change >=0.5% OR volume ratio >=1.2x
	SignalNoiseLow SignalNoiseRating = "LOW_SIGNAL"

	// SignalNoiseNone indicates no meaningful price/volume impact - pure noise
	// Criteria: Price change <0.5% AND volume ratio <1.2x
	SignalNoiseNone SignalNoiseRating = "NOISE"

	// SignalNoiseRoutine indicates routine administrative announcement
	// These are standard regulatory filings that are NOT correlated with price/volume movements
	SignalNoiseRoutine SignalNoiseRating = "ROUTINE"
)

// PriceImpactData contains stock price movement around an announcement date
type PriceImpactData struct {
	PriceBefore       float64 `json:"price_before"`        // Close price 1 trading day before
	PriceAfter        float64 `json:"price_after"`         // Close price on announcement day (or next trading day)
	ChangePercent     float64 `json:"change_percent"`      // Percentage change (immediate reaction)
	VolumeBefore      int64   `json:"volume_before"`       // Average volume 5 days before
	VolumeAfter       int64   `json:"volume_after"`        // Average volume 5 days after
	VolumeChangeRatio float64 `json:"volume_change_ratio"` // Volume ratio (after/before)
	ImpactSignal      string  `json:"impact_signal"`       // "SIGNIFICANT", "MODERATE", "MINIMAL"

	// Pre-announcement analysis (T-5 to T-1)
	PreAnnouncementDrift   float64 `json:"pre_announcement_drift,omitempty"`
	PreAnnouncementPriceT5 float64 `json:"pre_announcement_price_t5,omitempty"`
	PreAnnouncementPriceT1 float64 `json:"pre_announcement_price_t1,omitempty"`
	HasSignificantPreDrift bool    `json:"has_significant_pre_drift,omitempty"`
	PreDriftInterpretation string  `json:"pre_drift_interpretation,omitempty"`
}

// SignalNoiseResult contains the full result of signal-to-noise analysis
type SignalNoiseResult struct {
	Rating      SignalNoiseRating
	Rationale   string
	IsAnomaly   bool
	AnomalyType string // "NO_REACTION", "UNEXPECTED_REACTION", ""
}

// DeduplicationStats tracks announcements that were consolidated
type DeduplicationStats struct {
	TotalBefore     int                  `json:"total_before"`
	TotalAfter      int                  `json:"total_after"`
	DuplicatesFound int                  `json:"duplicates_found"`
	Groups          []DeduplicationGroup `json:"groups,omitempty"`
}

// DeduplicationGroup represents a set of similar announcements consolidated into one
type DeduplicationGroup struct {
	Date      time.Time `json:"date"`
	Headlines []string  `json:"headlines"`
	Count     int       `json:"count"`
}

// PriceBar represents OHLCV price data for a single day
type PriceBar struct {
	Date   time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume int64
}

// MQSScores contains Market Quality Signal scores
type MQSScores struct {
	SignalToNoiseRatio float64 `json:"signal_to_noise_ratio"`
	HighSignalCount    int     `json:"high_signal_count"`
	RoutineCount       int     `json:"routine_count"`
}

// ProcessingSummary contains aggregated statistics and results from processing
type ProcessingSummary struct {
	TotalCount           int                    `json:"total_count"`
	HighRelevanceCount   int                    `json:"high_relevance_count"`
	MediumRelevanceCount int                    `json:"medium_relevance_count"`
	LowRelevanceCount    int                    `json:"low_relevance_count"`
	NoiseCount           int                    `json:"noise_count"`
	HighSignalCount      int                    `json:"high_signal_count"`
	ModerateSignalCount  int                    `json:"moderate_signal_count"`
	LowSignalCount       int                    `json:"low_signal_count"`
	RoutineCount         int                    `json:"routine_count"`
	AnomalyCount         int                    `json:"anomaly_count"`
	Announcements        []ProcessedAnnouncement `json:"announcements,omitempty"`
	MQSScores            *MQSScores             `json:"mqs_scores,omitempty"`
}
