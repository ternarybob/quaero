// Package navexa provides a client for the Navexa portfolio API.
package navexa

// Portfolio represents a Navexa portfolio from the /v1/portfolios endpoint.
type Portfolio struct {
	ID               int    `json:"id"`
	Name             string `json:"name"`
	DateCreated      string `json:"dateCreated"`
	BaseCurrencyCode string `json:"baseCurrencyCode"`
}

// Holding represents a holding from the /v1/portfolios/{id}/holdings endpoint.
type Holding struct {
	ID                int     `json:"id"`
	Symbol            string  `json:"symbol"`
	Exchange          string  `json:"exchange"`
	Name              string  `json:"name"`
	SectorID          int     `json:"sectorId"`
	SectorCode        string  `json:"sectorCode"`
	SectorName        string  `json:"sectorName"`
	IndustryGroupID   int     `json:"industryGroupId"`
	IndustryGroupCode string  `json:"industryGroupCode"`
	IndustryGroupName string  `json:"industryGroupName"`
	IndustryID        int     `json:"industryId"`
	IndustryCode      string  `json:"industryCode"`
	IndustryName      string  `json:"industryName"`
	SubIndustryID     int     `json:"subIndustryId"`
	SubIndustryCode   string  `json:"subIndustryCode"`
	SubIndustryName   string  `json:"subIndustryName"`
	HoldingTypeID     int     `json:"holdingTypeId"`
	DateCreated       string  `json:"dateCreated"`
	TotalQuantity     float64 `json:"totalQuantity,omitempty"` // Added when enriched from performance
}

// PerformanceHolding represents a holding with performance data from the performance API.
type PerformanceHolding struct {
	Symbol        string  `json:"symbol"`
	Name          string  `json:"name"`
	Exchange      string  `json:"exchange"`
	TotalQuantity float64 `json:"totalQuantity"`
	HoldingWeight float64 `json:"holdingWeight"`
	CurrencyCode  string  `json:"currencyCode"`
	TotalReturn   struct {
		TotalValue   float64 `json:"totalValue"`
		TotalCost    float64 `json:"totalCost"`
		CostBasis    float64 `json:"costBasis"`
		CapitalGain  float64 `json:"capitalGain"`
		Dividends    float64 `json:"dividends"`
		CurrencyGain float64 `json:"currencyGain"`
		ReturnPct    float64 `json:"returnPercent"`
	} `json:"totalReturn"`
}

// TotalReturnInfo represents the return details object in performance responses.
type TotalReturnInfo struct {
	TotalValue   float64 `json:"totalValue"`
	TotalCost    float64 `json:"totalCost"`
	CostBasis    float64 `json:"costBasis"`
	CapitalGain  float64 `json:"capitalGain"`
	Dividends    float64 `json:"dividends"`
	CurrencyGain float64 `json:"currencyGain"`
	ReturnPct    float64 `json:"returnPercent"`
}

// PerformanceResponse represents the raw performance API response.
type PerformanceResponse struct {
	Holdings         []PerformanceHolding `json:"holdings"`
	BaseCurrencyCode string               `json:"baseCurrencyCode"`
	TotalValue       float64              `json:"totalValue"`
	TotalCost        float64              `json:"totalCost"`
	CostBasis        float64              `json:"costBasis"`
	TotalReturn      TotalReturnInfo      `json:"totalReturn"`
	CapitalGain      float64              `json:"capitalGain"`
	Dividends        float64              `json:"dividends"`
	CurrencyGain     float64              `json:"currencyGain"`
	ReturnPct        float64              `json:"returnPercent"`
}

// PortfolioWithHoldings represents a portfolio with its holdings enriched with performance data.
type PortfolioWithHoldings struct {
	Portfolio Portfolio
	Holdings  []EnrichedHolding
	FetchedAt string
}

// EnrichedHolding combines holding info with performance metrics.
type EnrichedHolding struct {
	Symbol        string  `json:"symbol"`
	Name          string  `json:"name"`
	Exchange      string  `json:"exchange"`
	Quantity      float64 `json:"quantity"`
	AvgBuyPrice   float64 `json:"avgBuyPrice"`
	CurrentValue  float64 `json:"currentValue"`
	HoldingWeight float64 `json:"holdingWeight"`
	CurrencyCode  string  `json:"currencyCode"`
}

// PortfolioInput defines the supported input types for portfolio operations.
type PortfolioInput struct {
	// Type specifies the input type: "ticker_list", "navexa_portfolio", "navexa_portfolio_id"
	Type string

	// Tickers is a list of ticker symbols (for "ticker_list" type)
	Tickers []string

	// NavexaPortfolioName is the portfolio name to search for (for "navexa_portfolio" type)
	NavexaPortfolioName string

	// NavexaPortfolioID is the explicit portfolio ID (for "navexa_portfolio_id" type)
	NavexaPortfolioID int

	// CacheHours specifies how long to cache portfolio data (default: 24)
	CacheHours int

	// ForceRefresh bypasses cache if true
	ForceRefresh bool
}

// PortfolioInputType constants
const (
	InputTypeTickerList        = "ticker_list"
	InputTypeNavexaPortfolio   = "navexa_portfolio"
	InputTypeNavexaPortfolioID = "navexa_portfolio_id"
)
