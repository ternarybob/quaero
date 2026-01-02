# ASX Portfolio Intelligence System — Data Models

## Document Purpose
This document defines all data structures used throughout the system. Implementations should use these as the canonical reference for type definitions.

---

## Core Principles

1. **Immutability**: Data models are immutable after creation
2. **Validation**: All models validate on construction
3. **Serialization**: All models serialize to JSON/YAML
4. **Documentation**: All fields have clear descriptions

---

## Portfolio Models

### Holding

```go
// Holding represents a single portfolio position
type Holding struct {
    // Identification
    Ticker      string `json:"ticker" validate:"required,asx_ticker"`
    Name        string `json:"name" validate:"required"`
    
    // Classification
    Sector      string `json:"sector" validate:"required"`
    Industry    string `json:"industry,omitempty"`
    HoldingType string `json:"holding_type" validate:"oneof=smsf trader"`
    
    // Position
    Units       float64 `json:"units" validate:"required,gt=0"`
    AvgPrice    float64 `json:"avg_price" validate:"required,gt=0"`
    
    // Targets
    TargetWeightPct float64 `json:"target_weight_pct" validate:"gte=0,lte=100"`
    
    // Computed (set after load)
    CostBasis   float64 `json:"cost_basis"`
}

// Validate performs validation on the holding
func (h *Holding) Validate() error {
    // Custom validation logic
    if h.Units <= 0 {
        return errors.New("units must be positive")
    }
    if h.AvgPrice <= 0 {
        return errors.New("avg_price must be positive")
    }
    return nil
}

// ComputeCostBasis calculates the total cost basis
func (h *Holding) ComputeCostBasis() {
    h.CostBasis = h.Units * h.AvgPrice
}
```

### Portfolio State

```go
// PortfolioState represents the complete portfolio snapshot
type PortfolioState struct {
    // Metadata
    Meta PortfolioMeta `json:"meta"`
    
    // Holdings
    Holdings []Holding `json:"holdings" validate:"required,min=1,dive"`
    
    // Aggregations (computed)
    SectorAllocation    map[string]float64 `json:"sector_allocation"`
    HoldingTypeSplit    map[string]float64 `json:"holding_type_split"`
    TotalCostBasis      float64            `json:"total_cost_basis"`
}

type PortfolioMeta struct {
    Name              string    `json:"name"`
    AsOf              time.Time `json:"as_of"`
    TotalHoldings     int       `json:"total_holdings"`
    BenchmarkPrimary  string    `json:"benchmark_primary"`
    BenchmarkSecondary string   `json:"benchmark_secondary,omitempty"`
    BaseCurrency      string    `json:"base_currency"`
}
```

---

## Price & Market Data Models

### TickerRaw

```go
// TickerRaw contains compressed market data for a single ticker
// This is NOT raw OHLCV - it's pre-computed derived values
type TickerRaw struct {
    Ticker          string    `json:"ticker"`
    FetchTimestamp  time.Time `json:"fetch_timestamp"`
    
    Price           PriceData       `json:"price"`
    Volume          VolumeData      `json:"volume"`
    Volatility      VolatilityData  `json:"volatility"`
    RelativeStrength RSData         `json:"relative_strength"`
    Fundamentals    FundamentalsData `json:"fundamentals,omitempty"`
    
    // Data quality flags
    HasFundamentals bool   `json:"has_fundamentals"`
    DataQuality     string `json:"data_quality"` // complete, partial, stale
    Errors          []string `json:"errors,omitempty"`
}

type PriceData struct {
    // Current
    Current     float64 `json:"current"`
    PrevClose   float64 `json:"prev_close"`
    Open        float64 `json:"open"`
    High        float64 `json:"high"`
    Low         float64 `json:"low"`
    Change1DPct float64 `json:"change_1d_pct"`
    
    // Key Levels (pre-computed, not full history)
    High52W float64 `json:"high_52w"`
    Low52W  float64 `json:"low_52w"`
    EMA20   float64 `json:"ema_20"`
    EMA50   float64 `json:"ema_50"`
    EMA200  float64 `json:"ema_200"`
    VWAP20  float64 `json:"vwap_20"`
    
    // Returns (pre-computed)
    Return1WPct  float64 `json:"return_1w_pct"`
    Return4WPct  float64 `json:"return_4w_pct"`
    Return12WPct float64 `json:"return_12w_pct"`
    Return26WPct float64 `json:"return_26w_pct"`
    Return52WPct float64 `json:"return_52w_pct"`
}

type VolumeData struct {
    Current     int64   `json:"current"`
    SMA20       float64 `json:"sma_20"`
    SMA50       float64 `json:"sma_50"`
    ZScore20    float64 `json:"zscore_20"`
    Trend5Dvs20D string `json:"trend_5d_vs_20d"` // rising, falling, flat
}

type VolatilityData struct {
    ATR14         float64 `json:"atr_14"`
    ATR21         float64 `json:"atr_21"`
    ATRPctOfPrice float64 `json:"atr_pct_of_price"`
}

type RSData struct {
    VsXJO3M      float64 `json:"vs_xjo_3m"`
    VsXJO6M      float64 `json:"vs_xjo_6m"`
    VsSector3M   float64 `json:"vs_sector_3m,omitempty"`
}
```

### Fundamentals Data

```go
type FundamentalsData struct {
    // Valuation
    MarketCapM     float64 `json:"market_cap_m"`
    PERatio        float64 `json:"pe_ratio,omitempty"`
    PEVsSectorMedian float64 `json:"pe_vs_sector_median,omitempty"`
    
    // Revenue
    RevenueTTMM    float64 `json:"revenue_ttm_m"`
    RevenueYoYPct  float64 `json:"revenue_yoy_pct"`
    
    // Margins
    EBITDAMarginPct     float64 `json:"ebitda_margin_pct"`
    EBITDAMarginDeltaYoY float64 `json:"ebitda_margin_delta_yoy"`
    GrossMarginPct      float64 `json:"gross_margin_pct,omitempty"`
    
    // Cash Flow
    OperatingCFTTMM float64 `json:"operating_cf_ttm_m"`
    OCFToEBITDA     float64 `json:"ocf_to_ebitda"`
    FCFTTMM         float64 `json:"fcf_ttm_m"`
    FCFMarginPct    float64 `json:"fcf_margin_pct"`
    
    // Balance Sheet
    NetDebtM        float64 `json:"net_debt_m"`
    NetDebtToEBITDA float64 `json:"net_debt_to_ebitda"`
    CurrentRatio    float64 `json:"current_ratio,omitempty"`
    
    // Returns
    ROICPct float64 `json:"roic_pct,omitempty"`
    ROEPct  float64 `json:"roe_pct"`
    ROAPct  float64 `json:"roa_pct,omitempty"`
    
    // Capital Structure
    SharesOutstandingM float64 `json:"shares_outstanding_m"`
    Dilution12MPct     float64 `json:"dilution_12m_pct"`
    
    // Quality Flags (derived)
    CashConversionQuality string `json:"cash_conversion_quality"` // good, fair, poor
    BalanceSheetRisk      string `json:"balance_sheet_risk"`      // low, medium, high
}
```

---

## Announcement Models

### TickerAnnouncements

```go
// TickerAnnouncements contains processed announcements for a ticker
type TickerAnnouncements struct {
    Ticker          string    `json:"ticker"`
    FetchTimestamp  time.Time `json:"fetch_timestamp"`
    
    AnnouncementCount30D int            `json:"announcement_count_30d"`
    PRHeavyIssuer        bool           `json:"pr_heavy_issuer"`
    Announcements        []Announcement `json:"announcements"`
    
    // Summary stats
    Summary AnnouncementSummary `json:"summary"`
}

type Announcement struct {
    Date     string `json:"date"` // YYYY-MM-DD
    Headline string `json:"headline"`
    
    // Classification
    Type           string  `json:"type"` // See AnnouncementType enum
    SubstanceScore float64 `json:"substance_score"`
    PREntropyScore float64 `json:"pr_entropy_score"`
    
    // Reaction validation
    Reaction      ReactionData `json:"reaction"`
    ReactionScore float64      `json:"reaction_score"`
    
    // Final score
    SNI float64 `json:"sni"` // Signal-to-Noise Index
    
    // AI-generated summary
    Summary     string `json:"summary"`
    SignalClass string `json:"signal_class"` // HIGH_POSITIVE, HIGH_NEGATIVE, NOISE
}

type ReactionData struct {
    PriceT1Pct    float64 `json:"price_t1_pct"`
    PriceT3Pct    float64 `json:"price_t3_pct"`
    VolumeT1Ratio float64 `json:"volume_t1_ratio"`
    Held50Pct     bool    `json:"held_50pct"`
}

type AnnouncementSummary struct {
    HighSignalCount    int    `json:"high_signal_count"`
    NoiseCount         int    `json:"noise_count"`
    AvgSNI30D          float64 `json:"avg_sni_30d"`
    MostRecentMaterial string `json:"most_recent_material"`
    Sentiment30D       string `json:"sentiment_30d"` // positive, negative, neutral
}
```

### Announcement Types (Enum)

```go
// AnnouncementType categorizes ASX announcements
type AnnouncementType string

const (
    AnnTypeQuantifiedContract AnnouncementType = "quantified_contract"
    AnnTypeGuidanceChange     AnnouncementType = "guidance_change"
    AnnTypeCapitalRaise       AnnouncementType = "capital_raise"
    AnnTypeResults            AnnouncementType = "results"
    AnnTypeMaterialChange     AnnouncementType = "material_change"
    AnnTypeCorporateAction    AnnouncementType = "corporate_action"
    AnnTypeStrategicReview    AnnouncementType = "strategic_review"
    AnnTypeAppendix4C         AnnouncementType = "appendix_4c"
    AnnTypeAppendix4E         AnnouncementType = "appendix_4e"
    AnnTypeDirectorInterest   AnnouncementType = "director_interest"
    AnnTypeSubstantialHolder  AnnouncementType = "substantial_holder"
    AnnTypeTradingHalt        AnnouncementType = "trading_halt"
    AnnTypePRUpdate           AnnouncementType = "pr_update"
    AnnTypeAdministrative     AnnouncementType = "administrative"
)
```

---

## Signal Models

### TickerSignals

```go
// TickerSignals contains all computed signals for a ticker
type TickerSignals struct {
    Ticker           string    `json:"ticker"`
    ComputeTimestamp time.Time `json:"compute_timestamp"`
    
    // Core price data (carried forward)
    Price PriceSignals `json:"price"`
    
    // Computed signals
    PBAS    PBASSignal    `json:"pbas"`
    VLI     VLISignal     `json:"vli"`
    Regime  RegimeSignal  `json:"regime"`
    RS      RSSignal      `json:"relative_strength"`
    Cooked  CookedSignal  `json:"cooked"`
    
    // Quality summary
    Quality QualitySignal `json:"quality"`
    
    // Announcement summary
    Announcements AnnouncementSignals `json:"announcements"`
    
    // Justified returns
    JustifiedReturn JustifiedReturnSignal `json:"justified_return"`
    
    // Risk flags
    RiskFlags []string `json:"risk_flags"`
}

type PriceSignals struct {
    Current              float64 `json:"current"`
    Change1DPct          float64 `json:"change_1d_pct"`
    Return12WPct         float64 `json:"return_12w_pct"`
    Return52WPct         float64 `json:"return_52w_pct"`
    VsEMA20              string  `json:"vs_ema20"`  // above, below, at
    VsEMA50              string  `json:"vs_ema50"`
    VsEMA200             string  `json:"vs_ema200"`
    DistanceTo52WHighPct float64 `json:"distance_to_52w_high_pct"`
    DistanceTo52WLowPct  float64 `json:"distance_to_52w_low_pct"`
}
```

### PBAS Signal

```go
// PBASSignal represents the Price-Business Alignment Score
type PBASSignal struct {
    Score            float64 `json:"score"`            // 0.0 - 1.0
    BusinessMomentum float64 `json:"business_momentum"`
    PriceMomentum    float64 `json:"price_momentum"`
    Divergence       float64 `json:"divergence"`
    Interpretation   string  `json:"interpretation"`   // underpriced, neutral, overpriced
}
```

### VLI Signal

```go
// VLISignal represents the Volume Lead Indicator
type VLISignal struct {
    Score       float64 `json:"score"`        // -1.0 to 1.0
    Label       string  `json:"label"`        // accumulating, distributing, neutral
    VolZScore   float64 `json:"vol_zscore"`
    PriceVsVWAP float64 `json:"price_vs_vwap"`
}
```

### Regime Signal

```go
// RegimeSignal represents the price action regime classification
type RegimeSignal struct {
    Classification string  `json:"classification"` // See RegimeType enum
    Confidence     float64 `json:"confidence"`
    TrendBias      string  `json:"trend_bias"`     // bullish, bearish
    EMAStack       string  `json:"ema_stack"`      // bullish, bearish, mixed
}

// RegimeType categorizes price action regimes
type RegimeType string

const (
    RegimeBreakout      RegimeType = "breakout"
    RegimeTrendUp       RegimeType = "trend_up"
    RegimeTrendDown     RegimeType = "trend_down"
    RegimeAccumulation  RegimeType = "accumulation"
    RegimeDistribution  RegimeType = "distribution"
    RegimeRange         RegimeType = "range"
    RegimeDecay         RegimeType = "decay"
    RegimeUndefined     RegimeType = "undefined"
)
```

### Cooked Signal

```go
// CookedSignal indicates if a stock is overvalued/decoupled from fundamentals
type CookedSignal struct {
    IsCooked bool     `json:"is_cooked"`
    Score    int      `json:"score"`    // 0-5, cooked if >= 2
    Reasons  []string `json:"reasons"`  // Which conditions triggered
}
```

### Other Signals

```go
type RSSignal struct {
    VsXJO3M        float64 `json:"vs_xjo_3m"`
    VsXJO6M        float64 `json:"vs_xjo_6m"`
    RSRankPercentile int   `json:"rs_rank_percentile"`
}

type QualitySignal struct {
    Overall          string `json:"overall"`           // good, fair, poor
    CashConversion   string `json:"cash_conversion"`
    BalanceSheetRisk string `json:"balance_sheet_risk"`
    MarginTrend      string `json:"margin_trend"`      // improving, stable, declining
}

type AnnouncementSignals struct {
    HighSignalCount30D     int     `json:"high_signal_count_30d"`
    MostRecentMaterial     string  `json:"most_recent_material"`
    MostRecentMaterialSNI  float64 `json:"most_recent_material_sni"`
    Sentiment30D           string  `json:"sentiment_30d"`
    PRHeavyIssuer          bool    `json:"pr_heavy_issuer"`
}

type JustifiedReturnSignal struct {
    Expected12MPct   float64 `json:"expected_12m_pct"`
    Actual12MPct     float64 `json:"actual_12m_pct"`
    DivergencePct    float64 `json:"divergence_pct"`
    Interpretation   string  `json:"interpretation"` // aligned, ahead, behind
}
```

---

## Assessment Models

### TickerAssessment

```go
// TickerAssessment contains the AI-generated assessment for a ticker
type TickerAssessment struct {
    Ticker      string `json:"ticker"`
    HoldingType string `json:"holding_type"` // smsf, trader
    
    Decision  AssessmentDecision `json:"decision"`
    Reasoning AssessmentReasoning `json:"reasoning"`
    EntryExit EntryExitParams    `json:"entry_exit"`
    
    RiskFlags    []string `json:"risk_flags"`
    ThesisStatus string   `json:"thesis_status"` // intact, weakening, strengthening, broken
    
    JustifiedGain JustifiedGainAssessment `json:"justified_gain_assessment"`
    
    // Validation
    ValidationPassed bool     `json:"validation_passed"`
    ValidationErrors []string `json:"validation_errors,omitempty"`
}

type AssessmentDecision struct {
    Action     string `json:"action"`     // accumulate, hold, reduce, exit, buy, add, trim, watch
    Confidence string `json:"confidence"` // high, medium, low
    Urgency    string `json:"urgency"`    // immediate, this_week, monitor
}

type AssessmentReasoning struct {
    Primary  string   `json:"primary"`  // 1-2 sentence main rationale
    Evidence []string `json:"evidence"` // 3+ specific data points
}

type EntryExitParams struct {
    // For entries
    Setup       string `json:"setup,omitempty"`
    EntryZone   string `json:"entry_zone,omitempty"`
    
    // For all
    StopLoss    string  `json:"stop_loss"`
    StopLossPct float64 `json:"stop_loss_pct"`
    Target1     string  `json:"target_1,omitempty"`
    Invalidation string `json:"invalidation"`
}

type JustifiedGainAssessment struct {
    Justified12MPct float64 `json:"justified_12m_pct"`
    CurrentGainPct  float64 `json:"current_gain_pct"`
    Verdict         string  `json:"verdict"` // aligned, ahead, behind
}
```

---

## Portfolio Rollup Models

### PortfolioRollup

```go
// PortfolioRollup aggregates portfolio-level metrics
type PortfolioRollup struct {
    Meta        RollupMeta         `json:"meta"`
    Performance PerformanceMetrics `json:"performance"`
    Allocation  AllocationMetrics  `json:"allocation"`
    
    ConcentrationAlerts []string           `json:"concentration_alerts"`
    CorrelationClusters []CorrelationCluster `json:"correlation_clusters"`
    
    ActionSummary ActionSummary `json:"action_summary"`
    CashPosition  CashPosition  `json:"cash_position"`
    
    RebalanceSuggestions []RebalanceSuggestion `json:"rebalance_suggestions"`
}

type RollupMeta struct {
    AsOf              time.Time `json:"as_of"`
    HoldingsAssessed  int       `json:"holdings_assessed"`
    AssessmentsValid  int       `json:"assessments_valid"`
}

type PerformanceMetrics struct {
    TotalValue    float64 `json:"total_value"`
    TotalCost     float64 `json:"total_cost"`
    TotalPnL      float64 `json:"total_pnl"`
    TotalPnLPct   float64 `json:"total_pnl_pct"`
    
    Return1MPct   float64 `json:"return_1m_pct,omitempty"`
    Return3MPct   float64 `json:"return_3m_pct,omitempty"`
    ReturnYTDPct  float64 `json:"return_ytd_pct,omitempty"`
    
    VsXJOYTDPct   float64 `json:"vs_xjo_ytd_pct"`
    VsXSOYTDPct   float64 `json:"vs_xso_ytd_pct,omitempty"`
}

type AllocationMetrics struct {
    BySector      map[string]float64 `json:"by_sector"`
    ByHoldingType map[string]float64 `json:"by_holding_type"`
    ByRegime      map[string]float64 `json:"by_regime"`
}

type CorrelationCluster struct {
    Tickers     []string `json:"tickers"`
    Correlation float64  `json:"correlation"`
    Note        string   `json:"note"`
}

type ActionSummary struct {
    ImmediateActions int            `json:"immediate_actions"`
    WatchClosely     int            `json:"watch_closely"`
    HoldNoAction     int            `json:"hold_no_action"`
    Actions          []ActionItem   `json:"actions"`
}

type ActionItem struct {
    Ticker  string `json:"ticker"`
    Action  string `json:"action"`
    Urgency string `json:"urgency"`
    Reason  string `json:"reason"`
}

type CashPosition struct {
    CurrentPct     float64 `json:"current_pct"`
    TargetPct      float64 `json:"target_pct"`
    Recommendation string  `json:"recommendation"`
}

type RebalanceSuggestion struct {
    From   string `json:"from"`
    To     string `json:"to"`
    Reason string `json:"reason"`
}
```

---

## Report Models

### DailyReport

```go
// DailyReport is the final assembled report
type DailyReport struct {
    // Metadata
    GeneratedAt    time.Time `json:"generated_at"`
    ReportDate     string    `json:"report_date"`
    ProcessingTime float64   `json:"processing_time_seconds"`
    
    // Content sections
    ExecutiveSummary ExecutiveSummary    `json:"executive_summary"`
    MarketContext    MarketContext       `json:"market_context"`
    ActionsRequired  []ActionSection     `json:"actions_required"`
    HoldingsSummary  []HoldingBlockSummary `json:"holdings_summary"`
    PortfolioHealth  PortfolioHealth     `json:"portfolio_health"`
    UpcomingCatalysts []Catalyst         `json:"upcoming_catalysts"`
    ScreeningResults  *ScreeningResults  `json:"screening_results,omitempty"`
    Appendix         Appendix            `json:"appendix"`
    
    // Raw content for email
    MarkdownContent string `json:"markdown_content"`
    HTMLContent     string `json:"html_content,omitempty"`
}

type ExecutiveSummary struct {
    PortfolioValue   float64 `json:"portfolio_value"`
    DailyChange      float64 `json:"daily_change"`
    DailyChangePct   float64 `json:"daily_change_pct"`
    YTDReturn        float64 `json:"ytd_return"`
    VsBenchmark      float64 `json:"vs_benchmark"`
    Positions        int     `json:"positions"`
    MarketRegime     string  `json:"market_regime"`
    PriorityItems    []string `json:"priority_items"`
}

type MarketContext struct {
    IndexLevel       float64 `json:"index_level"`
    IndexVs50EMA     string  `json:"index_vs_50_ema"`
    IndexVs200EMA    string  `json:"index_vs_200_ema"`
    MarketRegime     string  `json:"market_regime"`
    Breadth          float64 `json:"breadth_pct_above_200ema"`
    KeyEvents        []string `json:"key_events"`
}

type ActionSection struct {
    Urgency    string           `json:"urgency"` // urgent, watch
    Holdings   []HoldingAction  `json:"holdings"`
}

type HoldingAction struct {
    Ticker          string   `json:"ticker"`
    Name            string   `json:"name"`
    Action          string   `json:"action"`
    Position        string   `json:"position"`
    CurrentPrice    float64  `json:"current_price"`
    PnLPct          float64  `json:"pnl_pct"`
    Weight          float64  `json:"weight"`
    TechnicalDetail string   `json:"technical_detail"`
    FundamentalDetail string `json:"fundamental_detail"`
    AnnouncementDetail string `json:"announcement_detail"`
    Evidence        []string `json:"evidence"`
    RiskFlags       []string `json:"risk_flags"`
    Execution       string   `json:"execution"`
    StopLoss        string   `json:"stop_loss"`
}

type HoldingBlockSummary struct {
    BlockName       string            `json:"block_name"`
    WeightPct       float64           `json:"weight_pct"`
    TargetPct       float64           `json:"target_pct"`
    Holdings        []HoldingRow      `json:"holdings"`
    BlockAssessment string            `json:"block_assessment"`
    BlockRisks      []string          `json:"block_risks"`
}

type HoldingRow struct {
    Ticker    string  `json:"ticker"`
    Price     float64 `json:"price"`
    PnLPct    float64 `json:"pnl_pct"`
    PBAS      float64 `json:"pbas"`
    Regime    string  `json:"regime"`
    VLI       string  `json:"vli"`
    Action    string  `json:"action"`
}

type PortfolioHealth struct {
    ConcentrationChecks []HealthCheck      `json:"concentration_checks"`
    QualityDistribution QualityDistribution `json:"quality_distribution"`
    RegimeDistribution  map[string]float64 `json:"regime_distribution"`
}

type HealthCheck struct {
    Check  string `json:"check"`
    Status string `json:"status"` // pass, warning, fail
    Detail string `json:"detail"`
}

type QualityDistribution struct {
    Underpriced int `json:"underpriced"` // PBAS > 0.65
    Fair        int `json:"fair"`        // PBAS 0.50-0.65
    Watch       int `json:"watch"`       // PBAS < 0.50
}

type Catalyst struct {
    Date    string `json:"date"`
    Ticker  string `json:"ticker"`
    Event   string `json:"event"`
    Impact  string `json:"impact"`
}

type ScreeningResults struct {
    Candidates []ScreeningCandidate `json:"candidates"`
}

type ScreeningCandidate struct {
    Ticker        string   `json:"ticker"`
    Name          string   `json:"name"`
    Sector        string   `json:"sector"`
    WhyFlagged    []string `json:"why_flagged"`
    TechnicalSetup string  `json:"technical_setup"`
    StrategyMatch []string `json:"strategy_match"`
}

type Appendix struct {
    SignalDefinitions map[string]string `json:"signal_definitions"`
    StrategyVersion   string            `json:"strategy_version"`
    DataSources       []string          `json:"data_sources"`
}
```

---

## Validation Helpers

```go
// ValidationResult represents the outcome of validating a model
type ValidationResult struct {
    Valid    bool     `json:"valid"`
    Errors   []string `json:"errors"`
    Warnings []string `json:"warnings"`
}

// Validator interface for all models
type Validator interface {
    Validate() ValidationResult
}
```

---

## Serialization

All models should implement:

```go
// ToYAML serializes the model to YAML
func (m *Model) ToYAML() ([]byte, error)

// ToJSON serializes the model to JSON
func (m *Model) ToJSON() ([]byte, error)

// FromYAML deserializes from YAML
func (m *Model) FromYAML(data []byte) error

// FromJSON deserializes from JSON
func (m *Model) FromJSON(data []byte) error
```

---

## Next Document
Proceed to `03-tool-specifications.md` for tool interface definitions.
