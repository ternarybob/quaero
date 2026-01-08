package rating

import "fmt"

// CalculateNFR calculates the Narrative-to-Fact Ratio.
// Score: 0.0 to 1.0
//
// Classifies announcements as:
// - Fact-based: quarterly reports, annual reports, contracts, acquisitions
// - Narrative-based: drilling results, trading halts (many are speculative), other
//
// Higher fact ratio = better signal quality
func CalculateNFR(announcements []Announcement) NFRResult {
	if len(announcements) == 0 {
		return NFRResult{
			Score: 0.5, // Neutral if no data
			Components: NFRComponents{
				TotalAnnouncements:     0,
				FactAnnouncements:      0,
				NarrativeAnnouncements: 0,
				FactRatio:              0.5,
			},
			Reasoning: "Neutral: No announcements available for analysis",
		}
	}

	factCount := 0
	narrativeCount := 0

	for _, a := range announcements {
		if isFactBased(a.Type) {
			factCount++
		} else {
			narrativeCount++
		}
	}

	total := factCount + narrativeCount
	factRatio := 0.0
	if total > 0 {
		factRatio = float64(factCount) / float64(total)
	}

	components := NFRComponents{
		TotalAnnouncements:     total,
		FactAnnouncements:      factCount,
		NarrativeAnnouncements: narrativeCount,
		FactRatio:              factRatio,
	}

	// Score is directly the fact ratio (0.0 to 1.0)
	score := ClampFloat64(factRatio, 0, 1)

	// Build reasoning
	var reasoning string
	if score >= 0.7 {
		reasoning = fmt.Sprintf("High quality: %.0f%% fact-based announcements (%d/%d)",
			factRatio*100, factCount, total)
	} else if score >= 0.4 {
		reasoning = fmt.Sprintf("Mixed quality: %.0f%% fact-based announcements (%d/%d)",
			factRatio*100, factCount, total)
	} else {
		reasoning = fmt.Sprintf("Low quality: %.0f%% fact-based announcements (%d/%d)",
			factRatio*100, factCount, total)
	}

	return NFRResult{
		Score:      score,
		Components: components,
		Reasoning:  reasoning,
	}
}

// isFactBased classifies announcement type
func isFactBased(t AnnouncementType) bool {
	switch t {
	case TypeQuarterly, TypeAnnualReport, TypeContract, TypeAcquisition:
		return true
	case TypeTradingHalt, TypeCapitalRaise, TypeDrilling, TypeOther:
		return false
	default:
		return false
	}
}
