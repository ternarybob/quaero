// Package rating provides pure calculation functions for stock investability ratings.
// All functions are stateless and perform no I/O.
package rating

import "time"

// AnnouncementType categorizes announcement types
type AnnouncementType string

const (
	TypeTradingHalt  AnnouncementType = "trading_halt"
	TypeCapitalRaise AnnouncementType = "capital_raise"
	TypeQuarterly    AnnouncementType = "quarterly"
	TypeAnnualReport AnnouncementType = "annual_report"
	TypeDrilling     AnnouncementType = "drilling"
	TypeAcquisition  AnnouncementType = "acquisition"
	TypeContract     AnnouncementType = "contract"
	TypeOther        AnnouncementType = "other"
)

// Fundamentals represents stock fundamental data
type Fundamentals struct {
	Ticker                   string
	CompanyName              string
	Sector                   string
	MarketCap                float64
	SharesOutstandingCurrent int64
	SharesOutstanding3YAgo   *int64
	CashBalance              float64
	QuarterlyCashBurn        float64
	RevenueTTM               float64
	IsProfitable             bool
	HasProducingAsset        bool
}

// Announcement represents a company announcement
type Announcement struct {
	Date             time.Time
	Headline         string
	Type             AnnouncementType
	IsPriceSensitive bool
}

// PriceBar represents OHLCV data
type PriceBar struct {
	Date   time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume int64
}

// RatingLabel represents the investability label
type RatingLabel string

const (
	LabelSpeculative    RatingLabel = "SPECULATIVE"
	LabelLowAlpha       RatingLabel = "LOW_ALPHA"
	LabelWatchlist      RatingLabel = "WATCHLIST"
	LabelInvestable     RatingLabel = "INVESTABLE"
	LabelHighConviction RatingLabel = "HIGH_CONVICTION"
)

// BFSComponents holds BFS calculation details
type BFSComponents struct {
	HasRevenue        bool    `json:"has_revenue"`
	RevenueAmount     float64 `json:"revenue_amount"`
	CashRunwayMonths  float64 `json:"cash_runway_months"`
	HasProducingAsset bool    `json:"has_producing_asset"`
	IsProfitable      bool    `json:"is_profitable"`
}

// BFSResult is the output of CalculateBFS
type BFSResult struct {
	Score          int           `json:"score"` // 0, 1, or 2
	IndicatorCount int           `json:"indicator_count"`
	Components     BFSComponents `json:"components"`
	Reasoning      string        `json:"reasoning"`
}

// CDSComponents holds CDS calculation details
type CDSComponents struct {
	SharesCAGR       float64 `json:"shares_cagr"`
	TradingHaltsPA   float64 `json:"trading_halts_pa"`
	CapitalRaisesPA  float64 `json:"capital_raises_pa"`
	AnalysisPeriodMo int     `json:"analysis_period_mo"`
}

// CDSResult is the output of CalculateCDS
type CDSResult struct {
	Score      int           `json:"score"` // 0, 1, or 2
	Components CDSComponents `json:"components"`
	Reasoning  string        `json:"reasoning"`
}

// NFRComponents holds NFR calculation details
type NFRComponents struct {
	TotalAnnouncements     int     `json:"total_announcements"`
	FactAnnouncements      int     `json:"fact_announcements"`
	NarrativeAnnouncements int     `json:"narrative_announcements"`
	FactRatio              float64 `json:"fact_ratio"`
}

// NFRResult is the output of CalculateNFR
type NFRResult struct {
	Score      float64       `json:"score"` // 0.0 to 1.0
	Components NFRComponents `json:"components"`
	Reasoning  string        `json:"reasoning"`
}

// PPSEventDetail holds price progression event info
type PPSEventDetail struct {
	Date         time.Time `json:"date"`
	Headline     string    `json:"headline"`
	PriceBefore  float64   `json:"price_before"`
	PriceAfter   float64   `json:"price_after"`
	RetentionPct float64   `json:"retention_pct"`
}

// PPSResult is the output of CalculatePPS
type PPSResult struct {
	Score        float64          `json:"score"` // 0.0 to 1.0
	EventDetails []PPSEventDetail `json:"event_details"`
	Reasoning    string           `json:"reasoning"`
}

// VRSComponents holds VRS calculation details
type VRSComponents struct {
	RegimeCount       int     `json:"regime_count"`
	StableRegimesPct  float64 `json:"stable_regimes_pct"`
	VolatilityPattern string  `json:"volatility_pattern"`
}

// VRSResult is the output of CalculateVRS
type VRSResult struct {
	Score      float64       `json:"score"` // 0.0 to 1.0
	Components VRSComponents `json:"components"`
	Reasoning  string        `json:"reasoning"`
}

// OBResult is the output of CalculateOB
type OBResult struct {
	Score          float64 `json:"score"` // 0.0, 0.5, or 1.0
	CatalystFound  bool    `json:"catalyst_found"`
	TimeframeFound bool    `json:"timeframe_found"`
	Reasoning      string  `json:"reasoning"`
}

// AllScores holds all component scores
type AllScores struct {
	BFS BFSResult `json:"bfs"`
	CDS CDSResult `json:"cds"`
	NFR NFRResult `json:"nfr"`
	PPS PPSResult `json:"pps"`
	VRS VRSResult `json:"vrs"`
	OB  OBResult  `json:"ob"`
}

// RatingResult is the output of CalculateRating
type RatingResult struct {
	Label         RatingLabel `json:"label"`
	Investability *float64    `json:"investability,omitempty"` // 0-100, nil if gate failed
	GatePassed    bool        `json:"gate_passed"`
	Scores        AllScores   `json:"scores"`
	Reasoning     string      `json:"reasoning"`
}
