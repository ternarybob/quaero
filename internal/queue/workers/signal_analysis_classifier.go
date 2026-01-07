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

	// ClassificationSpeculativeHype indicates high initial reaction followed by reversal
	ClassificationSpeculativeHype = "SPECULATIVE_HYPE"

	// ClassificationRoutine indicates standard filing with normal market response
	ClassificationRoutine = "ROUTINE"
)

// Communication style constants
const (
	// StyleTransparent indicates data-driven, credible disclosure
	StyleTransparent = "TRANSPARENT_DATA_DRIVEN"

	// StylePromotional indicates noisy or low-credibility disclosure
	StylePromotional = "PROMOTIONAL_SENTIMENT"

	// StyleLeaky indicates significant pre-announcement price drift
	StyleLeaky = "HIGH_PRE_DRIFT"

	// StyleStandard indicates normal disclosure practices
	StyleStandard = "STANDARD"
)

// ClassifyAnnouncement determines the signal classification based on metrics and content.
//
// Decision matrix:
//   - TRUE_SIGNAL: volume_ratio > 2.0 AND abs(day_of_change) > 3% AND abs(pre_drift) < 2%
//   - PRICED_IN: abs(pre_drift) > 2% AND abs(day_of_change) < 1%
//   - SENTIMENT_NOISE: isRoutineCategory AND (volume_ratio > 1.5 OR abs(day_of_change) > 2%)
//   - MANAGEMENT_BLUFF: price_sensitive AND (volume_ratio < 0.5 OR promotional content)
//   - ROUTINE: default
func ClassifyAnnouncement(metrics ClassificationMetrics, priceSensitive bool, category string) string {
	return ClassifyAnnouncementWithContent(metrics, priceSensitive, category, "")
}

// ClassifyAnnouncementWithContent determines the signal classification based on metrics and headline content.
// The headline is analyzed for promotional language patterns that may indicate MANAGEMENT_BLUFF.
func ClassifyAnnouncementWithContent(metrics ClassificationMetrics, priceSensitive bool, category string, headline string) string {
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

	// SPECULATIVE_HYPE: High initial reaction followed by immediate reversal
	// Criteria: Day change > 3% AND PostDrift is opposite sign and reverses > 50% of the move
	if absChange > 3.0 && (metrics.PostDrift*metrics.DayOfChange < 0) && (math.Abs(metrics.PostDrift) > absChange*0.5) {
		return ClassificationSpeculativeHype
	}

	// MANAGEMENT_BLUFF: Marked as price-sensitive but no market impact
	// Dual detection criteria:
	// 1. Market-based: Low volume AND low price move despite price-sensitive flag
	// 2. Content-based: Promotional/exaggerated language without substance
	if priceSensitive {
		// Market-based detection: management claimed importance but market disagrees
		if metrics.VolumeRatio < 0.5 && absChange < 1.0 {
			return ClassificationManagementBluff
		}

		// Content-based detection: promotional headlines that didn't deliver
		// Only flag if there was weak market reaction (< 2% change)
		if headline != "" && absChange < 2.0 && hasPromotionalLanguage(headline) {
			return ClassificationManagementBluff
		}
	}

	// ROUTINE: Normal market response to normal announcement
	return ClassificationRoutine
}

// hasPromotionalLanguage detects promotional/exaggerated language in headlines.
// Returns true if headline contains patterns that suggest overstated importance.
func hasPromotionalLanguage(headline string) bool {
	upper := strings.ToUpper(headline)

	// Promotional superlatives and exaggerations
	promotionalPatterns := []string{
		"WORLD-CLASS", "WORLD CLASS", "WORLD LEADING",
		"GAME-CHANGING", "GAME CHANGING", "TRANSFORMATIONAL",
		"EXCEPTIONAL", "OUTSTANDING", "REMARKABLE",
		"UNPRECEDENTED", "RECORD-BREAKING", "RECORD BREAKING",
		"MASSIVE", "HUGE", "SIGNIFICANT MILESTONE",
		"BREAKTHROUGH", "MAJOR DISCOVERY", "EXCITING",
		"STELLAR", "EXTRAORDINARY", "PHENOMENAL",
	}

	for _, pattern := range promotionalPatterns {
		if strings.Contains(upper, pattern) {
			return true
		}
	}

	// Vague positive claims without specifics
	vaguePositivePatterns := []string{
		"POSITIVE PROGRESS", "EXCELLENT PROGRESS",
		"STRONG RESULTS", "OUTSTANDING RESULTS",
		"CONFIRMS STRATEGY", "VALIDATES APPROACH",
		"ON TRACK", "WELL POSITIONED",
	}

	// Count vague positive patterns - flag if multiple present
	vagueCount := 0
	for _, pattern := range vaguePositivePatterns {
		if strings.Contains(upper, pattern) {
			vagueCount++
		}
	}
	if vagueCount >= 2 {
		return true
	}

	return false
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
		case ClassificationSpeculativeHype:
			summary.CountSentimentNoise++ // Count as noise for ratios, but track separately if needed
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
// Net Signal-based scoring:
//   - Start at 5 (neutral)
//   - Calculate NET_SIGNAL = TRUE_SIGNAL - PRICED_IN
//   - +0.8 per NET_SIGNAL (capped at +3/-3)
//   - +0.5 for high credibility (> 0.8) AND low leak score (< 0.2)
//   - -0.8 for each MANAGEMENT_BLUFF (capped at -2)
//   - -0.5 for high noise ratio (> 0.3) if no true signals
//   - Result clamped to [1, 10]
//
// This net-signal approach ensures:
//   - TRUE_SIGNAL=5, PRICED_IN=6 → NET=-1 → Score ~4 (NEUTRAL)
//   - TRUE_SIGNAL=8, PRICED_IN=2 → NET=+6 → Score ~8 (HIGH)
//   - TRUE_SIGNAL=2, PRICED_IN=8 → NET=-6 → Score ~2 (LOW)
func CalculateConvictionScore(summary SignalSummary, classifications []AnnouncementClassification) int {
	score := 5.0

	// Calculate NET_SIGNAL = TRUE_SIGNAL - PRICED_IN
	// This is the core signal quality metric
	netSignal := summary.CountTrueSignal - summary.CountPricedIn

	// +0.8 per net signal point (capped at +3/-3)
	// Positive net = more true signals than leaks = higher conviction
	// Negative net = more leaks than true signals = lower conviction
	netBonus := math.Max(math.Min(float64(netSignal)*0.8, 3.0), -3.0)
	score += netBonus

	// +0.5 for high credibility AND low leak score (combined quality indicator)
	// Only award if management is both accurate AND information is not leaking
	if summary.CredibilityScore > 0.8 && summary.LeakScore < 0.2 {
		score += 0.5
	}

	// -0.8 for each MANAGEMENT_BLUFF (capped at -2.0)
	// Management marked as price-sensitive but market didn't react
	// This indicates poor communication quality or overstated importance
	bluffPenalty := math.Min(float64(summary.CountManagementBluff)*0.8, 2.0)
	score -= bluffPenalty

	// -0.5 for high noise ratio ONLY if no true signals exist
	// Stocks generating noise without substance
	if summary.NoiseRatio > 0.3 && summary.CountTrueSignal == 0 {
		score -= 0.5
	}

	// Clamp to [1, 10]
	return int(math.Round(clamp(score, 1, 10)))
}

// DetermineCommunicationStyle classifies the company's disclosure style.
//
// Style determination (in order of precedence):
//   - HIGH_PRE_DRIFT: leak_score > 0.3 (significant pre-announcement price movement)
//   - TRANSPARENT_DATA_DRIVEN: credibility > 0.8 AND leak_score < 0.1
//   - PROMOTIONAL_SENTIMENT: noise_ratio > 0.3 OR credibility < 0.5
//   - STANDARD: default
func DetermineCommunicationStyle(summary SignalSummary) string {
	// Check for high pre-announcement drift first (most severe)
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
