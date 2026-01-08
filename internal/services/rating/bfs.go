package rating

import "fmt"

// CalculateBFS calculates the Business Foundation Score.
// Score: 0 (weak), 1 (developing), 2 (strong)
//
// Indicators evaluated:
// - Revenue > $10M TTM
// - Cash runway > 18 months
// - Has producing asset
// - Is profitable
//
// Scoring:
// - 0 indicators: Score 0
// - 1 indicator: Score 1
// - 2+ indicators: Score 2
func CalculateBFS(f Fundamentals) BFSResult {
	cashRunway := calculateCashRunway(f.CashBalance, f.QuarterlyCashBurn)

	hasRevenue := f.RevenueTTM > 10_000_000 // $10M threshold
	hasCashRunway := cashRunway > 18        // 18 months

	components := BFSComponents{
		HasRevenue:        hasRevenue,
		RevenueAmount:     f.RevenueTTM,
		CashRunwayMonths:  cashRunway,
		HasProducingAsset: f.HasProducingAsset,
		IsProfitable:      f.IsProfitable,
	}

	// Count indicators met
	indicators := 0
	if hasRevenue {
		indicators++
	}
	if hasCashRunway {
		indicators++
	}
	if f.HasProducingAsset {
		indicators++
	}
	if f.IsProfitable {
		indicators++
	}

	// Determine score
	score := 0
	if indicators >= 2 {
		score = 2
	} else if indicators == 1 {
		score = 1
	}

	// Build reasoning
	var reasoning string
	switch score {
	case 2:
		reasoning = fmt.Sprintf("Strong foundation: %d of 4 indicators met", indicators)
	case 1:
		reasoning = fmt.Sprintf("Developing foundation: %d of 4 indicators met", indicators)
	default:
		reasoning = "Weak foundation: No indicators met"
	}

	return BFSResult{
		Score:          score,
		IndicatorCount: indicators,
		Components:     components,
		Reasoning:      reasoning,
	}
}

// calculateCashRunway determines months of runway based on cash and burn rate
func calculateCashRunway(cashBalance, quarterlyCashBurn float64) float64 {
	if quarterlyCashBurn <= 0 {
		return 999 // Effectively infinite if not burning cash
	}
	return (cashBalance / quarterlyCashBurn) * 3 // Convert quarterly to monthly
}
