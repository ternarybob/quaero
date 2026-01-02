package signals

// CookedDetector detects overvalued/decoupled stocks
type CookedDetector struct {
	threshold int // Number of triggers to flag as cooked (default: 2)
}

// NewCookedDetector creates a new cooked detector
func NewCookedDetector() *CookedDetector {
	return &CookedDetector{threshold: 2}
}

// Detect checks if a stock is "cooked" (overvalued/decoupled from fundamentals)
func (d *CookedDetector) Detect(raw TickerRaw, pbas PBASSignal) CookedSignal {
	triggers := 0
	reasons := make([]string, 0)

	// Trigger 1: PBAS too low (price ahead of business)
	if pbas.Score < 0.30 {
		triggers++
		reasons = append(reasons, "pbas_below_0.30")
	}

	// Trigger 2: Price-Revenue divergence
	// Price up significantly more than justified by revenue growth
	priceReturn := raw.Price.Return52WPct
	revGrowth := raw.Fundamentals.RevenueYoYPct

	if priceReturn > 30 && revGrowth > 0 && priceReturn > (revGrowth*2.5) {
		triggers++
		reasons = append(reasons, "price_revenue_divergence")
	}

	// Also trigger if revenue is negative but price is up significantly
	if priceReturn > 20 && revGrowth < 0 {
		triggers++
		reasons = append(reasons, "price_up_revenue_down")
	}

	// Trigger 3: High dilution
	if raw.Fundamentals.Dilution12MPct > 10 {
		triggers++
		reasons = append(reasons, "dilution_above_10pct")
	}

	// Trigger 4: Poor cash conversion
	// OCF/EBITDA below 60% and positive (not just missing)
	ocfToEBITDA := raw.Fundamentals.OCFToEBITDA
	if ocfToEBITDA > 0 && ocfToEBITDA < 0.60 {
		triggers++
		reasons = append(reasons, "poor_cash_conversion")
	}

	// Trigger 5: Extended above 200 EMA
	if raw.Price.EMA200 > 0 {
		priceVs200 := raw.Price.Current / raw.Price.EMA200
		if priceVs200 > 1.30 {
			triggers++
			reasons = append(reasons, "extended_above_200ema")
		}
	}

	isCooked := triggers >= d.threshold

	// Only return reasons if actually cooked
	if !isCooked {
		reasons = nil
	}

	return CookedSignal{
		IsCooked: isCooked,
		Score:    triggers,
		Reasons:  reasons,
	}
}

// Trigger constants for reference
const (
	CookedTriggerLowPBAS            = "pbas_below_0.30"
	CookedTriggerPriceRevDivergence = "price_revenue_divergence"
	CookedTriggerPriceUpRevDown     = "price_up_revenue_down"
	CookedTriggerHighDilution       = "dilution_above_10pct"
	CookedTriggerPoorCashConversion = "poor_cash_conversion"
	CookedTriggerExtendedAboveEMA   = "extended_above_200ema"
)
