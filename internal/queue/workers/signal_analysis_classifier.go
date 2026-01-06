// -----------------------------------------------------------------------
// SignalAnalysisClassifier - Classification and scoring logic
// Pure functions for announcement classification and aggregate scoring
// -----------------------------------------------------------------------

package workers

import (
	"math"
	"strings"
)

// Classification constants define the signal classifications
const (
	// ClassificationTrueSignal indicates genuine information signal with market impact
	ClassificationTrueSignal = "TRUE_SIGNAL"

	// ClassificationPricedIn indicates information was already reflected in price
	ClassificationPricedIn = "PRICED_IN"

	// ClassificationSentimentNoise indicates routine filing generating unexpected reaction
	ClassificationSentimentNoise = "SENTIMENT_NOISE"

	// ClassificationManagementBluff indicates price-sensitive marking without market impact
	ClassificationManagementBluff = "MANAGEMENT_BLUFF"

	// ClassificationRoutine indicates standard filing with normal market response
	ClassificationRoutine = "ROUTINE"
)

// Communication style constants
const (
	// StyleTransparent indicates data-driven, credible disclosure
	StyleTransparent = "TRANSPARENT_DATA_DRIVEN"

	// StylePromotional indicates noisy or low-credibility disclosure
	StylePromotional = "PROMOTIONAL_SENTIMENT"

	// StyleLeaky indicates potential information leakage
	StyleLeaky = "LEAKY_INSIDER_RISK"

	// StyleStandard indicates normal disclosure practices
	StyleStandard = "STANDARD"
)

// ClassifyAnnouncement determines the signal classification based on metrics.
//
// Decision matrix:
//   - TRUE_SIGNAL: volume_ratio > 2.0 AND abs(day_of_change) > 3% AND abs(pre_drift) < 2%
//   - PRICED_IN: abs(pre_drift) > 2% AND abs(day_of_change) < 1%
//   - SENTIMENT_NOISE: isRoutineCategory AND (volume_ratio > 1.5 OR abs(day_of_change) > 2%)
//   - MANAGEMENT_BLUFF: price_sensitive AND volume_ratio < 0.8
//   - ROUTINE: default
func ClassifyAnnouncement(metrics ClassificationMetrics, priceSensitive bool, category string) string {
	absChange := math.Abs(metrics.DayOfChange)
	absPreDrift := math.Abs(metrics.PreDrift)

	// TRUE_SIGNAL: Strong market reaction without prior leak
	// Criteria: High volume, high price change, minimal pre-drift (no information leakage)
	if metrics.VolumeRatio > 2.0 && absChange > 3.0 && absPreDrift < 2.0 {
		return ClassificationTrueSignal
	}

	// PRICED_IN: Information already reflected in market price
	// Criteria: Significant pre-drift (price moved before announcement) but minimal day-of reaction
	if absPreDrift > 2.0 && absChange < 1.0 {
		return ClassificationPricedIn
	}

	// SENTIMENT_NOISE: Routine filing generating unexpected market reaction
	// Criteria: Routine category but with volume or price reaction (retail speculation)
	if isRoutineCategory(category) && (metrics.VolumeRatio > 1.5 || absChange > 2.0) {
		return ClassificationSentimentNoise
	}

	// MANAGEMENT_BLUFF: Marked as price-sensitive but no market impact
	// Criteria: Management said it's important but market disagrees
	if priceSensitive && metrics.VolumeRatio < 0.8 {
		return ClassificationManagementBluff
	}

	// ROUTINE: Normal market response to normal announcement
	return ClassificationRoutine
}

// CalculateAggregates computes summary metrics from a list of classifications.
// This is a pure function with no side effects.
func CalculateAggregates(classifications []AnnouncementClassification) SignalSummary {
	summary := SignalSummary{
		TotalAnnouncements: len(classifications),
	}

	if len(classifications) == 0 {
		// Empty case - set defaults
		summary.ConvictionScore = 5
		summary.CommunicationStyle = StyleStandard
		return summary
	}

	priceSensitiveCount := 0

	// Count classifications
	for _, c := range classifications {
		switch c.Classification {
		case ClassificationTrueSignal:
			summary.CountTrueSignal++
		case ClassificationPricedIn:
			summary.CountPricedIn++
		case ClassificationSentimentNoise:
			summary.CountSentimentNoise++
		case ClassificationManagementBluff:
			summary.CountManagementBluff++
		case ClassificationRoutine:
			summary.CountRoutine++
		}

		if c.ManagementSensitive {
			priceSensitiveCount++
		}
	}

	// Calculate ratios
	total := float64(summary.TotalAnnouncements)
	if total > 0 {
		summary.SignalRatio = float64(summary.CountTrueSignal) / total
		summary.NoiseRatio = float64(summary.CountSentimentNoise) / total
	}

	// Calculate price-sensitive-dependent metrics
	if priceSensitiveCount > 0 {
		psCount := float64(priceSensitiveCount)
		summary.LeakScore = float64(summary.CountPricedIn) / psCount
		summary.CredibilityScore = 1.0 - (float64(summary.CountManagementBluff) / psCount)
	} else {
		// No price-sensitive announcements - neutral scores
		summary.LeakScore = 0
		summary.CredibilityScore = 1.0
	}

	// Clamp credibility to [0, 1]
	summary.CredibilityScore = clamp(summary.CredibilityScore, 0, 1)
	summary.LeakScore = clamp(summary.LeakScore, 0, 1)
	summary.SignalRatio = clamp(summary.SignalRatio, 0, 1)
	summary.NoiseRatio = clamp(summary.NoiseRatio, 0, 1)

	// Calculate conviction score and communication style
	summary.ConvictionScore = CalculateConvictionScore(summary, classifications)
	summary.CommunicationStyle = DetermineCommunicationStyle(summary)

	return summary
}

// CalculateConvictionScore computes the conviction score (1-10).
//
// Scoring rules:
//   - Start at 5 (neutral)
//   - +1 for each TRUE_SIGNAL with volume_ratio > 3.0
//   - -1 for each PRICED_IN (potential leak)
//   - -1 for each MANAGEMENT_BLUFF (credibility hit)
//   - -0.5 for high noise_ratio (> 0.3)
//   - Result clamped to [1, 10]
func CalculateConvictionScore(summary SignalSummary, classifications []AnnouncementClassification) int {
	score := 5.0

	// +1 for high-conviction TRUE_SIGNALs
	for _, c := range classifications {
		if c.Classification == ClassificationTrueSignal && c.Metrics.VolumeRatio > 3.0 {
			score += 1.0
		}
	}

	// -1 for each PRICED_IN (potential information leakage)
	score -= float64(summary.CountPricedIn)

	// -1 for each MANAGEMENT_BLUFF (credibility penalty)
	score -= float64(summary.CountManagementBluff)

	// -0.5 for high noise ratio
	if summary.NoiseRatio > 0.3 {
		score -= 0.5
	}

	// Clamp to [1, 10]
	return int(math.Round(clamp(score, 1, 10)))
}

// DetermineCommunicationStyle classifies the company's disclosure style.
//
// Style determination (in order of precedence):
//   - LEAKY_INSIDER_RISK: leak_score > 0.3
//   - TRANSPARENT_DATA_DRIVEN: credibility > 0.8 AND leak_score < 0.1
//   - PROMOTIONAL_SENTIMENT: noise_ratio > 0.3 OR credibility < 0.5
//   - STANDARD: default
func DetermineCommunicationStyle(summary SignalSummary) string {
	// Check for leaky insider risk first (most severe)
	if summary.LeakScore > 0.3 {
		return StyleLeaky
	}

	// Check for transparent, data-driven style (best case)
	if summary.CredibilityScore > 0.8 && summary.LeakScore < 0.1 {
		return StyleTransparent
	}

	// Check for promotional sentiment (noisy or low credibility)
	if summary.NoiseRatio > 0.3 || summary.CredibilityScore < 0.5 {
		return StylePromotional
	}

	// Default to standard
	return StyleStandard
}

// DeriveRiskFlags populates risk flags from summary metrics.
func DeriveRiskFlags(summary SignalSummary) RiskFlags {
	return RiskFlags{
		HighLeakRisk:     summary.LeakScore > 0.3,
		SpeculativeBase:  summary.NoiseRatio > 0.3,
		ReliableSignals:  summary.SignalRatio > 0.2 && summary.CredibilityScore > 0.8,
		InsufficientData: summary.TotalAnnouncements < 5,
	}
}

// isRoutineCategory checks if the category is a routine administrative filing.
// These are standard regulatory filings that should not normally move markets.
func isRoutineCategory(category string) bool {
	upper := strings.ToUpper(category)

	routinePatterns := []string{
		"APPENDIX 3B", // New issue announcement
		"APPENDIX 3X", // Initial director interest
		"APPENDIX 3Y", // Change of director interest
		"APPENDIX 3Z", // Final director interest
		"DIRECTOR INTEREST",
		"COMPANY SECRETARY",
		"CHANGE OF ADDRESS",
		"CLEANSING STATEMENT",
		"DAILY SHARE BUY-BACK",
		"TRADING POLICY",
		"CHANGE OF AUDITOR",
	}

	for _, pattern := range routinePatterns {
		if strings.Contains(upper, pattern) {
			return true
		}
	}

	return false
}

// clamp restricts a value to the range [min, max]
func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
