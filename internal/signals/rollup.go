package signals

import "time"

// PortfolioRollup aggregates portfolio-level metrics
type PortfolioRollup struct {
	Meta        RollupMeta         `json:"meta"`
	Performance PerformanceMetrics `json:"performance"`
	Allocation  AllocationMetrics  `json:"allocation"`

	ConcentrationAlerts []string             `json:"concentration_alerts"`
	CorrelationClusters []CorrelationCluster `json:"correlation_clusters,omitempty"`

	ActionSummary ActionSummary `json:"action_summary"`
	CashPosition  CashPosition  `json:"cash_position,omitempty"`

	RebalanceSuggestions []RebalanceSuggestion `json:"rebalance_suggestions,omitempty"`
}

// RollupMeta contains rollup metadata
type RollupMeta struct {
	AsOf             time.Time `json:"as_of"`
	HoldingsAssessed int       `json:"holdings_assessed"`
	AssessmentsValid int       `json:"assessments_valid"`
}

// PerformanceMetrics contains return metrics
type PerformanceMetrics struct {
	TotalValue  float64 `json:"total_value"`
	TotalCost   float64 `json:"total_cost"`
	TotalPnL    float64 `json:"total_pnl"`
	TotalPnLPct float64 `json:"total_pnl_pct"`

	Return1MPct  float64 `json:"return_1m_pct,omitempty"`
	Return3MPct  float64 `json:"return_3m_pct,omitempty"`
	ReturnYTDPct float64 `json:"return_ytd_pct,omitempty"`

	VsXJOYTDPct float64 `json:"vs_xjo_ytd_pct,omitempty"`
	VsXSOYTDPct float64 `json:"vs_xso_ytd_pct,omitempty"`
}

// AllocationMetrics contains allocation breakdowns
type AllocationMetrics struct {
	BySector      map[string]float64 `json:"by_sector"`
	ByHoldingType map[string]float64 `json:"by_holding_type"`
	ByRegime      map[string]float64 `json:"by_regime"`
}

// CorrelationCluster represents a group of correlated holdings
type CorrelationCluster struct {
	Tickers     []string `json:"tickers"`
	Correlation float64  `json:"correlation"`
	Note        string   `json:"note"`
}

// ActionSummary summarizes actions across the portfolio
type ActionSummary struct {
	ImmediateActions int          `json:"immediate_actions"`
	WatchClosely     int          `json:"watch_closely"`
	HoldNoAction     int          `json:"hold_no_action"`
	Actions          []ActionItem `json:"actions"`
}

// ActionItem represents an individual action
type ActionItem struct {
	Ticker  string `json:"ticker"`
	Action  string `json:"action"`
	Urgency string `json:"urgency"`
	Reason  string `json:"reason"`
}

// CashPosition represents the portfolio's cash position
type CashPosition struct {
	CurrentPct     float64 `json:"current_pct"`
	TargetPct      float64 `json:"target_pct"`
	Recommendation string  `json:"recommendation"`
}

// RebalanceSuggestion represents a rebalancing suggestion
type RebalanceSuggestion struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Reason string `json:"reason"`
}

// Concentration limit constants
const (
	MaxPositionPct = 8.0  // Single position limit
	MaxTop5Pct     = 40.0 // Top 5 positions limit
	MaxSectorPct   = 30.0 // Single sector limit
)

// HasAlerts returns true if there are concentration alerts
func (pr *PortfolioRollup) HasAlerts() bool {
	return len(pr.ConcentrationAlerts) > 0
}

// HasUrgentActions returns true if there are immediate actions
func (pr *PortfolioRollup) HasUrgentActions() bool {
	return pr.ActionSummary.ImmediateActions > 0
}

// GetTotalActions returns the total number of actions
func (pr *PortfolioRollup) GetTotalActions() int {
	return len(pr.ActionSummary.Actions)
}
