// -----------------------------------------------------------------------
// MQS Classifier - Management Quality Score Classification Logic
// Pure functions for classifying announcements and calculating MQS scores
// -----------------------------------------------------------------------

package workers

import (
	"math"
	"strings"
)

// ClassifyLeakage determines the leakage classification based on pre-announcement drift.
// Returns the classification and a score from 0.0 (poor) to 1.0 (excellent).
//
// Classification rules:
//   - HIGH_LEAKAGE: pre_drift > 3% in direction of announcement outcome -> 0.0-0.3
//   - TIGHT_SHIP: abs(pre_drift) < 1% AND pre_volume_ratio < 1.2 -> 0.7-1.0
//   - NEUTRAL: Everything else -> 0.4-0.6
func ClassifyLeakage(preDriftPct, preVolumeRatio, dayOfChangePct float64) (LeakageClass, float64) {
	absPreDrift := math.Abs(preDriftPct)

	// Check if pre-drift is aligned with outcome (same direction as day-of change)
	aligned := (preDriftPct > 0 && dayOfChangePct > 0) || (preDriftPct < 0 && dayOfChangePct < 0)

	// HIGH_LEAKAGE: pre_drift > 3% aligned with outcome
	if absPreDrift > 3.0 && aligned {
		// Score 0.0-0.3 based on severity (higher drift = lower score)
		score := math.Max(0.0, 0.3-(absPreDrift-3.0)*0.05)
		return LeakageHigh, score
	}

	// TIGHT_SHIP: minimal pre-drift AND minimal volume increase
	if absPreDrift < 1.0 && preVolumeRatio < 1.2 {
		// Score 0.7-1.0 based on tightness (lower drift = higher score)
		score := 0.7 + (1.0-absPreDrift)*0.3
		return LeakageTight, math.Min(1.0, score)
	}

	// NEUTRAL: everything else
	// Score 0.4-0.6 based on drift magnitude
	score := 0.6 - absPreDrift*0.05
	score = math.Max(0.4, math.Min(0.6, score))
	return LeakageNeutral, score
}

// ClassifyConviction determines conviction classification based on volume/price alignment.
// Returns the classification and a score from 0.0 (poor) to 1.0 (excellent).
//
// Classification rules:
//   - INSTITUTIONAL_CONVICTION: abs(day_change) > 2% AND volume_ratio > 3.0 -> 0.8-1.0
//   - RETAIL_HYPE: day_change > 5% AND volume_ratio < 2.0 -> 0.1-0.3
//   - LOW_INTEREST: abs(day_change) < 1% AND volume_ratio < 1.0 -> 0.4-0.6
//   - MIXED: Everything else -> 0.4-0.7
func ClassifyConviction(dayOfChangePct, volumeRatio float64) (ConvictionClass, float64) {
	absChange := math.Abs(dayOfChangePct)

	// INSTITUTIONAL_CONVICTION: High price change backed by high volume
	if absChange > 2.0 && volumeRatio > 3.0 {
		// Score 0.8-1.0 based on conviction strength
		score := 0.8 + math.Min(0.2, (volumeRatio-3.0)*0.04)
		return ConvictionInstitutional, math.Min(1.0, score)
	}

	// RETAIL_HYPE: High price change with LOW volume (no institutional backing)
	if dayOfChangePct > 5.0 && volumeRatio < 2.0 {
		// Score 0.1-0.3 based on lack of volume
		score := 0.1 + volumeRatio*0.1
		return ConvictionRetailHype, math.Min(0.3, score)
	}

	// LOW_INTEREST: Minimal movement all around
	if absChange < 1.0 && volumeRatio < 1.0 {
		// Score 0.4-0.6 (not necessarily bad, just not notable)
		score := 0.5
		return ConvictionLowInterest, score
	}

	// MIXED: Some reaction but not clearly institutional or hype
	// Score 0.4-0.7 based on volume support
	score := 0.4 + math.Min(0.3, volumeRatio*0.1)
	return ConvictionMixed, score
}

// ClassifyRetention determines retention classification based on price sustainability.
// Returns the classification and a score from 0.0 (poor) to 1.0 (excellent).
//
// retention_ratio (ρ) = day_10_change / day_of_change
// Classification rules:
//   - ABSORBED: 0.7 ≤ ρ ≤ 1.3 -> 0.7-0.9
//   - CONTINUED: ρ > 1.3 -> 0.9-1.0
//   - SOLD_NEWS: ρ < 0.5 -> 0.1-0.3
//   - REVERSED: ρ < 0 (opposite direction) -> 0.0-0.2
func ClassifyRetention(dayOfChangePct, day10ChangePct float64) (RetentionClass, float64) {
	// Handle zero day-of change (spec: set retention_ratio = 1.0)
	if math.Abs(dayOfChangePct) < 0.01 {
		return RetentionAbsorbed, 0.8 // No change to retain, neutral score
	}

	retentionRatio := day10ChangePct / dayOfChangePct

	// REVERSED: Price went opposite direction
	if retentionRatio < 0 {
		// Score 0.0-0.2 based on severity
		score := math.Max(0.0, 0.2+retentionRatio*0.1)
		return RetentionReversed, math.Max(0.0, score)
	}

	// SOLD_NEWS: Gained but then gave back most of it
	if retentionRatio < 0.5 {
		// Score 0.1-0.3
		score := 0.1 + retentionRatio*0.4
		return RetentionSoldNews, score
	}

	// ABSORBED: Maintained the move (0.7 ≤ ρ ≤ 1.3)
	if retentionRatio >= 0.7 && retentionRatio <= 1.3 {
		// Score 0.7-0.9
		score := 0.7 + math.Min(0.2, (retentionRatio-0.7)*0.33)
		return RetentionAbsorbed, score
	}

	// CONTINUED: Price continued in same direction beyond initial move
	if retentionRatio > 1.3 {
		// Score 0.9-1.0
		score := 0.9 + math.Min(0.1, (retentionRatio-1.3)*0.1)
		return RetentionContinued, math.Min(1.0, score)
	}

	// Fallback for 0.5 ≤ ρ < 0.7 - partial retention
	score := 0.3 + retentionRatio*0.5
	return RetentionSoldNews, math.Min(0.5, score)
}

// DetectTone analyzes announcement headline for language tone.
// Returns OPTIMISTIC, CONSERVATIVE, or DATA_DRY.
func DetectTone(headline string) ToneClass {
	upper := strings.ToUpper(headline)

	// Optimistic superlatives and promotional language
	optimisticPatterns := []string{
		"WORLD-CLASS", "WORLD CLASS", "WORLD LEADING",
		"GAME-CHANGING", "GAME CHANGING", "TRANSFORMATIONAL",
		"EXCEPTIONAL", "OUTSTANDING", "REMARKABLE",
		"UNPRECEDENTED", "RECORD-BREAKING", "RECORD BREAKING",
		"STRONG GROWTH", "STELLAR", "EXTRAORDINARY",
		"BREAKTHROUGH", "MAJOR DISCOVERY", "EXCITING",
		"REVOLUTIONARY", "PHENOMENAL", "BEST-IN-CLASS",
	}

	for _, pattern := range optimisticPatterns {
		if strings.Contains(upper, pattern) {
			return ToneOptimistic
		}
	}

	// Conservative/hedged language
	conservativePatterns := []string{
		"SUBJECT TO", "MAY ", "EXPECTS ", "EXPECTED TO",
		"POTENTIAL", "APPROXIMATELY", "ESTIMATED",
		"TARGETING", "GUIDANCE", "FORECAST",
	}

	conservativeCount := 0
	for _, pattern := range conservativePatterns {
		if strings.Contains(upper, pattern) {
			conservativeCount++
		}
	}

	if conservativeCount >= 2 {
		return ToneConservative
	}

	// Check for data-dry (primarily numbers and minimal adjectives)
	// Simple heuristic: if headline is short and contains numbers
	hasNumbers := strings.ContainsAny(headline, "0123456789")
	isShort := len(headline) < 50

	if hasNumbers && isShort && conservativeCount == 0 {
		return ToneDataDry
	}

	if conservativeCount == 1 {
		return ToneConservative
	}

	return ToneDataDry
}

// CalculateCompositeMQS calculates the weighted composite MQS score.
// Weights: leakage=0.33, conviction=0.33, retention=0.34
func CalculateCompositeMQS(leakage, conviction, retention float64) float64 {
	return (leakage * 0.33) + (conviction * 0.33) + (retention * 0.34)
}

// DetermineMQSTier determines the overall tier classification.
//
// Tier rules:
//   - TIER_1_OPERATOR: composite >= 0.75 AND leakage >= 0.7 AND retention >= 0.7
//   - TIER_2_HONEST_STRUGGLER: composite >= 0.50 AND leakage >= 0.6
//   - TIER_3_PROMOTER: composite < 0.50 OR leakage < 0.4 OR retention < 0.4
func DetermineMQSTier(composite, leakage, retention float64) MQSTier {
	// TIER_3_PROMOTER: Any disqualifying factor
	if composite < 0.50 || leakage < 0.4 || retention < 0.4 {
		return TierPromoter
	}

	// TIER_1_OPERATOR: All criteria met
	if composite >= 0.75 && leakage >= 0.7 && retention >= 0.7 {
		return TierOperator
	}

	// TIER_2_HONEST_STRUGGLER: Decent composite and clean disclosure
	if composite >= 0.50 && leakage >= 0.6 {
		return TierHonestStruggler
	}

	return TierPromoter
}

// DetermineConfidence determines confidence level based on announcement count.
func DetermineConfidence(announcementCount int) MQSConfidence {
	if announcementCount >= 20 {
		return ConfidenceHigh
	}
	if announcementCount >= 10 {
		return ConfidenceMedium
	}
	return ConfidenceLow
}
