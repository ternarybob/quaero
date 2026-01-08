package rating

import "fmt"

// CalculateCDS calculates the Capital Discipline Score.
// Score: 0 (poor), 1 (moderate), 2 (strong)
//
// Factors evaluated:
// - Shares outstanding CAGR (dilution)
// - Trading halts per annum
// - Capital raises per annum
//
// Scoring:
// - Strong (2): CAGR <= 15%, halts/yr <= 2, raises/yr <= 1
// - Moderate (1): CAGR <= 30%, halts/yr <= 4, raises/yr <= 2
// - Poor (0): Exceeds moderate thresholds
func CalculateCDS(f Fundamentals, announcements []Announcement, months int) CDSResult {
	if months <= 0 {
		months = 36 // Default to 3 years
	}
	years := float64(months) / 12

	sharesCAGR := calculateSharesCAGR(f.SharesOutstandingCurrent, f.SharesOutstanding3YAgo)
	haltsPA := countByType(announcements, TypeTradingHalt) / years
	raisesPA := countByType(announcements, TypeCapitalRaise) / years

	components := CDSComponents{
		SharesCAGR:       sharesCAGR,
		TradingHaltsPA:   haltsPA,
		CapitalRaisesPA:  raisesPA,
		AnalysisPeriodMo: months,
	}

	// Score based on dilution and capital market activity
	score := 2 // Start with strong

	// Degrade to moderate
	if sharesCAGR > 0.15 || haltsPA > 2 || raisesPA > 1 {
		score = 1
	}

	// Degrade to poor
	if sharesCAGR > 0.30 || haltsPA > 4 || raisesPA > 2 {
		score = 0
	}

	// Build reasoning
	var reasoning string
	switch score {
	case 2:
		reasoning = fmt.Sprintf("Strong discipline: CAGR=%.1f%%, halts/yr=%.1f, raises/yr=%.1f",
			sharesCAGR*100, haltsPA, raisesPA)
	case 1:
		reasoning = fmt.Sprintf("Moderate discipline: CAGR=%.1f%%, halts/yr=%.1f, raises/yr=%.1f",
			sharesCAGR*100, haltsPA, raisesPA)
	default:
		reasoning = fmt.Sprintf("Poor discipline: CAGR=%.1f%%, halts/yr=%.1f, raises/yr=%.1f",
			sharesCAGR*100, haltsPA, raisesPA)
	}

	return CDSResult{
		Score:      score,
		Components: components,
		Reasoning:  reasoning,
	}
}

// calculateSharesCAGR determines share count growth rate
func calculateSharesCAGR(current int64, threeYearsAgo *int64) float64 {
	if threeYearsAgo == nil || *threeYearsAgo == 0 {
		return 0
	}
	return CAGR(float64(*threeYearsAgo), float64(current), 3)
}

// countByType counts announcements of a specific type
func countByType(announcements []Announcement, annType AnnouncementType) float64 {
	count := 0
	for _, a := range announcements {
		if a.Type == annType {
			count++
		}
	}
	return float64(count)
}
