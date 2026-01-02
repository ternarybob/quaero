// Package signals provides types and computations for ASX stock signal analysis.
// This implements the signal computation algorithms from the ASX Portfolio Intelligence System.
package signals

import (
	"encoding/gob"
	"time"
)

func init() {
	// Register types for gob encoding (required for BadgerHold storage of interface{} fields)
	gob.Register(TickerSignals{})
	gob.Register(PriceSignals{})
	gob.Register(PBASSignal{})
	gob.Register(VLISignal{})
	gob.Register(RegimeSignal{})
	gob.Register(RSSignal{})
	gob.Register(CookedSignal{})
	gob.Register(QualitySignal{})
	gob.Register(AnnouncementSignals{})
	gob.Register(JustifiedReturnSignal{})
}

// TickerSignals contains all computed signals for a ticker
type TickerSignals struct {
	Ticker           string    `json:"ticker"`
	ComputeTimestamp time.Time `json:"compute_timestamp"`

	// Core price data (carried forward)
	Price PriceSignals `json:"price"`

	// Computed signals
	PBAS   PBASSignal   `json:"pbas"`
	VLI    VLISignal    `json:"vli"`
	Regime RegimeSignal `json:"regime"`
	RS     RSSignal     `json:"relative_strength"`
	Cooked CookedSignal `json:"cooked"`

	// Quality summary
	Quality QualitySignal `json:"quality"`

	// Announcement summary
	Announcements AnnouncementSignals `json:"announcements"`

	// Justified returns
	JustifiedReturn JustifiedReturnSignal `json:"justified_return"`

	// Risk flags
	RiskFlags            []string `json:"risk_flags"`
	RiskFlagsDescription string   `json:"risk_flags_description"` // What risk flags represent
}

// PriceSignals contains price-related signals
type PriceSignals struct {
	Current              float64 `json:"current"`
	Change1DPct          float64 `json:"change_1d_pct"`
	Return12WPct         float64 `json:"return_12w_pct"`
	Return52WPct         float64 `json:"return_52w_pct"`
	VsEMA20              string  `json:"vs_ema20"` // above, below, at
	VsEMA50              string  `json:"vs_ema50"`
	VsEMA200             string  `json:"vs_ema200"`
	DistanceTo52WHighPct float64 `json:"distance_to_52w_high_pct"`
	DistanceTo52WLowPct  float64 `json:"distance_to_52w_low_pct"`
}

// PBASSignal represents the Price-Business Alignment Score
type PBASSignal struct {
	Score            float64 `json:"score"`             // 0.0 - 1.0
	BusinessMomentum float64 `json:"business_momentum"` // Composite business momentum
	PriceMomentum    float64 `json:"price_momentum"`    // 12-month price return
	Divergence       float64 `json:"divergence"`        // BM - PM
	Interpretation   string  `json:"interpretation"`    // underpriced, neutral, overpriced
	Description      string  `json:"description"`       // What this signal measures
	Comment          string  `json:"comment"`           // AI-generated interpretation of the score
}

// VLISignal represents the Volume Lead Indicator
type VLISignal struct {
	Score       float64 `json:"score"`         // -1.0 to 1.0
	Label       string  `json:"label"`         // accumulating, distributing, neutral
	VolZScore   float64 `json:"vol_zscore"`    // Volume z-score
	PriceVsVWAP float64 `json:"price_vs_vwap"` // Price relative to 20-day VWAP
	Description string  `json:"description"`   // What this signal measures
	Comment     string  `json:"comment"`       // AI-generated interpretation of the score
}

// RegimeSignal represents the price action regime classification
type RegimeSignal struct {
	Classification string  `json:"classification"` // breakout, trend_up, trend_down, accumulation, distribution, range, decay, undefined
	Confidence     float64 `json:"confidence"`     // 0.0 - 1.0
	TrendBias      string  `json:"trend_bias"`     // bullish, bearish, neutral
	EMAStack       string  `json:"ema_stack"`      // bullish, bearish, mixed
	Description    string  `json:"description"`    // What this signal measures
	Comment        string  `json:"comment"`        // AI-generated interpretation of the score
}

// RegimeType categorizes price action regimes
type RegimeType string

const (
	RegimeBreakout     RegimeType = "breakout"
	RegimeTrendUp      RegimeType = "trend_up"
	RegimeTrendDown    RegimeType = "trend_down"
	RegimeAccumulation RegimeType = "accumulation"
	RegimeDistribution RegimeType = "distribution"
	RegimeRange        RegimeType = "range"
	RegimeDecay        RegimeType = "decay"
	RegimeUndefined    RegimeType = "undefined"
)

// CookedSignal indicates if a stock is overvalued/decoupled from fundamentals
type CookedSignal struct {
	IsCooked    bool     `json:"is_cooked"`   // true if score >= 2
	Score       int      `json:"score"`       // 0-5, number of triggers
	Reasons     []string `json:"reasons"`     // Which conditions triggered
	Description string   `json:"description"` // What this signal measures
	Comment     string   `json:"comment"`     // AI-generated interpretation of the score
}

// RSSignal represents relative strength vs benchmark
type RSSignal struct {
	VsXJO3M          float64 `json:"vs_xjo_3m"`          // RS ratio vs XJO over 3 months
	VsXJO6M          float64 `json:"vs_xjo_6m"`          // RS ratio vs XJO over 6 months
	RSRankPercentile int     `json:"rs_rank_percentile"` // Estimated percentile rank
	Description      string  `json:"description"`        // What this signal measures
	Comment          string  `json:"comment"`            // AI-generated interpretation of the score
}

// QualitySignal represents business quality assessment
type QualitySignal struct {
	Overall          string `json:"overall"`            // good, fair, poor
	CashConversion   string `json:"cash_conversion"`    // good, fair, poor
	BalanceSheetRisk string `json:"balance_sheet_risk"` // low, medium, high
	MarginTrend      string `json:"margin_trend"`       // improving, stable, declining
	Description      string `json:"description"`        // What this signal measures
	Comment          string `json:"comment"`            // AI-generated interpretation of the score
}

// AnnouncementSignals summarizes announcement data
type AnnouncementSignals struct {
	HighSignalCount30D    int     `json:"high_signal_count_30d"`
	MostRecentMaterial    string  `json:"most_recent_material"`
	MostRecentMaterialSNI float64 `json:"most_recent_material_sni"`
	Sentiment30D          string  `json:"sentiment_30d"` // positive, negative, neutral
	PRHeavyIssuer         bool    `json:"pr_heavy_issuer"`
}

// JustifiedReturnSignal represents justified return analysis
type JustifiedReturnSignal struct {
	Expected12MPct float64 `json:"expected_12m_pct"` // Expected return based on business momentum
	Actual12MPct   float64 `json:"actual_12m_pct"`   // Actual 12-month return
	DivergencePct  float64 `json:"divergence_pct"`   // Actual - Expected
	Interpretation string  `json:"interpretation"`   // aligned, ahead, behind, slightly_ahead, slightly_behind, price_ahead, price_behind
	Description    string  `json:"description"`      // What this signal measures
	Comment        string  `json:"comment"`          // AI-generated interpretation of the score
}

// ValidationResult represents the outcome of validating a model
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}
