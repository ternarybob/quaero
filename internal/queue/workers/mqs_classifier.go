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

// ClassifyRetention determines retention classification based on announcement day direction
// and 10-day follow-through. Returns the classification and an individual score (-1, 0, or +1).
//
// Classification rules:
//   - NEUTRAL: |day_of_change| < 1% -> score 0
//   - POSITIVE: day_of > 0 AND day_10 >= day_of * 0.5 (held/continued) -> score +1
//   - FADE: day_of > 0 AND day_10 < day_of * 0.5 (price faded) -> score -1
//   - OVER_REACTION: day_of < 0 AND day_10 >= day_of * 0.5 (recovered) -> score +1
//   - SUSTAINED_DROP: day_of < 0 AND day_10 < day_of * 0.5 (stayed down) -> score -1
func ClassifyRetention(dayOfChangePct, day10ChangePct float64) (RetentionClass, float64) {
	// NEUTRAL: Day-of change less than 1% - no significant move
	if math.Abs(dayOfChangePct) < 1.0 {
		return RetentionNeutral, 0.0
	}

	// Price ROSE on announcement day
	if dayOfChangePct > 0 {
		// Check if price held (at least 50% of the gain retained)
		if day10ChangePct >= dayOfChangePct*0.5 {
			// POSITIVE: Price rose and held/continued
			return RetentionPositive, 1.0
		}
		// FADE: Price rose but didn't hold
		return RetentionFade, -1.0
	}

	// Price FELL on announcement day (dayOfChangePct < 0)
	// Check if price recovered (day10 is less negative or positive)
	// Recovery means day_10 >= day_of * 0.5 (i.e., recovered at least half the drop)
	if day10ChangePct >= dayOfChangePct*0.5 {
		// OVER_REACTION: Market over-reacted, price recovered
		return RetentionOverReaction, 1.0
	}
	// SUSTAINED_DROP: Price fell and stayed down
	return RetentionSustainedDrop, -1.0
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
// All 5 components contribute equally (20% each) per prompt_2.md specification.
// Parameters: leakage (Information Integrity), conviction (Institutional Conviction),
// clarity (Clarity Index), efficiency (Communication Efficiency), retention (Value Sustainability)
func CalculateCompositeMQS(leakage, conviction, clarity, efficiency, retention float64) float64 {
	return (leakage * 0.20) + (conviction * 0.20) + (clarity * 0.20) + (efficiency * 0.20) + (retention * 0.20)
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

// SegmentBenchmark identifies benchmark indices for segment comparison
type SegmentBenchmark string

const (
	BenchmarkASXLargeCapEMA     SegmentBenchmark = "ASX_LARGE_CAP_EMA"     // ASX 20/50 EMA
	BenchmarkASXMidCapEMA       SegmentBenchmark = "ASX_MID_CAP_EMA"       // ASX 50-200 EMA
	BenchmarkASXSmallOrdsMedian SegmentBenchmark = "ASX_SMALL_ORDS_MEDIAN" // Small Ords Median
)

// GetSegmentBenchmark returns the appropriate benchmark index based on market cap
// Per prompt_3.md: Compare individual stock performance against its specific segment median
func GetSegmentBenchmark(marketCapUSD int64) SegmentBenchmark {
	const (
		largeCap = 10_000_000_000 // $10B+
		midCap   = 2_000_000_000  // $2B - $10B
	)

	if marketCapUSD > largeCap {
		return BenchmarkASXLargeCapEMA
	}
	if marketCapUSD > midCap {
		return BenchmarkASXMidCapEMA
	}
	return BenchmarkASXSmallOrdsMedian
}

// GetVolumeZScoreThreshold returns the appropriate Z-score threshold based on asset class
// Per prompt_3.md: Small-Cap typically requires Z > 2.0 to filter noise
func GetVolumeZScoreThreshold(assetClass AssetClass) float64 {
	switch assetClass {
	case AssetClassLargeCap:
		return 1.5 // Lower threshold for large caps (more liquid)
	case AssetClassMidCap:
		return 2.0 // Standard threshold
	default: // Small-Cap
		return 2.5 // Higher threshold to filter retail noise
	}
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
// Per prompt_2.md specification:
//   - HIGH_TRUST_LEADER: composite >= 0.70 (High integrity, low leakage, high efficiency)
//   - STABLE_STEWARD: composite >= 0.50 AND < 0.70 (Generally reliable, occasional issues)
//   - STRATEGIC_RISK: composite < 0.50 (Low efficiency, frequent leakage, poor resolution)
func DetermineMQSTier(composite, leakage, retention float64) MQSTier {
	if composite >= 0.70 {
		return TierHighTrustLeader
	}
	if composite >= 0.50 {
		return TierStableSteward
	}
	return TierStrategicRisk
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
