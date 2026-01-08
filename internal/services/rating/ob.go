package rating

import (
	"regexp"
	"strings"
	"time"
)

// CalculateOB calculates the Optionality Bonus.
// Score: 0.0, 0.5, or 1.0
//
// Evaluates presence of:
// - Clear catalyst (specific event that could move the stock)
// - Defined timeframe (when the catalyst is expected)
//
// Scoring:
// - 0.0: No catalyst found
// - 0.5: Catalyst found but no defined timeframe
// - 1.0: Catalyst with defined timeframe
func CalculateOB(announcements []Announcement, bfsScore int) OBResult {
	if len(announcements) == 0 {
		return OBResult{
			Score:          0,
			CatalystFound:  false,
			TimeframeFound: false,
			Reasoning:      "No optionality: No announcements to analyze",
		}
	}

	// Only look for optionality if business foundation exists (BFS >= 1)
	if bfsScore < 1 {
		return OBResult{
			Score:          0,
			CatalystFound:  false,
			TimeframeFound: false,
			Reasoning:      "No optionality: Weak business foundation (BFS < 1)",
		}
	}

	// Look for catalyst keywords in recent announcements
	catalystFound := false
	timeframeFound := false

	// Consider only recent announcements (last 90 days)
	cutoff := time.Now().AddDate(0, 0, -90)

	for _, ann := range announcements {
		if ann.Date.Before(cutoff) {
			continue
		}

		headline := strings.ToLower(ann.Headline)

		// Check for catalyst indicators
		if hasCatalyst(headline) {
			catalystFound = true
		}

		// Check for timeframe indicators
		if hasTimeframe(headline) {
			timeframeFound = true
		}
	}

	// Determine score
	var score float64
	var reasoning string

	if catalystFound && timeframeFound {
		score = 1.0
		reasoning = "Full optionality: Catalyst with defined timeframe identified"
	} else if catalystFound {
		score = 0.5
		reasoning = "Partial optionality: Catalyst found but timeframe unclear"
	} else {
		score = 0
		reasoning = "No optionality: No near-term catalysts identified"
	}

	return OBResult{
		Score:          score,
		CatalystFound:  catalystFound,
		TimeframeFound: timeframeFound,
		Reasoning:      reasoning,
	}
}

// hasCatalyst checks for catalyst keywords in headline
func hasCatalyst(headline string) bool {
	catalystPatterns := []string{
		"drilling",
		"results",
		"contract",
		"agreement",
		"acquisition",
		"merger",
		"fda approval",
		"trial results",
		"production",
		"commissioning",
		"offtake",
		"resource upgrade",
		"feasibility",
		"license",
		"permit",
		"approval",
	}

	for _, pattern := range catalystPatterns {
		if strings.Contains(headline, pattern) {
			return true
		}
	}
	return false
}

// hasTimeframe checks for timeframe indicators in headline
func hasTimeframe(headline string) bool {
	// Patterns for explicit timeframes
	timeframePatterns := []string{
		"q1", "q2", "q3", "q4",
		"2024", "2025", "2026",
		"january", "february", "march", "april", "may", "june",
		"july", "august", "september", "october", "november", "december",
		"this quarter", "next quarter",
		"this month", "next month",
		"imminent", "pending",
		"expected",
		"scheduled",
		"planned for",
	}

	for _, pattern := range timeframePatterns {
		if strings.Contains(headline, pattern) {
			return true
		}
	}

	// Check for "within X weeks/months" patterns
	withinPattern := regexp.MustCompile(`within\s+\d+\s+(week|month|day)`)
	if withinPattern.MatchString(headline) {
		return true
	}

	return false
}
