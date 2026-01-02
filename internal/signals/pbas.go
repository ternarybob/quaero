package signals

// PBASConfig holds configuration for PBAS calculation
type PBASConfig struct {
	// Weights for business momentum components
	WeightRevenue  float64 `json:"weight_revenue"`
	WeightOCF      float64 `json:"weight_ocf"`
	WeightMargin   float64 `json:"weight_margin"`
	WeightROIC     float64 `json:"weight_roic"`
	WeightDilution float64 `json:"weight_dilution"`

	// Sensitivity factor for sigmoid
	SensitivityK float64 `json:"sensitivity_k"`

	// Lookback period
	LookbackMonths int `json:"lookback_months"`
}

// DefaultPBASConfig returns the default configuration
func DefaultPBASConfig() PBASConfig {
	return PBASConfig{
		WeightRevenue:  0.35,
		WeightOCF:      0.25,
		WeightMargin:   0.20,
		WeightROIC:     0.10,
		WeightDilution: 0.10,
		SensitivityK:   5.0,
		LookbackMonths: 12,
	}
}

// PBASComputer computes Price-Business Alignment Score
type PBASComputer struct {
	config PBASConfig
}

// NewPBASComputer creates a new PBAS computer
func NewPBASComputer(config PBASConfig) *PBASComputer {
	return &PBASComputer{config: config}
}

// Compute calculates PBAS for a ticker
func (c *PBASComputer) Compute(raw TickerRaw) PBASSignal {
	// Handle missing fundamentals
	if !raw.HasFundamentals {
		return PBASSignal{
			Score:            0.5, // Neutral
			BusinessMomentum: 0,
			PriceMomentum:    raw.Price.Return52WPct / 100.0,
			Divergence:       0,
			Interpretation:   "neutral",
		}
	}

	// Extract and normalize components
	revGrowth := c.normalizeRevGrowth(raw.Fundamentals.RevenueYoYPct)
	ocfGrowth := c.estimateOCFGrowth(raw)
	marginDelta := c.normalizeMarginDelta(raw.Fundamentals.EBITDAMarginDeltaYoY)
	roicDelta := c.normalizeROICDelta(raw)
	dilution := c.normalizeDilution(raw.Fundamentals.Dilution12MPct)

	// Compute Business Momentum
	bm := c.config.WeightRevenue*revGrowth +
		c.config.WeightOCF*ocfGrowth +
		c.config.WeightMargin*marginDelta +
		c.config.WeightROIC*roicDelta -
		c.config.WeightDilution*dilution

	// Price Momentum (as decimal)
	pm := raw.Price.Return52WPct / 100.0

	// Compute divergence
	divergence := bm - pm

	// Apply sigmoid transformation
	pbas := sigmoid(c.config.SensitivityK * divergence)

	// Determine interpretation
	interpretation := "neutral"
	if pbas > 0.65 {
		interpretation = "underpriced"
	} else if pbas < 0.35 {
		interpretation = "overpriced"
	}

	return PBASSignal{
		Score:            round(pbas, 2),
		BusinessMomentum: round(bm, 3),
		PriceMomentum:    round(pm, 3),
		Divergence:       round(divergence, 3),
		Interpretation:   interpretation,
	}
}

// normalizeRevGrowth converts revenue growth to normalized score
// Revenue growth of 15%+ is excellent, 0-15% is moderate, negative is poor
func (c *PBASComputer) normalizeRevGrowth(revGrowthPct float64) float64 {
	// Convert to decimal and scale
	// -20% to +30% maps to approximately -0.3 to +0.3
	growth := revGrowthPct / 100.0
	normalized := growth * 1.0 // Simple 1:1 mapping, clamped
	return clamp(normalized, -0.3, 0.3)
}

// estimateOCFGrowth estimates OCF growth from available data
// Uses OCF/EBITDA ratio as a proxy for cash quality
func (c *PBASComputer) estimateOCFGrowth(raw TickerRaw) float64 {
	ocfToEBITDA := raw.Fundamentals.OCFToEBITDA

	// Higher cash conversion suggests healthier OCF growth
	if ocfToEBITDA >= 0.9 {
		return 0.2 // Strong cash conversion
	} else if ocfToEBITDA >= 0.7 {
		return 0.1
	} else if ocfToEBITDA >= 0.5 {
		return 0.0
	} else if ocfToEBITDA > 0 {
		return -0.1 // Poor cash conversion
	}
	return 0.0 // Unknown
}

// normalizeMarginDelta converts margin change to normalized score
// Margin delta of 2%+ is excellent, -2% or worse is concerning
func (c *PBASComputer) normalizeMarginDelta(deltaYoY float64) float64 {
	// Scale: -5% to +5% maps to -0.2 to +0.2
	normalized := deltaYoY / 25.0
	return clamp(normalized, -0.2, 0.2)
}

// normalizeROICDelta converts ROIC to normalized score
// Uses absolute ROIC level as proxy (delta not usually available)
func (c *PBASComputer) normalizeROICDelta(raw TickerRaw) float64 {
	// If ROIC not available, return neutral
	if raw.Fundamentals.ROICPct == 0 {
		return 0.0
	}

	// High ROIC is good indicator of quality
	roic := raw.Fundamentals.ROICPct
	if roic > 20 {
		return 0.15
	} else if roic > 15 {
		return 0.10
	} else if roic > 10 {
		return 0.05
	}
	return 0.0
}

// normalizeDilution converts dilution to penalty score
// Dilution is always a negative signal
func (c *PBASComputer) normalizeDilution(dilutionPct float64) float64 {
	// 0-2% is acceptable, 2-5% is concerning, 5-10% is bad, >10% is very bad
	if dilutionPct <= 2 {
		return 0.0
	} else if dilutionPct <= 5 {
		return 0.05
	} else if dilutionPct <= 10 {
		return 0.15
	}
	return 0.25 // >10% dilution
}
