package signals

import "time"

// TickerRaw contains compressed market data for a single ticker.
// This is NOT raw OHLCV - it's pre-computed derived values from the stock collector.
type TickerRaw struct {
	Ticker         string    `json:"ticker"`
	FetchTimestamp time.Time `json:"fetch_timestamp"`

	Price            PriceData        `json:"price"`
	Volume           VolumeData       `json:"volume"`
	Volatility       VolatilityData   `json:"volatility"`
	RelativeStrength RSData           `json:"relative_strength"`
	Fundamentals     FundamentalsData `json:"fundamentals"`

	// Data quality flags
	HasFundamentals bool     `json:"has_fundamentals"`
	DataQuality     string   `json:"data_quality"` // complete, partial, stale
	Errors          []string `json:"errors,omitempty"`
}

// PriceData contains price-related metrics
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

// VolumeData contains volume-related metrics
type VolumeData struct {
	Current      int64   `json:"current"`
	SMA20        float64 `json:"sma_20"`
	SMA50        float64 `json:"sma_50"`
	ZScore20     float64 `json:"zscore_20"`
	Trend5Dvs20D string  `json:"trend_5d_vs_20d"` // rising, falling, flat
}

// VolatilityData contains volatility metrics
type VolatilityData struct {
	ATR14         float64 `json:"atr_14"`
	ATR21         float64 `json:"atr_21"`
	ATRPctOfPrice float64 `json:"atr_pct_of_price"`
}

// RSData contains raw relative strength data (before signal computation)
type RSData struct {
	VsXJO3M    float64 `json:"vs_xjo_3m"`
	VsXJO6M    float64 `json:"vs_xjo_6m"`
	VsSector3M float64 `json:"vs_sector_3m,omitempty"`
}

// FundamentalsData contains fundamental metrics
type FundamentalsData struct {
	// Valuation
	MarketCapM       float64 `json:"market_cap_m"`
	PERatio          float64 `json:"pe_ratio,omitempty"`
	PEVsSectorMedian float64 `json:"pe_vs_sector_median,omitempty"`

	// Revenue
	RevenueTTMM   float64 `json:"revenue_ttm_m"`
	RevenueYoYPct float64 `json:"revenue_yoy_pct"`

	// Margins
	EBITDAMarginPct      float64 `json:"ebitda_margin_pct"`
	EBITDAMarginDeltaYoY float64 `json:"ebitda_margin_delta_yoy"`
	GrossMarginPct       float64 `json:"gross_margin_pct,omitempty"`

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

// IsComplete returns true if the raw data has all essential fields
func (tr *TickerRaw) IsComplete() bool {
	return tr.Price.Current > 0 && tr.Price.EMA200 > 0
}

// HasFundamentalData returns true if fundamental data is available
func (tr *TickerRaw) HasFundamentalData() bool {
	return tr.HasFundamentals && tr.Fundamentals.MarketCapM > 0
}
