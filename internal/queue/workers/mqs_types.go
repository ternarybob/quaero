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

// RetentionClass classifies price sustainability based on announcement day direction
type RetentionClass string

const (
	RetentionNeutral       RetentionClass = "NEUTRAL"        // Day-of change < 1% - no significant move (0)
	RetentionPositive      RetentionClass = "POSITIVE"       // Price rises on announcement AND holds/continues (+1)
	RetentionFade          RetentionClass = "FADE"           // Price rises on announcement BUT doesn't hold (-1)
	RetentionOverReaction  RetentionClass = "OVER_REACTION"  // Price falls on announcement BUT recovers (+1)
	RetentionSustainedDrop RetentionClass = "SUSTAINED_DROP" // Price falls on announcement AND stays down (-1)
)

// ToneClass classifies announcement language tone
type ToneClass string

const (
	ToneOptimistic   ToneClass = "OPTIMISTIC"   // superlatives, promotional language
	ToneConservative ToneClass = "CONSERVATIVE" // hedged language
	ToneDataDry      ToneClass = "DATA_DRY"     // primarily numbers and facts
)

// AssetClass represents market cap classification
type AssetClass string

const (
	AssetClassLargeCap AssetClass = "LARGE_CAP" // Market Cap > $10B
	AssetClassMidCap   AssetClass = "MID_CAP"   // Market Cap $2B-$10B
	AssetClassSmallCap AssetClass = "SMALL_CAP" // Market Cap < $2B
)

// EventMateriality represents Strategic vs Routine event classification
type EventMateriality string

const (
	EventStrategic EventMateriality = "STRATEGIC" // Earnings, Guidance, M&A, Clinical/Technical milestones (weight 1.0x)
	EventRoutine   EventMateriality = "ROUTINE"   // Buy-back updates, Admin notices, Appendix 3Ys (weight 0.2x)
)

// MQSTier represents the overall management quality tier
// Per prompt_2.md: 0.70-1.00 = High-Trust Leader, 0.50-0.69 = Stable Steward, <0.50 = Strategic Risk
type MQSTier string

const (
	TierHighTrustLeader MQSTier = "HIGH_TRUST_LEADER" // 0.70-1.00: High integrity, low leakage, high efficiency
	TierStableSteward   MQSTier = "STABLE_STEWARD"    // 0.50-0.69: Generally reliable but occasional issues
	TierStrategicRisk   MQSTier = "STRATEGIC_RISK"    // <0.50: Low efficiency, frequent leakage, poor resolution
)

// MQSConfidence represents the confidence level based on data availability
type MQSConfidence string

const (
	ConfidenceHigh   MQSConfidence = "HIGH"   // ≥ 20 announcements
	ConfidenceMedium MQSConfidence = "MEDIUM" // 10-19 announcements
	ConfidenceLow    MQSConfidence = "LOW"    // < 10 announcements
)

// MQS Output Schema Types

// MQSMeta contains metadata about the analyzed entity
type MQSMeta struct {
	AssetClass AssetClass `json:"asset_class"` // LARGE_CAP, MID_CAP, SMALL_CAP
	Sector     string     `json:"sector"`      // Industry sector
	MarketCap  int64      `json:"market_cap"`  // Market capitalization in currency units
}

// MQSOutput is the root output schema for management quality analysis
type MQSOutput struct {
	Ticker       string    `json:"ticker"`
	Exchange     string    `json:"exchange"`
	CompanyName  string    `json:"company_name"`
	AnalysisDate time.Time `json:"analysis_date"`
	PeriodStart  time.Time `json:"period_start"` // Requested period start (36 months ago)
	PeriodEnd    time.Time `json:"period_end"`   // Requested period end (now)

	// Announcement counts
	TotalAnnouncements          int `json:"total_announcements"`           // All announcements in period
	PriceSensitiveAnnouncements int `json:"price_sensitive_announcements"` // Price-sensitive only

	// Metadata (asset class, sector)
	Meta MQSMeta `json:"meta"`

	// Aggregate MQS scores
	ManagementQualityScore MQSScore `json:"management_quality_score"`

	// Component summaries (5 measures, 20% each)
	LeakageSummary    LeakageSummary    `json:"leakage_summary"`    // Information Integrity
	ConvictionSummary ConvictionSummary `json:"conviction_summary"` // Institutional Conviction
	ClaritySummary    ClaritySummary    `json:"clarity_summary"`    // Clarity Index (Volatility Resolution)
	EfficiencySummary EfficiencySummary `json:"efficiency_summary"` // Communication Efficiency
	RetentionSummary  RetentionSummary  `json:"retention_summary"`  // Value Sustainability

	// Detailed events with new analysis fields
	DetailedEvents []MQSDetailedEvent `json:"detailed_events"`

	// Individual announcements (legacy, kept for compatibility)
	Announcements []MQSAnnouncement `json:"announcements"`

	// High-impact announcements with news links (past 12 months)
	HighImpactAnnouncements []HighImpactAnnouncement `json:"high_impact_announcements"`

	// Pattern analysis
	Patterns PatternAnalysis `json:"patterns"`

	// Data quality info
	DataQuality DataQualityInfo `json:"data_quality"`
}

// MQSDetailedEvent represents an event with detailed analysis per new spec
type MQSDetailedEvent struct {
	Date         string           `json:"date"`
	Headline     string           `json:"headline"`
	Type         EventMateriality `json:"type"`          // STRATEGIC or ROUTINE
	ZScore       float64          `json:"z_score"`       // Volume Z-Score
	PreDriftCAR  float64          `json:"pre_drift_car"` // Cumulative Abnormal Return (5 days prior)
	Retention10D float64          `json:"retention_10d"` // Price retention at T+10
	IsLeakage    bool             `json:"is_leakage"`    // True if |CAR| > 2σ
}

// MQSScore contains the aggregate management quality scores
// All 5 components contribute equally (20% each) to the composite score
type MQSScore struct {
	LeakageIntegrity        float64       `json:"leakage_integrity"`        // Information integrity score (CAR-based) - 20%
	InstitutionalConviction float64       `json:"institutional_conviction"` // Volume Z-Score conviction score - 20%
	ClarityIndex            float64       `json:"clarity_index"`            // Volatility resolution score - 20%
	CommunicationEfficiency float64       `json:"communication_efficiency"` // Signal-to-churn ratio - 20%
	ValueSustainability     float64       `json:"value_sustainability"`     // Price retention score - 20%
	CompositeScore          float64       `json:"composite"`                // Weighted composite score
	Tier                    MQSTier       `json:"tier"`
	Confidence              MQSConfidence `json:"confidence"`
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
	Date           string  `json:"date"`
	Headline       string  `json:"headline"`
	PreDriftPct    float64 `json:"pre_drift_pct"`
	PriceSensitive bool    `json:"price_sensitive"` // ASX price-sensitive flag
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

	// VWAP-based metrics (per prompt_3.md)
	AverageVolumeZScore      float64 `json:"average_volume_z_score"`    // Average Z-score across events
	AveragePriceVsVWAP       float64 `json:"average_price_vs_vwap"`     // Average price vs VWAP ratio
	AverageConvictionScore   float64 `json:"average_conviction_score"`  // Average combined conviction score
	AboveVWAPCount           int     `json:"above_vwap_count"`          // Events where close > VWAP (smart money)
	HighVolumeAboveVWAPRatio float64 `json:"high_vol_above_vwap_ratio"` // Ratio: (high vol + above VWAP) / total
}

// ConvictionEvent represents a high conviction announcement
type ConvictionEvent struct {
	Date            string  `json:"date"`
	Headline        string  `json:"headline"`
	PriceChange     float64 `json:"price_change_pct"`
	VolumeRatio     float64 `json:"volume_ratio"`
	VolumeZScore    float64 `json:"volume_z_score"`   // Z-score of volume
	PriceVsVWAP     float64 `json:"price_vs_vwap"`    // Price vs VWAP ratio
	ConvictionScore float64 `json:"conviction_score"` // Combined score
	Class           string  `json:"class"`
}

// ClaritySummary contains volatility resolution metrics
// Formula: Clarity Index = σ_pre / σ_post (15 days before vs 15 days after, excluding Day 0)
// Score > 1.0 indicates management successfully lowered stock's risk profile
type ClaritySummary struct {
	TotalAnalyzed         int            `json:"total_analyzed"`
	AveragePreVolatility  float64        `json:"average_pre_volatility"`  // σ_pre: 15-day volatility before
	AveragePostVolatility float64        `json:"average_post_volatility"` // σ_post: 15-day volatility after
	AverageClarityIndex   float64        `json:"average_clarity_index"`   // Average σ_pre / σ_post
	VolatilityReduced     int            `json:"volatility_reduced"`      // Count where Index > 1.0
	VolatilityIncreased   int            `json:"volatility_increased"`    // Count where Index < 1.0
	ClarityScore          float64        `json:"clarity_score"`           // Normalized score 0.0-1.0
	HighClarityEvents     []ClarityEvent `json:"high_clarity_events"`     // Best resolution events
}

// ClarityEvent represents a volatility resolution event
type ClarityEvent struct {
	Date           string  `json:"date"`
	Headline       string  `json:"headline"`
	PreVolatility  float64 `json:"pre_volatility"`  // σ_pre
	PostVolatility float64 `json:"post_volatility"` // σ_post
	ClarityIndex   float64 `json:"clarity_index"`   // σ_pre / σ_post
}

// EfficiencySummary contains communication efficiency metrics
// Formula: Efficiency = |Day 0 % ΔPrice| / Volume Z-Score
// High efficiency = high price move on moderate volume = market trusts immediately
// Low efficiency = high volume churn for modest move = lack of credibility
type EfficiencySummary struct {
	TotalAnalyzed          int               `json:"total_analyzed"`
	AverageEfficiency      float64           `json:"average_efficiency"`       // Average signal-to-churn ratio
	HighEfficiencyCount    int               `json:"high_efficiency_count"`    // Events where efficiency > 1.0
	LowEfficiencyCount     int               `json:"low_efficiency_count"`     // Events where efficiency < 0.5
	NeutralEfficiencyCount int               `json:"neutral_efficiency_count"` // Events in between
	EfficiencyScore        float64           `json:"efficiency_score"`         // Normalized score 0.0-1.0
	HighEfficiencyEvents   []EfficiencyEvent `json:"high_efficiency_events"`   // Best efficiency events
}

// EfficiencyEvent represents a communication efficiency event
type EfficiencyEvent struct {
	Date            string  `json:"date"`
	Headline        string  `json:"headline"`
	PriceChangePct  float64 `json:"price_change_pct"` // |Day 0 % ΔPrice|
	VolumeZScore    float64 `json:"volume_z_score"`   // Volume Z-Score
	EfficiencyRatio float64 `json:"efficiency_ratio"` // |ΔPrice| / Z-Score
}

// RetentionSummary contains price retention metrics (Value Sustainability)
// Score = Positive Events / (Positive Events + Negative Events)
// Range: 0.0 to 1.0 (ratio of positive to total non-neutral events)
type RetentionSummary struct {
	TotalAnalyzed      int         `json:"total_analyzed"`       // Price-sensitive events only
	NeutralCount       int         `json:"neutral_count"`        // Day-of change < 1%
	PositiveCount      int         `json:"positive_count"`       // Price rises and holds/continues
	FadeCount          int         `json:"fade_count"`           // Price rises but fades
	OverReactionCount  int         `json:"over_reaction_count"`  // Price falls but recovers
	SustainedDropCount int         `json:"sustained_drop_count"` // Price falls and stays down
	PositiveEvents     int         `json:"positive_events"`      // PositiveCount + OverReactionCount
	NegativeEvents     int         `json:"negative_events"`      // FadeCount + SustainedDropCount
	RetentionScore     float64     `json:"retention_score"`      // Positive / (Positive + Negative)
	SignificantFades   []FadeEvent `json:"significant_fades"`    // Worst fade events
}

// FadeEvent represents a significant price fade after announcement
type FadeEvent struct {
	Date           string  `json:"date"`
	Headline       string  `json:"headline"`
	DayOfChange    float64 `json:"day_of_change_pct"`
	Day10Change    float64 `json:"day_10_change_pct"`
	RetentionRatio float64 `json:"retention_ratio"`
	PriceSensitive bool    `json:"price_sensitive"` // ASX price-sensitive flag
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

	// Institutional conviction metrics (per prompt_3.md)
	VolumeZScore    float64 `json:"volume_z_score"`   // Z-score of volume vs 20-day MA
	VWAP20          float64 `json:"vwap_20"`          // 20-day Volume Weighted Average Price
	PriceVsVWAP     float64 `json:"price_vs_vwap"`    // (Close - VWAP) / VWAP as ratio
	ConvictionScore float64 `json:"conviction_score"` // Combined score: (z_score * 0.6) + (price_vs_vwap * 0.4)
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
