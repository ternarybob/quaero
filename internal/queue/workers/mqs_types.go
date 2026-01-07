// -----------------------------------------------------------------------
// MQS Types - Management Quality Score Framework
// Defines types and classifications for evaluating management quality
// based on announcement patterns, market reactions, and execution
// -----------------------------------------------------------------------

package workers

import "time"

// MQS Component Classifications

// LeakageClass classifies information integrity (pre-announcement drift)
type LeakageClass string

const (
	LeakageHigh    LeakageClass = "HIGH_LEAKAGE" // pre_drift > 3% aligned with outcome
	LeakageTight   LeakageClass = "TIGHT_SHIP"   // abs(pre_drift) < 1% AND pre_volume_ratio < 1.2
	LeakageNeutral LeakageClass = "NEUTRAL"      // Everything else
)

// ConvictionClass classifies volume/price alignment
type ConvictionClass string

const (
	ConvictionInstitutional ConvictionClass = "INSTITUTIONAL_CONVICTION" // abs(change) > 2% AND volume_ratio > 3.0
	ConvictionRetailHype    ConvictionClass = "RETAIL_HYPE"              // change > 5% AND volume_ratio < 2.0
	ConvictionLowInterest   ConvictionClass = "LOW_INTEREST"             // abs(change) < 1% AND volume_ratio < 1.0
	ConvictionMixed         ConvictionClass = "MIXED"                    // Everything else
)

// RetentionClass classifies price sustainability
type RetentionClass string

const (
	RetentionAbsorbed  RetentionClass = "ABSORBED"  // 0.7 ≤ ρ ≤ 1.3
	RetentionContinued RetentionClass = "CONTINUED" // ρ > 1.3
	RetentionSoldNews  RetentionClass = "SOLD_NEWS" // ρ < 0.5
	RetentionReversed  RetentionClass = "REVERSED"  // ρ < 0 (opposite direction)
)

// ToneClass classifies announcement language tone
type ToneClass string

const (
	ToneOptimistic   ToneClass = "OPTIMISTIC"   // superlatives, promotional language
	ToneConservative ToneClass = "CONSERVATIVE" // hedged language
	ToneDataDry      ToneClass = "DATA_DRY"     // primarily numbers and facts
)

// MQSTier represents the overall management quality tier
type MQSTier string

const (
	TierOperator        MQSTier = "TIER_1_OPERATOR"         // composite ≥ 0.75 AND leakage ≥ 0.7 AND retention ≥ 0.7
	TierHonestStruggler MQSTier = "TIER_2_HONEST_STRUGGLER" // composite ≥ 0.50 AND leakage ≥ 0.6
	TierPromoter        MQSTier = "TIER_3_PROMOTER"         // composite < 0.50 OR leakage < 0.4 OR retention < 0.4
)

// MQSConfidence represents the confidence level based on data availability
type MQSConfidence string

const (
	ConfidenceHigh   MQSConfidence = "HIGH"   // ≥ 20 announcements
	ConfidenceMedium MQSConfidence = "MEDIUM" // 10-19 announcements
	ConfidenceLow    MQSConfidence = "LOW"    // < 10 announcements
)

// MQS Output Schema Types

// MQSOutput is the root output schema for management quality analysis
type MQSOutput struct {
	Ticker       string    `json:"ticker"`
	Exchange     string    `json:"exchange"`
	CompanyName  string    `json:"company_name"`
	AnalysisDate time.Time `json:"analysis_date"`
	PeriodStart  time.Time `json:"period_start"`
	PeriodEnd    time.Time `json:"period_end"`

	// Aggregate MQS scores
	ManagementQualityScore MQSScore `json:"management_quality_score"`

	// Component summaries
	LeakageSummary    LeakageSummary    `json:"leakage_summary"`
	ConvictionSummary ConvictionSummary `json:"conviction_summary"`
	RetentionSummary  RetentionSummary  `json:"retention_summary"`

	// Individual announcements
	Announcements []MQSAnnouncement `json:"announcements"`

	// High-impact announcements with news links (past 12 months)
	HighImpactAnnouncements []HighImpactAnnouncement `json:"high_impact_announcements"`

	// Pattern analysis
	Patterns PatternAnalysis `json:"patterns"`

	// Data quality info
	DataQuality DataQualityInfo `json:"data_quality"`
}

// MQSScore contains the aggregate management quality scores
type MQSScore struct {
	LeakageScore    float64       `json:"leakage_score"`
	ConvictionScore float64       `json:"conviction_score"`
	RetentionScore  float64       `json:"retention_score"`
	CompositeScore  float64       `json:"composite_score"`
	Tier            MQSTier       `json:"tier"`
	Confidence      MQSConfidence `json:"confidence"`
}

// LeakageSummary contains information integrity metrics
type LeakageSummary struct {
	TotalAnalyzed      int               `json:"total_analyzed"`
	HighLeakageCount   int               `json:"high_leakage_count"`
	TightShipCount     int               `json:"tight_ship_count"`
	NeutralCount       int               `json:"neutral_count"`
	AveragePreDriftPct float64           `json:"average_pre_drift_pct"`
	LeakageRatio       float64           `json:"leakage_ratio"` // high_leakage / total
	WorstLeakages      []LeakageIncident `json:"worst_leakages"`
}

// LeakageIncident represents a significant pre-announcement drift event
type LeakageIncident struct {
	Date        string  `json:"date"`
	Headline    string  `json:"headline"`
	PreDriftPct float64 `json:"pre_drift_pct"`
	Direction   string  `json:"direction"` // ALIGNED or OPPOSING
}

// ConvictionSummary contains institutional conviction metrics
type ConvictionSummary struct {
	TotalAnalyzed        int               `json:"total_analyzed"`
	InstitutionalCount   int               `json:"institutional_conviction_count"`
	RetailHypeCount      int               `json:"retail_hype_count"`
	LowInterestCount     int               `json:"low_interest_count"`
	MixedCount           int               `json:"mixed_count"`
	AverageVolumeRatio   float64           `json:"average_volume_ratio"`
	InstitutionalRatio   float64           `json:"institutional_ratio"` // institutional / total
	HighConvictionEvents []ConvictionEvent `json:"high_conviction_events"`
}

// ConvictionEvent represents a high conviction announcement
type ConvictionEvent struct {
	Date        string  `json:"date"`
	Headline    string  `json:"headline"`
	PriceChange float64 `json:"price_change_pct"`
	VolumeRatio float64 `json:"volume_ratio"`
	Class       string  `json:"class"`
}

// RetentionSummary contains price retention metrics
type RetentionSummary struct {
	TotalAnalyzed         int         `json:"total_analyzed"`
	AbsorbedCount         int         `json:"absorbed_count"`
	SoldNewsCount         int         `json:"sold_news_count"`
	ContinuedCount        int         `json:"continued_count"`
	ReversedCount         int         `json:"reversed_count"`
	AverageRetentionRatio float64     `json:"average_retention_ratio"`
	RetentionRate         float64     `json:"retention_rate"` // (absorbed + continued) / total
	SignificantFades      []FadeEvent `json:"significant_fades"`
}

// FadeEvent represents a significant price fade after announcement
type FadeEvent struct {
	Date           string  `json:"date"`
	Headline       string  `json:"headline"`
	DayOfChange    float64 `json:"day_of_change_pct"`
	Day10Change    float64 `json:"day_10_change_pct"`
	RetentionRatio float64 `json:"retention_ratio"`
}

// FinancialResultType represents the type of financial result
type FinancialResultType string

const (
	ResultTypeFY FinancialResultType = "FY"       // Full Year Results
	ResultTypeHY FinancialResultType = "HY"       // Half Year Results
	ResultTypeQ1 FinancialResultType = "Q1"       // Q1 Report
	ResultTypeQ2 FinancialResultType = "Q2"       // Q2 Report
	ResultTypeQ3 FinancialResultType = "Q3"       // Q3 Report
	ResultTypeQ4 FinancialResultType = "Q4"       // Q4 Report
	ResultType4C FinancialResultType = "4C"       // Appendix 4C (Quarterly Cashflow)
	ResultType4D FinancialResultType = "4D"       // Appendix 4D (Half-Year Report)
	ResultType4E FinancialResultType = "4E"       // Appendix 4E (Preliminary Final Report)
	ResultTypeAG FinancialResultType = "GUIDANCE" // Earnings Guidance/Update
)

// FinancialResult represents an extracted financial result announcement
type FinancialResult struct {
	Date         string              `json:"date"`          // Announcement date
	Type         FinancialResultType `json:"type"`          // FY, HY, Q1-Q4, 4C, 4D, 4E, GUIDANCE
	Period       string              `json:"period"`        // e.g., "FY24", "H1 FY25", "Q3 FY24"
	Headline     string              `json:"headline"`      // Original headline
	PDFURL       string              `json:"pdf_url"`       // URL to the PDF announcement
	DayOfChange  float64             `json:"day_of_change"` // Price change on announcement day
	Day10Change  float64             `json:"day_10_change"` // Price change 10 days after
	VolumeRatio  float64             `json:"volume_ratio"`  // Volume ratio vs average
	MarketReview string              `json:"market_review"` // POSITIVE, NEGATIVE, NEUTRAL based on price/volume

	// YoY comparison fields (compared to same period in prior year)
	PriorPeriodDate     string  `json:"prior_period_date,omitempty"`    // Date of prior period result
	PriorPeriodChange   float64 `json:"prior_period_change,omitempty"`  // Day-of change in prior period
	YoYReactionDiff     float64 `json:"yoy_reaction_diff,omitempty"`    // Difference in market reaction vs prior year
	ReactionTrend       string  `json:"reaction_trend,omitempty"`       // IMPROVING, STABLE, DECLINING
	ConsecutivePositive int     `json:"consecutive_positive,omitempty"` // Streak of positive results
	ConsecutiveNegative int     `json:"consecutive_negative,omitempty"` // Streak of negative results

	// Business metrics from EODHD fundamentals data (matched by period end date)
	Revenue       int64   `json:"revenue,omitempty"`        // Total revenue in currency units
	NetIncome     int64   `json:"net_income,omitempty"`     // Net income (profit/loss)
	EBITDA        int64   `json:"ebitda,omitempty"`         // EBITDA
	GrossMargin   float64 `json:"gross_margin,omitempty"`   // Gross margin percentage
	NetMargin     float64 `json:"net_margin,omitempty"`     // Net margin percentage
	OperatingCF   int64   `json:"operating_cf,omitempty"`   // Operating cash flow
	FreeCF        int64   `json:"free_cf,omitempty"`        // Free cash flow
	RevenueYoY    float64 `json:"revenue_yoy,omitempty"`    // Revenue YoY growth percentage
	NetIncomeYoY  float64 `json:"net_income_yoy,omitempty"` // Net income YoY growth percentage
	HasFinancials bool    `json:"has_financials,omitempty"` // True if EODHD data was matched
}

// MQSAnnouncement represents a single analyzed announcement
type MQSAnnouncement struct {
	Date           string `json:"date"`
	Headline       string `json:"headline"`
	Category       string `json:"category"`
	PriceSensitive bool   `json:"price_sensitive"`

	// Lead-in metrics (5 trading days before)
	LeadIn LeadMetrics `json:"lead_in"`

	// Day-of metrics
	DayOf DayOfMetrics `json:"day_of"`

	// Lead-out metrics (10 trading days after)
	LeadOut LeadMetrics `json:"lead_out"`

	// Classifications
	LeakageClass    LeakageClass    `json:"leakage_class"`
	ConvictionClass ConvictionClass `json:"conviction_class"`
	RetentionClass  RetentionClass  `json:"retention_class"`

	// Scores (0.0 - 1.0)
	LeakageScore    float64 `json:"leakage_score"`
	ConvictionScore float64 `json:"conviction_score"`
	RetentionScore  float64 `json:"retention_score"`
}

// LeadMetrics contains price/volume metrics for lead-in or lead-out periods
type LeadMetrics struct {
	PriceChangePct float64 `json:"price_change_pct"`
	VolumeRatio    float64 `json:"volume_ratio"`
	TradingDays    int     `json:"trading_days"`
	StartPrice     float64 `json:"start_price"`
	EndPrice       float64 `json:"end_price"`
}

// DayOfMetrics contains announcement day market data
type DayOfMetrics struct {
	Open           float64 `json:"open"`
	High           float64 `json:"high"`
	Low            float64 `json:"low"`
	Close          float64 `json:"close"`
	Volume         int64   `json:"volume"`
	PriceChangePct float64 `json:"price_change_pct"`
	VolumeRatio    float64 `json:"volume_ratio"`
}

// PatternAnalysis contains detected patterns
type PatternAnalysis struct {
	PRHeavySignals   []string          `json:"pr_heavy_signals"`
	QualitySignals   []string          `json:"quality_signals"`
	DividendAnalysis *DividendAnalysis `json:"dividend_analysis,omitempty"`
	SeasonalNotes    []string          `json:"seasonal_notes"`
}

// DividendAnalysis contains dividend-related metrics
type DividendAnalysis struct {
	DividendsInPeriod          int     `json:"dividends_in_period"`
	AverageExDateDriftPct      float64 `json:"average_ex_date_drift_pct"`
	AveragePaymentDateDriftPct float64 `json:"average_payment_date_drift_pct"`
	ConsistentPayments         bool    `json:"consistent_payments"`
}

// DataQualityInfo contains data quality metadata
type DataQualityInfo struct {
	AnnouncementsCount   int       `json:"announcements_count"`
	TradingDaysCount     int       `json:"trading_days_count"`
	DataGaps             []string  `json:"data_gaps"`
	AnnouncementsDocID   string    `json:"announcements_doc_id"`
	EODDocID             string    `json:"eod_doc_id"`
	GeneratedAt          time.Time `json:"generated_at"`
	ProcessingDurationMs int64     `json:"processing_duration_ms"`
}

// HighImpactAnnouncement represents a high-impact announcement with news link
// These are announcements from the past 12 months with significant price/volume changes
// Impact is determined by: price change, volume, and price retention (fade analysis)
type HighImpactAnnouncement struct {
	Date           string  `json:"date"`
	Headline       string  `json:"headline"`
	Type           string  `json:"type"`                      // Announcement type/category
	PriceSensitive bool    `json:"price_sensitive"`           // ASX price-sensitive flag
	PriceChangePct float64 `json:"price_change_pct"`          // Day-of price change
	VolumeRatio    float64 `json:"volume_ratio"`              // Volume vs 20-day average
	Day10ChangePct float64 `json:"day10_change_pct"`          // Price change after 10 trading days
	RetentionRatio float64 `json:"retention_ratio"`           // Day10/DayOf ratio - >0.5 means price held
	NewsLink       string  `json:"news_link,omitempty"`       // EODHD news article link
	NewsTitle      string  `json:"news_title,omitempty"`      // EODHD news article title
	NewsSource     string  `json:"news_source,omitempty"`     // News source (e.g., "Reuters", "ASX")
	Sentiment      string  `json:"sentiment,omitempty"`       // Sentiment from EODHD (positive/negative/neutral)
	ImpactRating   string  `json:"impact_rating"`             // HIGH_SIGNAL only (minimal fade), MODERATE excluded
	PDFURL         string  `json:"pdf_url,omitempty"`         // ASX announcement PDF URL
	DocumentKey    string  `json:"document_key,omitempty"`    // Internal document reference
	PDFStorageKey  string  `json:"pdf_storage_key,omitempty"` // Key for retrieving PDF from BadgerDB KV storage
	PDFDownloaded  bool    `json:"pdf_downloaded"`            // Whether PDF was successfully downloaded
	PDFSizeBytes   int64   `json:"pdf_size_bytes,omitempty"`  // Size of downloaded PDF in bytes
}

// EODHDNewsItem represents a news item from EODHD API
// Used internally for matching announcements to news articles
type EODHDNewsItem struct {
	Date      time.Time
	Title     string
	Content   string
	Link      string
	Symbols   []string
	Tags      []string
	Sentiment string // positive, negative, neutral
}
