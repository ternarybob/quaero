package signals

import (
	"errors"
	"time"
)

// Holding represents a single portfolio position
type Holding struct {
	// Identification
	Ticker string `json:"ticker"`
	Name   string `json:"name"`

	// Classification
	Sector      string `json:"sector"`
	Industry    string `json:"industry,omitempty"`
	HoldingType string `json:"holding_type"` // smsf, trader

	// Position
	Units    float64 `json:"units"`
	AvgPrice float64 `json:"avg_price"`

	// Targets
	TargetWeightPct float64 `json:"target_weight_pct"`

	// Computed (set after load)
	CostBasis float64 `json:"cost_basis"`
}

// Validate performs validation on the holding
func (h *Holding) Validate() error {
	if h.Ticker == "" {
		return errors.New("ticker is required")
	}
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

// PortfolioMeta contains portfolio metadata
type PortfolioMeta struct {
	Name               string    `json:"name"`
	AsOf               time.Time `json:"as_of"`
	TotalHoldings      int       `json:"total_holdings"`
	BenchmarkPrimary   string    `json:"benchmark_primary"`   // e.g., XJO
	BenchmarkSecondary string    `json:"benchmark_secondary"` // e.g., XSO
	BaseCurrency       string    `json:"base_currency"`       // e.g., AUD
}

// PortfolioState represents the complete portfolio snapshot
type PortfolioState struct {
	// Metadata
	Meta PortfolioMeta `json:"meta"`

	// Holdings
	Holdings []Holding `json:"holdings"`

	// Aggregations (computed)
	SectorAllocation map[string]float64 `json:"sector_allocation"`
	HoldingTypeSplit map[string]float64 `json:"holding_type_split"`
	TotalCostBasis   float64            `json:"total_cost_basis"`
}

// Validate performs validation on the portfolio state
func (ps *PortfolioState) Validate() error {
	if len(ps.Holdings) == 0 {
		return errors.New("portfolio has no holdings")
	}

	seenTickers := make(map[string]bool)
	for _, h := range ps.Holdings {
		if err := h.Validate(); err != nil {
			return err
		}
		if seenTickers[h.Ticker] {
			return errors.New("duplicate ticker: " + h.Ticker)
		}
		seenTickers[h.Ticker] = true
	}

	return nil
}

// ComputeAggregations calculates derived values
func (ps *PortfolioState) ComputeAggregations() {
	ps.SectorAllocation = make(map[string]float64)
	ps.HoldingTypeSplit = make(map[string]float64)
	ps.TotalCostBasis = 0

	for i := range ps.Holdings {
		ps.Holdings[i].ComputeCostBasis()
		ps.TotalCostBasis += ps.Holdings[i].CostBasis
	}

	// Calculate allocations (by cost basis)
	if ps.TotalCostBasis > 0 {
		for _, h := range ps.Holdings {
			pct := h.CostBasis / ps.TotalCostBasis * 100
			ps.SectorAllocation[h.Sector] += pct
			ps.HoldingTypeSplit[h.HoldingType] += pct
		}
	}

	ps.Meta.TotalHoldings = len(ps.Holdings)
}

// GetHoldingTypes returns a map of ticker to holding type
func (ps *PortfolioState) GetHoldingTypes() map[string]string {
	result := make(map[string]string)
	for _, h := range ps.Holdings {
		result[h.Ticker] = h.HoldingType
	}
	return result
}

// GetTickers returns a list of all tickers in the portfolio
func (ps *PortfolioState) GetTickers() []string {
	tickers := make([]string, len(ps.Holdings))
	for i, h := range ps.Holdings {
		tickers[i] = h.Ticker
	}
	return tickers
}
