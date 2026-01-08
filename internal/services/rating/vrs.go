package rating

import (
	"fmt"
	"sort"
)

// CalculateVRS calculates the Volatility Regime Stability score.
// Score: 0.0 to 1.0
//
// Measures consistency of volatility patterns over time.
// Stable volatility = more predictable risk profile
//
// Process:
// 1. Calculate rolling volatility (e.g., 20-day window)
// 2. Identify volatility regimes (low/medium/high)
// 3. Count regime transitions
// 4. Score based on stability (fewer transitions = higher score)
func CalculateVRS(prices []PriceBar) VRSResult {
	if len(prices) < 40 { // Need minimum history for rolling calculation
		return VRSResult{
			Score: 0.5, // Neutral if insufficient data
			Components: VRSComponents{
				RegimeCount:       0,
				StableRegimesPct:  0,
				VolatilityPattern: "insufficient_data",
			},
			Reasoning: "Neutral: Insufficient price history for volatility analysis",
		}
	}

	// Sort prices by date
	sortedPrices := make([]PriceBar, len(prices))
	copy(sortedPrices, prices)
	sort.Slice(sortedPrices, func(i, j int) bool {
		return sortedPrices[i].Date.Before(sortedPrices[j].Date)
	})

	// Calculate daily returns
	returns := DailyReturns(sortedPrices)
	if len(returns) < 30 {
		return VRSResult{
			Score: 0.5,
			Components: VRSComponents{
				RegimeCount:       0,
				StableRegimesPct:  0,
				VolatilityPattern: "insufficient_data",
			},
			Reasoning: "Neutral: Insufficient return data for volatility analysis",
		}
	}

	// Calculate rolling volatility (20-day window)
	const windowSize = 20
	rollingVol := RollingVolatility(returns, windowSize)
	if len(rollingVol) < 10 {
		return VRSResult{
			Score: 0.5,
			Components: VRSComponents{
				RegimeCount:       0,
				StableRegimesPct:  0,
				VolatilityPattern: "insufficient_data",
			},
			Reasoning: "Neutral: Insufficient data for regime detection",
		}
	}

	// Classify into regimes based on percentiles
	regimes := classifyRegimes(rollingVol)

	// Count regime transitions
	transitions := countTransitions(regimes)
	regimeCount := transitions + 1

	// Calculate percentage of time in stable (low/medium) regimes
	stableCount := 0
	for _, r := range regimes {
		if r == "low" || r == "medium" {
			stableCount++
		}
	}
	stableRegimesPct := float64(stableCount) / float64(len(regimes)) * 100

	// Determine dominant pattern
	pattern := determinePattern(regimes)

	// Score based on stability
	// Fewer transitions = higher score
	// More time in stable regimes = higher score
	transitionScore := 1.0 - ClampFloat64(float64(transitions)/20.0, 0, 1)
	stabilityScore := stableRegimesPct / 100

	score := ClampFloat64((transitionScore+stabilityScore)/2, 0, 1)

	components := VRSComponents{
		RegimeCount:       regimeCount,
		StableRegimesPct:  stableRegimesPct,
		VolatilityPattern: pattern,
	}

	// Build reasoning
	var reasoning string
	if score >= 0.7 {
		reasoning = fmt.Sprintf("Stable volatility: %d regimes, %.0f%% stable periods, pattern=%s",
			regimeCount, stableRegimesPct, pattern)
	} else if score >= 0.4 {
		reasoning = fmt.Sprintf("Moderate volatility: %d regimes, %.0f%% stable periods, pattern=%s",
			regimeCount, stableRegimesPct, pattern)
	} else {
		reasoning = fmt.Sprintf("Unstable volatility: %d regimes, %.0f%% stable periods, pattern=%s",
			regimeCount, stableRegimesPct, pattern)
	}

	return VRSResult{
		Score:      score,
		Components: components,
		Reasoning:  reasoning,
	}
}

// classifyRegimes assigns regime labels based on volatility percentiles
func classifyRegimes(rollingVol []float64) []string {
	if len(rollingVol) == 0 {
		return nil
	}

	// Calculate percentiles
	sorted := make([]float64, len(rollingVol))
	copy(sorted, rollingVol)
	sort.Float64s(sorted)

	p33 := sorted[len(sorted)/3]
	p67 := sorted[len(sorted)*2/3]

	regimes := make([]string, len(rollingVol))
	for i, v := range rollingVol {
		if v <= p33 {
			regimes[i] = "low"
		} else if v <= p67 {
			regimes[i] = "medium"
		} else {
			regimes[i] = "high"
		}
	}
	return regimes
}

// countTransitions counts regime changes
func countTransitions(regimes []string) int {
	if len(regimes) < 2 {
		return 0
	}

	transitions := 0
	for i := 1; i < len(regimes); i++ {
		if regimes[i] != regimes[i-1] {
			transitions++
		}
	}
	return transitions
}

// determinePattern identifies the dominant volatility pattern
func determinePattern(regimes []string) string {
	if len(regimes) == 0 {
		return "unknown"
	}

	counts := map[string]int{"low": 0, "medium": 0, "high": 0}
	for _, r := range regimes {
		counts[r]++
	}

	// Find dominant regime
	maxCount := 0
	dominant := "unknown"
	for regime, count := range counts {
		if count > maxCount {
			maxCount = count
			dominant = regime
		}
	}

	// Check for trending pattern (increasing or decreasing over time)
	if len(regimes) >= 10 {
		firstHalf := regimes[:len(regimes)/2]
		secondHalf := regimes[len(regimes)/2:]

		firstHighCount := 0
		secondHighCount := 0
		for _, r := range firstHalf {
			if r == "high" {
				firstHighCount++
			}
		}
		for _, r := range secondHalf {
			if r == "high" {
				secondHighCount++
			}
		}

		firstHighPct := float64(firstHighCount) / float64(len(firstHalf))
		secondHighPct := float64(secondHighCount) / float64(len(secondHalf))

		if secondHighPct > firstHighPct+0.2 {
			return "increasing_volatility"
		} else if firstHighPct > secondHighPct+0.2 {
			return "decreasing_volatility"
		}
	}

	return "predominantly_" + dominant
}
