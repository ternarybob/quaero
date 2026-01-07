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

// ClassifyAssetClass determines asset class based on market cap
// Large-Cap: > $10B, Mid-Cap: $2B-$10B, Small-Cap: < $2B
func ClassifyAssetClass(marketCapUSD int64) AssetClass {
	const (
		largeCap = 10_000_000_000 // $10B
		midCap   = 2_000_000_000  // $2B
	)

	if marketCapUSD >= largeCap {
		return AssetClassLargeCap
	}
	if marketCapUSD >= midCap {
		return AssetClassMidCap
	}
	return AssetClassSmallCap
}

// ClassifyEventMateriality determines if an announcement is Strategic or Routine
// Strategic: Earnings, Guidance, M&A, Clinical/Technical milestones (weight 1.0x)
// Routine: Buy-back updates, Admin notices, Appendix 3Ys (weight 0.2x)
func ClassifyEventMateriality(headline, category string) EventMateriality {
	upper := strings.ToUpper(headline)
	categoryUpper := strings.ToUpper(category)

	// Strategic patterns
	strategicPatterns := []string{
		// Earnings/Financial
		"RESULT", "EARNINGS", "PROFIT", "REVENUE", "FINANCIAL REPORT",
		"HALF YEAR", "FULL YEAR", "QUARTERLY", "ANNUAL",
		"APPENDIX 4D", "APPENDIX 4E", "4D", "4E",
		// Guidance
		"GUIDANCE", "FORECAST", "OUTLOOK", "TARGET", "UPGRADE", "DOWNGRADE",
		// M&A
		"ACQUISITION", "MERGER", "TAKEOVER", "DISPOSAL", "DIVESTMENT",
		"SALE OF", "PURCHASE OF",
		// Clinical/Technical milestones
		"CLINICAL TRIAL", "PHASE ", "FDA", "TGA", "APPROVAL",
		"PATENT", "DISCOVERY", "BREAKTHROUGH", "MILESTONE",
		"RESOURCE ESTIMATE", "RESERVE", "FEASIBILITY",
		// Capital events
		"CAPITAL RAISING", "PLACEMENT", "SPP", "RIGHTS ISSUE",
		"DIVIDEND",
	}

	for _, pattern := range strategicPatterns {
		if strings.Contains(upper, pattern) {
			return EventStrategic
		}
	}

	// Routine patterns (explicit check)
	routinePatterns := []string{
		"APPENDIX 3Y", "3Y", "CHANGE OF DIRECTOR",
		"BUY-BACK", "BUYBACK",
		"ADMINISTRATIVE", "CHANGE OF ADDRESS",
		"BECOMING A SUBSTANTIAL", "CEASING TO BE A SUBSTANTIAL",
		"DAILY SHARE BUY-BACK", "SHARE BUY BACK",
	}

	for _, pattern := range routinePatterns {
		if strings.Contains(upper, pattern) || strings.Contains(categoryUpper, pattern) {
			return EventRoutine
		}
	}

	// Default based on price sensitivity would be handled by caller
	// For now, default to Routine if not clearly Strategic
	return EventRoutine
}

// CalculateVolumeZScore calculates the volume Z-score for an announcement
// Z = (V - μ) / σ where μ and σ are 90-day rolling mean and std dev
func CalculateVolumeZScore(dayVolume int64, mean90Day, stdDev90Day float64) float64 {
	if stdDev90Day == 0 {
		return 0
	}
	return (float64(dayVolume) - mean90Day) / stdDev90Day
}

// IsConvictionTriggered determines if an event triggers conviction based on Z-score
// Trigger: Z > 2.0 (Large-Cap) or Z > 3.0 (Small/Mid-Cap) AND |Price Change| > 1.5%
func IsConvictionTriggered(zScore float64, priceChangePct float64, assetClass AssetClass) bool {
	absChange := math.Abs(priceChangePct)
	if absChange <= 1.5 {
		return false
	}

	switch assetClass {
	case AssetClassLargeCap:
		return zScore > 2.0
	default: // Mid-Cap and Small-Cap
		return zScore > 3.0
	}
}

// CalculateCAR calculates Cumulative Abnormal Return for pre-announcement period
// CAR = sum of daily abnormal returns over the lookback period
func CalculateCAR(dailyReturns []float64, marketReturns []float64) float64 {
	if len(dailyReturns) == 0 {
		return 0
	}

	car := 0.0
	for i := 0; i < len(dailyReturns); i++ {
		marketReturn := 0.0
		if i < len(marketReturns) {
			marketReturn = marketReturns[i]
		}
		// Abnormal return = actual return - expected (market) return
		abnormalReturn := dailyReturns[i] - marketReturn
		car += abnormalReturn
	}
	return car
}

// IsLeakage determines if CAR indicates information leakage
// Mark as 'Leakage' if |CAR| > 2σ where σ is 20-day rolling volatility
func IsLeakage(car, volatility20Day float64) bool {
	if volatility20Day == 0 {
		return false
	}
	return math.Abs(car) > 2*volatility20Day
}

// CalculateRetentionNew calculates price retention per new spec
// Retention = (Price_t+10 - Price_t-1) / (Price_t - Price_t-1)
func CalculateRetentionNew(priceT10, priceTMinus1, priceT float64) float64 {
	denominator := priceT - priceTMinus1
	if math.Abs(denominator) < 0.0001 {
		return 1.0 // No initial change, consider fully retained
	}
	return (priceT10 - priceTMinus1) / denominator
}

// DetermineMQSTier determines the overall tier classification.
//
// Tier rules:
//   - STITCHED_ALPHA: composite >= 0.75 AND leakage >= 0.7 AND retention >= 0.7
//   - STABLE_STEWARD: composite >= 0.50 AND leakage >= 0.6
//   - PROMOTER: leakage < 0.4 OR retention < 0.4
//   - WEAK_SIGNAL: composite < 0.30 OR insufficient data
func DetermineMQSTier(composite, leakage, retention float64) MQSTier {
	// WEAK_SIGNAL: Very low composite or poor data
	if composite < 0.30 {
		return TierWeakSignal
	}

	// PROMOTER: Any disqualifying factor (leakage or retention issues)
	if leakage < 0.4 || retention < 0.4 {
		return TierPromoter
	}

	// STITCHED_ALPHA: All criteria met (high quality)
	if composite >= 0.75 && leakage >= 0.7 && retention >= 0.7 {
		return TierStitchedAlpha
	}

	// STABLE_STEWARD: Decent composite and clean disclosure
	if composite >= 0.50 && leakage >= 0.6 {
		return TierStableSteward
	}

	// Default to PROMOTER if not meeting other criteria
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
