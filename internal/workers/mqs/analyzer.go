// -----------------------------------------------------------------------
// MQS Analyzer - Management Quality Score Analysis Functions
// Calculates MQS metrics from announcements and price data
// -----------------------------------------------------------------------

package mqs

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/ternarybob/quaero/internal/workers/market"
)

// MQSAnalyzer calculates Management Quality Scores from announcement and price data
type MQSAnalyzer struct {
	announcements []market.ASXAnnouncement
	prices        []market.OHLCV
	priceMap      map[string]market.OHLCV // date string -> OHLCV
	ticker        string
	exchange      string
	fundamentals  *market.FundamentalsFinancialData // EODHD financial data (optional)
	newsItems     []EODHDNewsItem            // EODHD news for matching (optional)
	marketCap     int64                      // Market capitalization
	sector        string                     // Industry sector
	assetClass    AssetClass                 // Asset class classification
}

// NewMQSAnalyzer creates a new MQS analyzer
func NewMQSAnalyzer(announcements []market.ASXAnnouncement, prices []market.OHLCV, ticker, exchange string) *MQSAnalyzer {
	// Build price map for O(1) lookups
	priceMap := make(map[string]market.OHLCV)
	for _, p := range prices {
		priceMap[p.Date.Format("2006-01-02")] = p
	}

	return &MQSAnalyzer{
		announcements: announcements,
		prices:        prices,
		priceMap:      priceMap,
		ticker:        ticker,
		exchange:      exchange,
	}
}

// SetFundamentals sets the EODHD fundamentals data for enriching financial results
func (a *MQSAnalyzer) SetFundamentals(data *market.FundamentalsFinancialData) {
	a.fundamentals = data
	if data != nil {
		a.marketCap = data.MarketCap
		a.sector = data.Sector
		a.assetClass = ClassifyAssetClass(data.MarketCap)
	}
}

// SetNews sets the EODHD news items for matching with high-impact announcements
func (a *MQSAnalyzer) SetNews(news []EODHDNewsItem) {
	a.newsItems = news
}

// filterAnnouncementsToPeriod filters announcements to the specified date range
func (a *MQSAnalyzer) filterAnnouncementsToPeriod(start, end time.Time) {
	filtered := make([]market.ASXAnnouncement, 0, len(a.announcements))
	for _, ann := range a.announcements {
		if !ann.Date.Before(start) && !ann.Date.After(end) {
			filtered = append(filtered, ann)
		}
	}
	a.announcements = filtered
}

// Analyze performs the full MQS analysis and returns the output
func (a *MQSAnalyzer) Analyze() *MQSOutput {
	now := time.Now()

	// Fixed 36-month lookback period per prompt_2.md
	periodEnd := now
	periodStart := now.AddDate(-3, 0, 0) // 36 months ago

	// Filter announcements to the analysis period
	a.filterAnnouncementsToPeriod(periodStart, periodEnd)

	// Count total announcements after filtering
	totalAnnouncements := len(a.announcements)

	// Count price-sensitive announcements
	priceSensitiveCount := 0
	for _, ann := range a.announcements {
		if ann.PriceSensitive {
			priceSensitiveCount++
		}
	}

	output := &MQSOutput{
		Ticker:                      fmt.Sprintf("%s.%s", a.exchange, a.ticker),
		Exchange:                    a.exchange,
		AnalysisDate:                now,
		PeriodStart:                 periodStart,
		PeriodEnd:                   periodEnd,
		TotalAnnouncements:          totalAnnouncements,
		PriceSensitiveAnnouncements: priceSensitiveCount,
		Meta: MQSMeta{
			AssetClass: a.assetClass,
			Sector:     a.sector,
			MarketCap:  a.marketCap,
		},
	}

	// Analyze each announcement
	mqsAnnouncements := a.analyzeAnnouncements()
	output.Announcements = mqsAnnouncements

	// Build detailed events with new analysis fields
	output.DetailedEvents = a.buildDetailedEvents(mqsAnnouncements)

	// Calculate component summaries (5 measures, 20% each)
	output.LeakageSummary = a.calculateLeakageSummary(mqsAnnouncements)
	output.ConvictionSummary = a.calculateConvictionSummary(mqsAnnouncements)
	output.ClaritySummary = a.calculateClaritySummary(mqsAnnouncements)
	output.EfficiencySummary = a.calculateEfficiencySummary(mqsAnnouncements)
	output.RetentionSummary = a.calculateRetentionSummary(mqsAnnouncements)

	// Calculate aggregate scores (all 5 measures at 20% each per prompt_2.md)
	output.ManagementQualityScore = a.calculateAggregateScore(
		output.LeakageSummary,
		output.ConvictionSummary,
		output.ClaritySummary,
		output.EfficiencySummary,
		output.RetentionSummary,
		len(mqsAnnouncements),
	)

	// Pattern analysis
	output.Patterns = a.detectPatterns(mqsAnnouncements)

	// Build high-impact announcements list (past 12 months with significant price/volume changes)
	output.HighImpactAnnouncements = a.buildHighImpactAnnouncements(mqsAnnouncements)

	// Data quality
	output.DataQuality = DataQualityInfo{
		AnnouncementsCount: len(a.announcements),
		TradingDaysCount:   len(a.prices),
		DataGaps:           a.detectDataGaps(),
		GeneratedAt:        now,
	}

	return output
}

// analyzeAnnouncements analyzes each announcement and returns MQS data
func (a *MQSAnalyzer) analyzeAnnouncements() []MQSAnnouncement {
	var results []MQSAnnouncement

	for _, ann := range a.announcements {
		mqsAnn := a.analyzeSingleAnnouncement(ann)
		if mqsAnn != nil {
			results = append(results, *mqsAnn)
		}
	}

	return results
}

// analyzeSingleAnnouncement calculates MQS metrics for one announcement
func (a *MQSAnalyzer) analyzeSingleAnnouncement(ann market.ASXAnnouncement) *MQSAnnouncement {
	// Get market data around announcement
	leadIn := a.calculateLeadIn(ann.Date)
	dayOf := a.calculateDayOf(ann.Date)
	leadOut := a.calculateLeadOut(ann.Date)

	// Skip if no day-of data
	if dayOf == nil {
		return nil
	}

	// Calculate 30-day MA volume for ratios
	maVolume := a.calculate30DayMAVolume(ann.Date)

	// Calculate pre-volume ratio
	preVolumeRatio := 1.0
	if maVolume > 0 && leadIn != nil {
		preVolumeRatio = leadIn.VolumeRatio
	}

	// Classify leakage - dayOf is guaranteed non-nil at this point
	dayOfChange := dayOf.PriceChangePct

	// Get leadIn price change (default 0 if no lead-in data)
	leadInPriceChange := 0.0
	if leadIn != nil {
		leadInPriceChange = leadIn.PriceChangePct
	}
	leakageClass, leakageScore := ClassifyLeakage(
		leadInPriceChange,
		preVolumeRatio,
		dayOfChange,
	)

	// Classify conviction
	volumeRatio := 1.0
	if maVolume > 0 && dayOf.Volume > 0 {
		volumeRatio = float64(dayOf.Volume) / float64(maVolume)
	}
	convictionClass, convictionScore := ClassifyConviction(dayOfChange, volumeRatio)

	// Classify retention
	day10Change := 0.0
	if leadOut != nil {
		day10Change = leadOut.PriceChangePct
	}
	retentionClass, retentionScore := ClassifyRetention(dayOfChange, day10Change)

	result := &MQSAnnouncement{
		Date:            ann.Date.Format("2006-01-02"),
		Headline:        ann.Headline,
		Category:        ann.Type,
		PriceSensitive:  ann.PriceSensitive,
		LeakageClass:    leakageClass,
		ConvictionClass: convictionClass,
		RetentionClass:  retentionClass,
		LeakageScore:    leakageScore,
		ConvictionScore: convictionScore,
		RetentionScore:  retentionScore,
	}

	if leadIn != nil {
		result.LeadIn = *leadIn
	}
	// dayOf is guaranteed non-nil
	result.DayOf = *dayOf
	if leadOut != nil {
		result.LeadOut = *leadOut
	}

	return result
}

// calculateLeadIn calculates the 5 trading days before announcement
func (a *MQSAnalyzer) calculateLeadIn(annDate time.Time) *LeadMetrics {
	// Find T-5 and T-1 prices
	var priceT5, priceT1 market.OHLCV
	var foundT5, foundT1 bool
	tradingDays := 0
	totalVolume := int64(0)

	// Look back to find 5 trading days
	for i := 1; i <= 15 && tradingDays < 5; i++ {
		checkDate := annDate.AddDate(0, 0, -i).Format("2006-01-02")
		if p, ok := a.priceMap[checkDate]; ok {
			tradingDays++
			totalVolume += p.Volume
			if !foundT1 {
				priceT1 = p
				foundT1 = true
			}
			if tradingDays == 5 {
				priceT5 = p
				foundT5 = true
			}
		}
	}

	if !foundT5 || !foundT1 {
		return nil
	}

	priceChange := 0.0
	if priceT5.Close > 0 {
		priceChange = ((priceT1.Close - priceT5.Close) / priceT5.Close) * 100
	}

	// Calculate volume ratio vs 30-day MA
	maVolume := a.calculate30DayMAVolume(annDate)
	avgVolume := float64(totalVolume) / float64(tradingDays)
	volumeRatio := 1.0
	if maVolume > 0 {
		volumeRatio = avgVolume / float64(maVolume)
	}

	return &LeadMetrics{
		PriceChangePct: priceChange,
		VolumeRatio:    volumeRatio,
		TradingDays:    tradingDays,
		StartPrice:     priceT5.Close,
		EndPrice:       priceT1.Close,
	}
}

// calculateDayOf calculates the announcement day metrics
func (a *MQSAnalyzer) calculateDayOf(annDate time.Time) *DayOfMetrics {
	annDateStr := annDate.Format("2006-01-02")

	// Find price on announcement date (or next trading day)
	var dayPrice market.OHLCV
	found := false

	if p, ok := a.priceMap[annDateStr]; ok {
		dayPrice = p
		found = true
	} else {
		// Check next 5 days for weekend/holiday
		for i := 1; i <= 5; i++ {
			checkDate := annDate.AddDate(0, 0, i).Format("2006-01-02")
			if p, ok := a.priceMap[checkDate]; ok {
				dayPrice = p
				found = true
				break
			}
		}
	}

	if !found {
		return nil
	}

	// Find previous day close for change calculation
	var prevClose float64
	for i := 1; i <= 10; i++ {
		checkDate := annDate.AddDate(0, 0, -i).Format("2006-01-02")
		if p, ok := a.priceMap[checkDate]; ok {
			prevClose = p.Close
			break
		}
	}

	priceChange := 0.0
	if prevClose > 0 {
		priceChange = ((dayPrice.Close - prevClose) / prevClose) * 100
	}

	// Calculate volume ratio vs 30-day MA
	maVolume := a.calculate30DayMAVolume(annDate)
	volumeRatio := 1.0
	if maVolume > 0 {
		volumeRatio = float64(dayPrice.Volume) / float64(maVolume)
	}

	// Calculate VWAP-based conviction metrics per prompt_3.md
	meanVol, stdDevVol := a.calculate20DayVolumeStats(annDate)
	volumeZScore := 0.0
	if stdDevVol > 0 {
		volumeZScore = (float64(dayPrice.Volume) - meanVol) / stdDevVol
	}

	vwap20 := a.calculate20DayVWAP(annDate)
	priceVsVWAP := 0.0
	if vwap20 > 0 {
		priceVsVWAP = (dayPrice.Close - vwap20) / vwap20
	}

	// Combined conviction score: (z_score * 0.6) + (price_vs_vwap * 0.4)
	convictionScore := (volumeZScore * 0.6) + (priceVsVWAP * 0.4)
	if convictionScore > 1.0 {
		convictionScore = 1.0
	}
	if convictionScore < -1.0 {
		convictionScore = -1.0
	}

	return &DayOfMetrics{
		Open:            dayPrice.Open,
		High:            dayPrice.High,
		Low:             dayPrice.Low,
		Close:           dayPrice.Close,
		Volume:          dayPrice.Volume,
		PriceChangePct:  priceChange,
		VolumeRatio:     volumeRatio,
		VolumeZScore:    volumeZScore,
		VWAP20:          vwap20,
		PriceVsVWAP:     priceVsVWAP,
		ConvictionScore: convictionScore,
	}
}

// calculateLeadOut calculates the 10 trading days after announcement
func (a *MQSAnalyzer) calculateLeadOut(annDate time.Time) *LeadMetrics {
	// Find T+1 and T+10 prices
	var priceT1, priceT10 market.OHLCV
	var foundT1, foundT10 bool
	tradingDays := 0
	totalVolume := int64(0)

	// Look forward to find 10 trading days
	for i := 1; i <= 20 && tradingDays < 10; i++ {
		checkDate := annDate.AddDate(0, 0, i).Format("2006-01-02")
		if p, ok := a.priceMap[checkDate]; ok {
			tradingDays++
			totalVolume += p.Volume
			if !foundT1 {
				priceT1 = p
				foundT1 = true
			}
			if tradingDays == 10 {
				priceT10 = p
				foundT10 = true
			}
		}
	}

	if !foundT10 || !foundT1 {
		return nil
	}

	// Get pre-announcement price (T-1)
	var prePrice float64
	for i := 1; i <= 10; i++ {
		checkDate := annDate.AddDate(0, 0, -i).Format("2006-01-02")
		if p, ok := a.priceMap[checkDate]; ok {
			prePrice = p.Close
			break
		}
	}

	priceChange := 0.0
	if prePrice > 0 {
		priceChange = ((priceT10.Close - prePrice) / prePrice) * 100
	}

	// Calculate volume ratio vs 30-day MA
	maVolume := a.calculate30DayMAVolume(annDate)
	avgVolume := float64(totalVolume) / float64(tradingDays)
	volumeRatio := 1.0
	if maVolume > 0 {
		volumeRatio = avgVolume / float64(maVolume)
	}

	return &LeadMetrics{
		PriceChangePct: priceChange,
		VolumeRatio:    volumeRatio,
		TradingDays:    tradingDays,
		StartPrice:     priceT1.Close,
		EndPrice:       priceT10.Close,
	}
}

// calculate30DayMAVolume calculates the 30-day moving average volume
func (a *MQSAnalyzer) calculate30DayMAVolume(refDate time.Time) int64 {
	totalVolume := int64(0)
	count := 0

	// Look back up to 45 calendar days to find 30 trading days
	for i := 1; i <= 45 && count < 30; i++ {
		checkDate := refDate.AddDate(0, 0, -i).Format("2006-01-02")
		if p, ok := a.priceMap[checkDate]; ok && p.Volume > 0 {
			totalVolume += p.Volume
			count++
		}
	}

	if count == 0 {
		return 0
	}
	return totalVolume / int64(count)
}

// calculate20DayVWAP calculates the 20-day Volume Weighted Average Price
// VWAP = Σ(TypicalPrice × Volume) / Σ(Volume)
// where TypicalPrice = (High + Low + Close) / 3
func (a *MQSAnalyzer) calculate20DayVWAP(refDate time.Time) float64 {
	var sumPV float64
	var sumV float64
	count := 0

	// Look back up to 30 calendar days to find 20 trading days
	for i := 1; i <= 30 && count < 20; i++ {
		checkDate := refDate.AddDate(0, 0, -i).Format("2006-01-02")
		if p, ok := a.priceMap[checkDate]; ok && p.Volume > 0 {
			typicalPrice := (p.High + p.Low + p.Close) / 3
			sumPV += typicalPrice * float64(p.Volume)
			sumV += float64(p.Volume)
			count++
		}
	}

	if sumV == 0 {
		return 0
	}
	return sumPV / sumV
}

// calculate20DayVolumeStats calculates 20-day volume mean and standard deviation for Z-score
func (a *MQSAnalyzer) calculate20DayVolumeStats(refDate time.Time) (mean float64, stdDev float64) {
	volumes := make([]float64, 0, 20)
	count := 0

	// Look back up to 30 calendar days to find 20 trading days
	for i := 1; i <= 30 && count < 20; i++ {
		checkDate := refDate.AddDate(0, 0, -i).Format("2006-01-02")
		if p, ok := a.priceMap[checkDate]; ok && p.Volume > 0 {
			volumes = append(volumes, float64(p.Volume))
			count++
		}
	}

	if len(volumes) == 0 {
		return 0, 0
	}

	// Calculate mean
	sum := 0.0
	for _, v := range volumes {
		sum += v
	}
	mean = sum / float64(len(volumes))

	// Calculate standard deviation
	if len(volumes) > 1 {
		sumSq := 0.0
		for _, v := range volumes {
			sumSq += (v - mean) * (v - mean)
		}
		stdDev = math.Sqrt(sumSq / float64(len(volumes)-1))
	}

	return mean, stdDev
}

// deduplicateByDate returns one announcement per date (the most significant one by price change)
// Same-day announcements should be counted as 1 for leakage analysis since we can't
// attribute the pre-drift to individual announcements without reading content
func deduplicateByDate(announcements []MQSAnnouncement) []MQSAnnouncement {
	// Group by date, keeping the most significant announcement (highest price change)
	byDate := make(map[string]*MQSAnnouncement)

	for i := range announcements {
		ann := &announcements[i]
		if existing, ok := byDate[ann.Date]; ok {
			// Keep the one with higher absolute price change
			if math.Abs(ann.DayOf.PriceChangePct) > math.Abs(existing.DayOf.PriceChangePct) {
				byDate[ann.Date] = ann
			}
		} else {
			byDate[ann.Date] = ann
		}
	}

	// Convert back to slice
	result := make([]MQSAnnouncement, 0, len(byDate))
	for _, ann := range byDate {
		result = append(result, *ann)
	}

	// Sort by date descending (newest first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Date > result[j].Date
	})

	return result
}

// filterPriceSensitive returns only price-sensitive announcements
func filterPriceSensitive(announcements []MQSAnnouncement) []MQSAnnouncement {
	var result []MQSAnnouncement
	for _, ann := range announcements {
		if ann.PriceSensitive {
			result = append(result, ann)
		}
	}
	return result
}

// filterSignificantMoves returns announcements with significant price/volume movement
// Uses market cap-based thresholds from assetClass field
func (a *MQSAnalyzer) filterSignificantMoves(announcements []MQSAnnouncement) []MQSAnnouncement {
	// Determine threshold based on asset class
	priceThreshold := 3.0  // Default for small-cap
	volumeThreshold := 2.0 // Volume ratio threshold

	switch a.assetClass {
	case AssetClassLargeCap:
		priceThreshold = 1.5 // Large-caps move less but moves are more significant
	case AssetClassMidCap:
		priceThreshold = 2.0
	case AssetClassSmallCap:
		priceThreshold = 3.0
	}

	var result []MQSAnnouncement
	for _, ann := range announcements {
		// Include if price-sensitive OR has significant price OR volume movement
		hasSignificantPrice := math.Abs(ann.DayOf.PriceChangePct) >= priceThreshold
		hasSignificantVolume := ann.DayOf.VolumeRatio >= volumeThreshold

		if ann.PriceSensitive || hasSignificantPrice || hasSignificantVolume {
			result = append(result, ann)
		}
	}
	return result
}

// calculateLeakageSummary aggregates leakage metrics across price-sensitive announcements
// Deduplicates by date and focuses on price-sensitive announcements
func (a *MQSAnalyzer) calculateLeakageSummary(announcements []MQSAnnouncement) LeakageSummary {
	// Focus on price-sensitive announcements and deduplicate by date
	filtered := filterPriceSensitive(announcements)
	filtered = deduplicateByDate(filtered)

	summary := LeakageSummary{
		TotalAnalyzed: len(filtered),
		WorstLeakages: []LeakageIncident{},
	}

	var totalPreDrift float64
	var leakages []LeakageIncident

	// Use filtered announcements (price-sensitive, deduplicated by date)
	for _, ann := range filtered {
		switch ann.LeakageClass {
		case LeakageHigh:
			summary.HighLeakageCount++
			leakages = append(leakages, LeakageIncident{
				Date:           ann.Date,
				Headline:       ann.Headline,
				PreDriftPct:    ann.LeadIn.PriceChangePct,
				PriceSensitive: ann.PriceSensitive,
			})
		case LeakageTight:
			summary.TightShipCount++
		case LeakageNeutral:
			summary.NeutralCount++
		}
		totalPreDrift += math.Abs(ann.LeadIn.PriceChangePct)
	}

	if len(filtered) > 0 {
		summary.AveragePreDriftPct = totalPreDrift / float64(len(filtered))
		summary.LeakageRatio = float64(summary.HighLeakageCount) / float64(len(filtered))
	}

	// Sort leakages by magnitude and take top 5
	sort.Slice(leakages, func(i, j int) bool {
		return math.Abs(leakages[i].PreDriftPct) > math.Abs(leakages[j].PreDriftPct)
	})
	if len(leakages) > 5 {
		leakages = leakages[:5]
	}
	summary.WorstLeakages = leakages

	return summary
}

// calculateConvictionSummary aggregates conviction metrics
// Updated per prompt_3.md to use VWAP-based institutional conviction scoring
func (a *MQSAnalyzer) calculateConvictionSummary(announcements []MQSAnnouncement) ConvictionSummary {
	summary := ConvictionSummary{
		TotalAnalyzed:        len(announcements),
		HighConvictionEvents: []ConvictionEvent{},
	}

	var totalVolumeRatio float64
	var totalVolumeZScore float64
	var totalPriceVsVWAP float64
	var totalConvictionScore float64
	var highConviction []ConvictionEvent
	highVolAboveVWAPCount := 0

	for _, ann := range announcements {
		totalVolumeRatio += ann.DayOf.VolumeRatio
		totalVolumeZScore += ann.DayOf.VolumeZScore
		totalPriceVsVWAP += ann.DayOf.PriceVsVWAP
		totalConvictionScore += ann.DayOf.ConvictionScore

		// Track events where price is above VWAP (institutional accumulation signal)
		if ann.DayOf.PriceVsVWAP > 0 {
			summary.AboveVWAPCount++

			// High volume + above VWAP = strong institutional signal
			if ann.DayOf.VolumeZScore > 1.0 {
				highVolAboveVWAPCount++
			}
		}

		switch ann.ConvictionClass {
		case ConvictionInstitutional:
			summary.InstitutionalCount++
			highConviction = append(highConviction, ConvictionEvent{
				Date:            ann.Date,
				Headline:        ann.Headline,
				PriceChange:     ann.DayOf.PriceChangePct,
				VolumeRatio:     ann.DayOf.VolumeRatio,
				VolumeZScore:    ann.DayOf.VolumeZScore,
				PriceVsVWAP:     ann.DayOf.PriceVsVWAP,
				ConvictionScore: ann.DayOf.ConvictionScore,
				Class:           string(ann.ConvictionClass),
			})
		case ConvictionRetailHype:
			summary.RetailHypeCount++
		case ConvictionLowInterest:
			summary.LowInterestCount++
		case ConvictionMixed:
			summary.MixedCount++
		}
	}

	if len(announcements) > 0 {
		n := float64(len(announcements))
		summary.AverageVolumeRatio = totalVolumeRatio / n
		summary.AverageVolumeZScore = totalVolumeZScore / n
		summary.AveragePriceVsVWAP = totalPriceVsVWAP / n
		summary.AverageConvictionScore = totalConvictionScore / n
		summary.InstitutionalRatio = float64(summary.InstitutionalCount) / n
		summary.HighVolumeAboveVWAPRatio = float64(highVolAboveVWAPCount) / n
	}

	// Sort by conviction score and take top 5
	sort.Slice(highConviction, func(i, j int) bool {
		return highConviction[i].ConvictionScore > highConviction[j].ConvictionScore
	})
	if len(highConviction) > 5 {
		highConviction = highConviction[:5]
	}
	summary.HighConvictionEvents = highConviction

	return summary
}

// calculateClaritySummary calculates volatility resolution metrics
// Formula: Clarity Index = σ_pre / σ_post (15 days before vs 15 days after, excluding Day 0)
// Score > 1.0 indicates management successfully lowered stock's risk profile
func (a *MQSAnalyzer) calculateClaritySummary(announcements []MQSAnnouncement) ClaritySummary {
	// Focus on price-sensitive announcements, deduplicate by date
	filtered := filterPriceSensitive(announcements)
	filtered = deduplicateByDate(filtered)

	summary := ClaritySummary{
		TotalAnalyzed:     len(filtered),
		HighClarityEvents: []ClarityEvent{},
	}

	if len(filtered) == 0 {
		return summary
	}

	var totalPreVol, totalPostVol, totalIndex float64
	var validCount int
	var events []ClarityEvent

	for _, ann := range filtered {
		// Calculate 15-day pre-announcement volatility
		preVol := a.calculate15DayVolatility(ann.Date, true) // before
		// Calculate 15-day post-announcement volatility
		postVol := a.calculate15DayVolatility(ann.Date, false) // after

		if preVol <= 0 || postVol <= 0 {
			continue // Skip if we can't calculate volatility
		}

		clarityIndex := preVol / postVol
		validCount++
		totalPreVol += preVol
		totalPostVol += postVol
		totalIndex += clarityIndex

		if clarityIndex > 1.0 {
			summary.VolatilityReduced++
		} else {
			summary.VolatilityIncreased++
		}

		events = append(events, ClarityEvent{
			Date:           ann.Date,
			Headline:       ann.Headline,
			PreVolatility:  preVol,
			PostVolatility: postVol,
			ClarityIndex:   clarityIndex,
		})
	}

	if validCount > 0 {
		summary.AveragePreVolatility = totalPreVol / float64(validCount)
		summary.AveragePostVolatility = totalPostVol / float64(validCount)
		summary.AverageClarityIndex = totalIndex / float64(validCount)
		// Normalize score: ratio of events that reduced volatility
		summary.ClarityScore = float64(summary.VolatilityReduced) / float64(validCount)
	}

	// Sort by clarity index descending and take top 5
	sort.Slice(events, func(i, j int) bool {
		return events[i].ClarityIndex > events[j].ClarityIndex
	})
	if len(events) > 5 {
		events = events[:5]
	}
	summary.HighClarityEvents = events

	return summary
}

// calculate15DayVolatility calculates 15-day return volatility before or after an announcement date
func (a *MQSAnalyzer) calculate15DayVolatility(dateStr string, before bool) float64 {
	_, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return 0
	}

	// Find the announcement day index in prices
	annIdx := -1
	for i, p := range a.prices {
		if p.Date.Format("2006-01-02") == dateStr {
			annIdx = i
			break
		}
	}

	if annIdx < 0 {
		return 0
	}

	var returns []float64
	if before {
		// Get 15 days before (excluding Day 0)
		// Prices are sorted newest first, so before means higher indices
		for i := annIdx + 1; i <= annIdx+16 && i < len(a.prices)-1; i++ {
			if a.prices[i+1].Close > 0 {
				ret := (a.prices[i].Close - a.prices[i+1].Close) / a.prices[i+1].Close
				returns = append(returns, ret)
			}
		}
	} else {
		// Get 15 days after (excluding Day 0)
		// Prices are sorted newest first, so after means lower indices
		for i := annIdx - 1; i >= annIdx-15 && i > 0; i-- {
			if a.prices[i+1].Close > 0 {
				ret := (a.prices[i].Close - a.prices[i+1].Close) / a.prices[i+1].Close
				returns = append(returns, ret)
			}
		}
	}

	if len(returns) < 5 {
		return 0 // Not enough data
	}

	// Calculate standard deviation of returns
	return calculateStdDev(returns)
}

// calculateEfficiencySummary calculates communication efficiency (Signal-to-Churn) metrics
// Per prompt_3.md:
//   - signal_to_churn = price_delta / (vol_at_event / market_avg_vol)
//   - Low delta with high volume = 'Churn' (Uncertainty)
//   - High delta with high volume = 'Pure Signal'
//
// Interpretation:
//   - High efficiency = market trusts immediately (price moves efficiently)
//   - Low efficiency = high volume churn for modest move = lack of credibility
func (a *MQSAnalyzer) calculateEfficiencySummary(announcements []MQSAnnouncement) EfficiencySummary {
	// Focus on price-sensitive announcements, deduplicate by date
	filtered := filterPriceSensitive(announcements)
	filtered = deduplicateByDate(filtered)

	summary := EfficiencySummary{
		TotalAnalyzed:        len(filtered),
		HighEfficiencyEvents: []EfficiencyEvent{},
	}

	if len(filtered) == 0 {
		return summary
	}

	var totalEfficiency float64
	var validCount int
	var events []EfficiencyEvent

	for _, ann := range filtered {
		// Per prompt_3.md: signal_to_churn = price_delta / (vol_at_event / market_avg_vol)
		// Which simplifies to: |price_delta| / volume_ratio
		volumeRatio := ann.DayOf.VolumeRatio
		priceDelta := math.Abs(ann.DayOf.PriceChangePct)

		// Skip if no volume increase (nothing to measure)
		if volumeRatio <= 1.0 {
			continue
		}

		// Calculate Signal-to-Churn ratio
		// High price move per unit volume = efficient signal transmission
		signalToChurn := priceDelta / volumeRatio
		validCount++
		totalEfficiency += signalToChurn

		// Classify efficiency per prompt_3.md thresholds
		if signalToChurn > 1.0 {
			summary.HighEfficiencyCount++ // Pure Signal
		} else if signalToChurn < 0.5 {
			summary.LowEfficiencyCount++ // Churn/Uncertainty
		} else {
			summary.NeutralEfficiencyCount++
		}

		events = append(events, EfficiencyEvent{
			Date:            ann.Date,
			Headline:        ann.Headline,
			PriceChangePct:  priceDelta,
			VolumeZScore:    ann.DayOf.VolumeZScore, // Use pre-calculated Z-score
			EfficiencyRatio: signalToChurn,
		})
	}

	if validCount > 0 {
		summary.AverageEfficiency = totalEfficiency / float64(validCount)
		// Score = ratio of "Pure Signal" (high efficiency) events
		summary.EfficiencyScore = float64(summary.HighEfficiencyCount) / float64(validCount)
	}

	// Sort by efficiency ratio descending and take top 5 (best signal events)
	sort.Slice(events, func(i, j int) bool {
		return events[i].EfficiencyRatio > events[j].EfficiencyRatio
	})
	if len(events) > 5 {
		events = events[:5]
	}
	summary.HighEfficiencyEvents = events

	return summary
}

// calculateStdDev calculates standard deviation of a slice of floats
func calculateStdDev(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// Calculate mean
	var sum float64
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))

	// Calculate variance
	var variance float64
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(values))

	return math.Sqrt(variance)
}

// calculateRetentionSummary aggregates retention metrics for price-sensitive announcements only.
// Deduplicates by date and calculates a score as a ratio of positive to negative events:
//   - Positive: Price rises and holds/continues, or falls but recovers
//   - Negative: Price rises but fades, or falls and stays down
//   - Neutral: Day-of change < 1% (excluded from ratio)
//
// Score = Positive Events / (Positive Events + Negative Events)
// Range: 0.0 to 1.0
func (a *MQSAnalyzer) calculateRetentionSummary(announcements []MQSAnnouncement) RetentionSummary {
	// Focus on price-sensitive announcements only, then deduplicate by date
	filtered := filterPriceSensitive(announcements)
	filtered = deduplicateByDate(filtered)

	summary := RetentionSummary{
		TotalAnalyzed:    len(filtered),
		SignificantFades: []FadeEvent{},
	}

	var fades []FadeEvent

	for _, ann := range filtered {
		switch ann.RetentionClass {
		case RetentionNeutral:
			summary.NeutralCount++
		case RetentionPositive:
			summary.PositiveCount++
		case RetentionFade:
			summary.FadeCount++
			// Track fade events for display
			retentionRatio := 0.0
			if math.Abs(ann.DayOf.PriceChangePct) > 0.01 {
				retentionRatio = ann.LeadOut.PriceChangePct / ann.DayOf.PriceChangePct
			}
			fades = append(fades, FadeEvent{
				Date:           ann.Date,
				Headline:       ann.Headline,
				DayOfChange:    ann.DayOf.PriceChangePct,
				Day10Change:    ann.LeadOut.PriceChangePct,
				RetentionRatio: retentionRatio,
				PriceSensitive: ann.PriceSensitive,
			})
		case RetentionOverReaction:
			summary.OverReactionCount++
		case RetentionSustainedDrop:
			summary.SustainedDropCount++
		}
	}

	// Calculate aggregate counts
	summary.PositiveEvents = summary.PositiveCount + summary.OverReactionCount
	summary.NegativeEvents = summary.FadeCount + summary.SustainedDropCount

	// Calculate score as ratio: Positive / (Positive + Negative)
	totalNonNeutral := summary.PositiveEvents + summary.NegativeEvents
	if totalNonNeutral > 0 {
		summary.RetentionScore = float64(summary.PositiveEvents) / float64(totalNonNeutral)
	} else {
		summary.RetentionScore = 0.5 // Neutral if no significant events
	}

	// Take top 5 worst fades (sorted by retention ratio, lowest first)
	sort.Slice(fades, func(i, j int) bool {
		return fades[i].RetentionRatio < fades[j].RetentionRatio
	})
	if len(fades) > 5 {
		fades = fades[:5]
	}
	summary.SignificantFades = fades

	return summary
}

// extractFinancialResults identifies and extracts FY/HY results and guidance announcements
func (a *MQSAnalyzer) extractFinancialResults() []FinancialResult {
	var results []FinancialResult

	for _, ann := range a.announcements {
		headline := strings.ToUpper(ann.Headline)

		// Determine result type
		resultType, period := a.classifyFinancialAnnouncement(headline, ann.Date)
		if resultType == "" {
			continue // Not a financial result
		}

		// Calculate metrics
		dayOf := a.calculateDayOf(ann.Date)
		leadOut := a.calculateLeadOut(ann.Date)

		var dayOfChange, day10Change, volumeRatio float64
		if dayOf != nil {
			dayOfChange = dayOf.PriceChangePct
			volumeRatio = dayOf.VolumeRatio
		}
		if leadOut != nil {
			day10Change = leadOut.PriceChangePct
		}

		// Determine market review
		marketReview := "NEUTRAL"
		if dayOfChange > 2.0 && volumeRatio > 1.5 {
			marketReview = "POSITIVE"
		} else if dayOfChange < -2.0 && volumeRatio > 1.5 {
			marketReview = "NEGATIVE"
		} else if dayOfChange > 1.0 {
			marketReview = "POSITIVE"
		} else if dayOfChange < -1.0 {
			marketReview = "NEGATIVE"
		}

		result := FinancialResult{
			Date:         ann.Date.Format("2006-01-02"),
			Type:         resultType,
			Period:       period,
			Headline:     ann.Headline,
			PDFURL:       ann.PDFURL,
			DayOfChange:  dayOfChange,
			Day10Change:  day10Change,
			VolumeRatio:  volumeRatio,
			MarketReview: marketReview,
		}

		// Enrich with EODHD financial data if available
		a.enrichResultWithFundamentals(&result, ann.Date)

		results = append(results, result)
	}

	// Sort by date descending (most recent first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Date > results[j].Date
	})

	// Calculate YoY comparisons and streaks
	a.enrichFinancialResultsWithYoY(results)

	return results
}

// enrichFinancialResultsWithYoY adds YoY comparison data to financial results
func (a *MQSAnalyzer) enrichFinancialResultsWithYoY(results []FinancialResult) {
	// Group results by type for comparison
	resultsByType := make(map[FinancialResultType][]int) // maps to indices
	for i, r := range results {
		resultsByType[r.Type] = append(resultsByType[r.Type], i)
	}

	// For each result, find prior period of same type
	for i := range results {
		result := &results[i]

		// Find prior period (same type, older date)
		indices := resultsByType[result.Type]
		for _, j := range indices {
			if j <= i {
				continue // Skip self and earlier in sorted list (more recent)
			}
			prior := results[j]
			// Check if it's approximately 1 year ago (300-400 days)
			resultDate, _ := time.Parse("2006-01-02", result.Date)
			priorDate, _ := time.Parse("2006-01-02", prior.Date)
			daysDiff := resultDate.Sub(priorDate).Hours() / 24
			if daysDiff >= 300 && daysDiff <= 400 {
				result.PriorPeriodDate = prior.Date
				result.PriorPeriodChange = prior.DayOfChange
				result.YoYReactionDiff = result.DayOfChange - prior.DayOfChange

				// Determine trend
				if result.YoYReactionDiff > 2.0 {
					result.ReactionTrend = "IMPROVING"
				} else if result.YoYReactionDiff < -2.0 {
					result.ReactionTrend = "DECLINING"
				} else {
					result.ReactionTrend = "STABLE"
				}
				break
			}
		}
	}

	// Calculate consecutive positive/negative streaks
	a.calculateResultStreaks(results)
}

// calculateResultStreaks calculates consecutive positive/negative result streaks
func (a *MQSAnalyzer) calculateResultStreaks(results []FinancialResult) {
	// Process in chronological order (reverse of sorted order)
	for i := len(results) - 1; i >= 0; i-- {
		result := &results[i]

		// Look at prior result (next in array since sorted descending)
		if i < len(results)-1 {
			prior := results[i+1]

			if result.MarketReview == "POSITIVE" {
				if prior.MarketReview == "POSITIVE" {
					result.ConsecutivePositive = prior.ConsecutivePositive + 1
				} else {
					result.ConsecutivePositive = 1
				}
				result.ConsecutiveNegative = 0
			} else if result.MarketReview == "NEGATIVE" {
				if prior.MarketReview == "NEGATIVE" {
					result.ConsecutiveNegative = prior.ConsecutiveNegative + 1
				} else {
					result.ConsecutiveNegative = 1
				}
				result.ConsecutivePositive = 0
			}
		} else {
			// First result in chronological order
			if result.MarketReview == "POSITIVE" {
				result.ConsecutivePositive = 1
			} else if result.MarketReview == "NEGATIVE" {
				result.ConsecutiveNegative = 1
			}
		}
	}
}

// enrichResultWithFundamentals matches a financial result with EODHD data and populates business metrics
func (a *MQSAnalyzer) enrichResultWithFundamentals(result *FinancialResult, announcementDate time.Time) {
	if a.fundamentals == nil {
		return
	}

	// Determine which data source to use based on result type
	var matchedPeriod *market.FundamentalsFinancialPeriod
	var priorPeriod *market.FundamentalsFinancialPeriod

	switch result.Type {
	case ResultTypeFY, ResultType4E:
		// Full year results - match with annual data
		matchedPeriod, priorPeriod = a.findMatchingAnnualPeriod(announcementDate)
	case ResultTypeHY, ResultType4D:
		// Half year results - match with quarterly data (2 quarters combined)
		matchedPeriod, priorPeriod = a.findMatchingHalfYearPeriod(announcementDate)
	case ResultType4C, ResultTypeQ1, ResultTypeQ2, ResultTypeQ3, ResultTypeQ4:
		// Quarterly results - match with quarterly data
		matchedPeriod, priorPeriod = a.findMatchingQuarterlyPeriod(announcementDate)
	default:
		// Guidance or other - no financial data to match
		return
	}

	if matchedPeriod == nil {
		return
	}

	// Populate financial metrics
	result.Revenue = matchedPeriod.TotalRevenue
	result.NetIncome = matchedPeriod.NetIncome
	result.EBITDA = matchedPeriod.EBITDA
	result.GrossMargin = matchedPeriod.GrossMargin
	result.NetMargin = matchedPeriod.NetMargin
	result.OperatingCF = matchedPeriod.OperatingCF
	result.FreeCF = matchedPeriod.FreeCF
	result.HasFinancials = true

	// Calculate YoY growth if prior period available
	if priorPeriod != nil && priorPeriod.TotalRevenue > 0 {
		result.RevenueYoY = float64(matchedPeriod.TotalRevenue-priorPeriod.TotalRevenue) / float64(priorPeriod.TotalRevenue) * 100
	}
	if priorPeriod != nil && priorPeriod.NetIncome != 0 {
		// Handle sign changes in net income
		if priorPeriod.NetIncome > 0 {
			result.NetIncomeYoY = float64(matchedPeriod.NetIncome-priorPeriod.NetIncome) / float64(priorPeriod.NetIncome) * 100
		}
	}
}

// findMatchingAnnualPeriod finds the annual period that matches the announcement date
// Returns the matched period and the prior year period for YoY comparison
func (a *MQSAnalyzer) findMatchingAnnualPeriod(announcementDate time.Time) (*market.FundamentalsFinancialPeriod, *market.FundamentalsFinancialPeriod) {
	if len(a.fundamentals.AnnualData) == 0 {
		return nil, nil
	}

	// Australian FY ends June 30 - announcement in Aug/Sep is for FY ending June 30
	// Find the FY end date that this announcement is reporting on
	targetFYEnd := time.Date(announcementDate.Year(), 6, 30, 0, 0, 0, 0, time.UTC)
	if announcementDate.Month() < 7 {
		// Announcement before July - reporting on prior FY
		targetFYEnd = time.Date(announcementDate.Year()-1, 6, 30, 0, 0, 0, 0, time.UTC)
	}

	// Also try December 31 for calendar year companies
	targetCalYearEnd := time.Date(announcementDate.Year()-1, 12, 31, 0, 0, 0, 0, time.UTC)
	if announcementDate.Month() >= 7 {
		// Announcement in second half - could be reporting on current calendar year
		targetCalYearEnd = time.Date(announcementDate.Year(), 12, 31, 0, 0, 0, 0, time.UTC)
	}

	var matched, prior *market.FundamentalsFinancialPeriod
	for i := range a.fundamentals.AnnualData {
		period := &a.fundamentals.AnnualData[i]
		periodDate, err := time.Parse("2006-01-02", period.EndDate)
		if err != nil {
			continue
		}

		// Match if period end is within 3 months of target FY end (June 30)
		diff := periodDate.Sub(targetFYEnd)
		if diff < 0 {
			diff = -diff
		}
		if diff <= 90*24*time.Hour {
			matched = period
		}

		// Also try calendar year end (December 31) for non-Australian FY companies
		if matched == nil {
			diff = periodDate.Sub(targetCalYearEnd)
			if diff < 0 {
				diff = -diff
			}
			if diff <= 90*24*time.Hour {
				matched = period
			}
		}

		// Prior year is 12 months before
		priorFYEnd := targetFYEnd.AddDate(-1, 0, 0)
		diff = periodDate.Sub(priorFYEnd)
		if diff < 0 {
			diff = -diff
		}
		if diff <= 90*24*time.Hour {
			prior = period
		}

		// Also try prior calendar year
		if prior == nil {
			priorCalYearEnd := targetCalYearEnd.AddDate(-1, 0, 0)
			diff = periodDate.Sub(priorCalYearEnd)
			if diff < 0 {
				diff = -diff
			}
			if diff <= 90*24*time.Hour {
				prior = period
			}
		}
	}

	return matched, prior
}

// findMatchingHalfYearPeriod finds quarterly periods that match a half-year announcement
func (a *MQSAnalyzer) findMatchingHalfYearPeriod(announcementDate time.Time) (*market.FundamentalsFinancialPeriod, *market.FundamentalsFinancialPeriod) {
	if len(a.fundamentals.QuarterlyData) == 0 {
		return nil, nil
	}

	// For half-year, we look for the most recent quarter before the announcement
	// and combine with the prior quarter
	var matched *market.FundamentalsFinancialPeriod
	for i := range a.fundamentals.QuarterlyData {
		period := &a.fundamentals.QuarterlyData[i]
		periodDate, err := time.Parse("2006-01-02", period.EndDate)
		if err != nil {
			continue
		}

		// Match if period end is within 3 months before announcement
		diff := announcementDate.Sub(periodDate)
		if diff >= 0 && diff <= 90*24*time.Hour {
			matched = period
			break
		}
	}

	// For prior period, look 6 months back
	var prior *market.FundamentalsFinancialPeriod
	if matched != nil {
		matchedDate, _ := time.Parse("2006-01-02", matched.EndDate)
		priorTarget := matchedDate.AddDate(0, -6, 0)
		for i := range a.fundamentals.QuarterlyData {
			period := &a.fundamentals.QuarterlyData[i]
			periodDate, err := time.Parse("2006-01-02", period.EndDate)
			if err != nil {
				continue
			}
			diff := periodDate.Sub(priorTarget)
			if diff < 0 {
				diff = -diff
			}
			if diff <= 45*24*time.Hour {
				prior = period
				break
			}
		}
	}

	return matched, prior
}

// findMatchingQuarterlyPeriod finds the quarterly period that matches the announcement date
func (a *MQSAnalyzer) findMatchingQuarterlyPeriod(announcementDate time.Time) (*market.FundamentalsFinancialPeriod, *market.FundamentalsFinancialPeriod) {
	if len(a.fundamentals.QuarterlyData) == 0 {
		return nil, nil
	}

	var matched, prior *market.FundamentalsFinancialPeriod
	for i := range a.fundamentals.QuarterlyData {
		period := &a.fundamentals.QuarterlyData[i]
		periodDate, err := time.Parse("2006-01-02", period.EndDate)
		if err != nil {
			continue
		}

		// Match if period end is within 2 months before announcement
		diff := announcementDate.Sub(periodDate)
		if diff >= 0 && diff <= 60*24*time.Hour {
			matched = period
			// Prior is same quarter last year (4 quarters back)
			if i+4 < len(a.fundamentals.QuarterlyData) {
				prior = &a.fundamentals.QuarterlyData[i+4]
			}
			break
		}
	}

	return matched, prior
}

// classifyFinancialAnnouncement determines the type and period of a financial announcement
func (a *MQSAnalyzer) classifyFinancialAnnouncement(headline string, date time.Time) (FinancialResultType, string) {
	// Extract fiscal year from date - Australian FY ends June 30
	fy := date.Year()
	if date.Month() >= 7 {
		fy++ // After July, we're in the next FY
	}
	fyStr := fmt.Sprintf("FY%02d", fy%100)

	// Appendix 4E - Preliminary Final Report (Full Year)
	if strings.Contains(headline, "APPENDIX 4E") || strings.Contains(headline, "PRELIMINARY FINAL") {
		return ResultType4E, fyStr
	}

	// Appendix 4D - Half Year Report
	if strings.Contains(headline, "APPENDIX 4D") || strings.Contains(headline, "HALF YEARLY") {
		h := "H1"
		if date.Month() >= 7 && date.Month() <= 12 {
			h = "H1"
		} else {
			h = "H2"
		}
		return ResultType4D, fmt.Sprintf("%s %s", h, fyStr)
	}

	// Appendix 4C - Quarterly Cashflow
	if strings.Contains(headline, "APPENDIX 4C") || (strings.Contains(headline, "4C") && strings.Contains(headline, "CASH")) {
		q := a.getQuarter(date)
		return ResultType4C, fmt.Sprintf("Q%d %s", q, fyStr)
	}

	// Full Year Results
	if strings.Contains(headline, "FULL YEAR") || strings.Contains(headline, "ANNUAL") ||
		(strings.Contains(headline, "FY") && strings.Contains(headline, "RESULT")) {
		return ResultTypeFY, fyStr
	}

	// Half Year Results
	if strings.Contains(headline, "HALF YEAR") || strings.Contains(headline, "1H") || strings.Contains(headline, "2H") ||
		strings.Contains(headline, "H1") || strings.Contains(headline, "H2") {
		h := "H1"
		if strings.Contains(headline, "2H") || strings.Contains(headline, "H2") {
			h = "H2"
		} else if date.Month() >= 7 && date.Month() <= 12 {
			h = "H1"
		} else {
			h = "H2"
		}
		return ResultTypeHY, fmt.Sprintf("%s %s", h, fyStr)
	}

	// Quarterly Reports (by quarter keywords)
	if strings.Contains(headline, "QUARTERLY") || strings.Contains(headline, "QUARTER") {
		q := a.getQuarter(date)
		if strings.Contains(headline, "Q1") || strings.Contains(headline, "FIRST QUARTER") {
			q = 1
		} else if strings.Contains(headline, "Q2") || strings.Contains(headline, "SECOND QUARTER") {
			q = 2
		} else if strings.Contains(headline, "Q3") || strings.Contains(headline, "THIRD QUARTER") {
			q = 3
		} else if strings.Contains(headline, "Q4") || strings.Contains(headline, "FOURTH QUARTER") {
			q = 4
		}
		switch q {
		case 1:
			return ResultTypeQ1, fmt.Sprintf("Q1 %s", fyStr)
		case 2:
			return ResultTypeQ2, fmt.Sprintf("Q2 %s", fyStr)
		case 3:
			return ResultTypeQ3, fmt.Sprintf("Q3 %s", fyStr)
		case 4:
			return ResultTypeQ4, fmt.Sprintf("Q4 %s", fyStr)
		}
	}

	// Guidance updates
	if strings.Contains(headline, "GUIDANCE") || strings.Contains(headline, "EARNINGS UPDATE") ||
		strings.Contains(headline, "PROFIT UPGRADE") || strings.Contains(headline, "PROFIT DOWNGRADE") {
		return ResultTypeAG, fyStr
	}

	// Generic results (FY or HY)
	if strings.Contains(headline, "RESULT") {
		// Try to determine if it's half or full year from context
		if date.Month() >= 1 && date.Month() <= 3 {
			// Jan-Mar typically H1 results for June 30 FY companies
			return ResultTypeHY, fmt.Sprintf("H1 %s", fyStr)
		} else if date.Month() >= 7 && date.Month() <= 9 {
			// Jul-Sep typically FY results
			return ResultTypeFY, fyStr
		}
		return ResultTypeFY, fyStr
	}

	return "", ""
}

// getQuarter returns the Australian FY quarter (1-4) for a date
// Q1: Jul-Sep, Q2: Oct-Dec, Q3: Jan-Mar, Q4: Apr-Jun
func (a *MQSAnalyzer) getQuarter(date time.Time) int {
	switch date.Month() {
	case 7, 8, 9:
		return 1
	case 10, 11, 12:
		return 2
	case 1, 2, 3:
		return 3
	case 4, 5, 6:
		return 4
	}
	return 1
}

// calculateAggregateScore calculates the composite MQS score and tier
// All 5 components contribute equally (20% each) per prompt_2.md specification
func (a *MQSAnalyzer) calculateAggregateScore(
	leakage LeakageSummary,
	conviction ConvictionSummary,
	clarity ClaritySummary,
	efficiency EfficiencySummary,
	retention RetentionSummary,
	announcementCount int,
) MQSScore {
	// Calculate component scores (0-1 scale)
	leakageScore := 1.0 - leakage.LeakageRatio // Lower leakage = higher score
	convictionScore := conviction.InstitutionalRatio
	clarityScore := clarity.ClarityScore          // Already normalized 0.0-1.0
	efficiencyScore := efficiency.EfficiencyScore // Already normalized 0.0-1.0
	retentionScore := retention.RetentionScore    // Already normalized 0.0-1.0

	// Calculate composite (all 5 measures at 20% each)
	composite := CalculateCompositeMQS(leakageScore, convictionScore, clarityScore, efficiencyScore, retentionScore)

	// Determine tier based on composite score
	tier := DetermineMQSTier(composite, leakageScore, retentionScore)

	// Determine confidence
	confidence := DetermineConfidence(announcementCount)

	return MQSScore{
		CompositeScore:          composite,
		LeakageIntegrity:        leakageScore,
		InstitutionalConviction: convictionScore,
		ClarityIndex:            clarityScore,
		CommunicationEfficiency: efficiencyScore,
		ValueSustainability:     retentionScore,
		Tier:                    tier,
		Confidence:              confidence,
	}
}

// buildDetailedEvents creates the detailed events array with new analysis fields
func (a *MQSAnalyzer) buildDetailedEvents(announcements []MQSAnnouncement) []MQSDetailedEvent {
	events := make([]MQSDetailedEvent, 0, len(announcements))

	// Calculate 90-day volume stats for Z-score calculation
	volumeMean, volumeStdDev := a.calculate90DayVolumeStats()

	// Calculate 20-day volatility for leakage detection
	volatility20Day := a.calculate20DayVolatility()

	for _, ann := range announcements {
		// Determine event materiality (Strategic vs Routine)
		materiality := ClassifyEventMateriality(ann.Headline, ann.Category)

		// Calculate volume Z-score
		zScore := 0.0
		if ann.DayOf.Volume > 0 && volumeStdDev > 0 {
			zScore = CalculateVolumeZScore(ann.DayOf.Volume, volumeMean, volumeStdDev)
		}

		// Calculate CAR (Cumulative Abnormal Return) for pre-announcement period
		// Using LeadIn.PriceChangePct as the pre-drift measure
		preDriftCAR := ann.LeadIn.PriceChangePct / 100.0 // Convert percentage to decimal

		// Determine if this is a leakage event
		isLeakage := IsLeakage(preDriftCAR, volatility20Day)

		// Calculate retention at T+10
		// Retention = (Price_t+10 - Price_t-1) / (Price_t - Price_t-1)
		retention10D := 0.0
		if ann.LeadOut.EndPrice > 0 && ann.LeadIn.StartPrice > 0 && ann.DayOf.Close > 0 {
			retention10D = CalculateRetentionNew(
				ann.LeadOut.EndPrice,  // Price at T+10
				ann.LeadIn.StartPrice, // Price at T-1 (before announcement)
				ann.DayOf.Close,       // Price at T (announcement day close)
			)
		}

		event := MQSDetailedEvent{
			Date:         ann.Date,
			Headline:     ann.Headline,
			Type:         materiality,
			ZScore:       zScore,
			PreDriftCAR:  preDriftCAR,
			Retention10D: retention10D,
			IsLeakage:    isLeakage,
		}
		events = append(events, event)
	}

	return events
}

// calculate90DayVolumeStats calculates the 90-day rolling mean and standard deviation of volume
func (a *MQSAnalyzer) calculate90DayVolumeStats() (mean, stdDev float64) {
	if len(a.prices) < 20 {
		return 0, 0
	}

	// Use up to 90 days of data
	lookback := 90
	if len(a.prices) < lookback {
		lookback = len(a.prices)
	}

	// Calculate mean
	var sum float64
	for i := 0; i < lookback; i++ {
		sum += float64(a.prices[i].Volume)
	}
	mean = sum / float64(lookback)

	// Calculate standard deviation
	var sumSquares float64
	for i := 0; i < lookback; i++ {
		diff := float64(a.prices[i].Volume) - mean
		sumSquares += diff * diff
	}
	variance := sumSquares / float64(lookback)
	stdDev = math.Sqrt(variance)

	return mean, stdDev
}

// calculate20DayVolatility calculates the 20-day rolling volatility (standard deviation of returns)
func (a *MQSAnalyzer) calculate20DayVolatility() float64 {
	if len(a.prices) < 21 {
		return 0
	}

	// Calculate daily returns for last 20 days
	returns := make([]float64, 0, 20)
	for i := 0; i < 20 && i < len(a.prices)-1; i++ {
		if a.prices[i+1].Close > 0 {
			dailyReturn := (a.prices[i].Close - a.prices[i+1].Close) / a.prices[i+1].Close
			returns = append(returns, dailyReturn)
		}
	}

	if len(returns) == 0 {
		return 0
	}

	// Calculate mean return
	var sum float64
	for _, r := range returns {
		sum += r
	}
	mean := sum / float64(len(returns))

	// Calculate standard deviation
	var sumSquares float64
	for _, r := range returns {
		diff := r - mean
		sumSquares += diff * diff
	}
	variance := sumSquares / float64(len(returns))
	return math.Sqrt(variance)
}

// detectPatterns identifies recurring patterns in announcement behavior
func (a *MQSAnalyzer) detectPatterns(announcements []MQSAnnouncement) PatternAnalysis {
	patterns := PatternAnalysis{
		PRHeavySignals: []string{},
		QualitySignals: []string{},
		SeasonalNotes:  []string{},
	}

	// Check for recurring leakage around specific event types
	leakageByType := make(map[string]int)
	totalByType := make(map[string]int)

	for _, ann := range announcements {
		totalByType[ann.Category]++
		if ann.LeakageClass == LeakageHigh {
			leakageByType[ann.Category]++
		}
	}

	for category, leakCount := range leakageByType {
		total := totalByType[category]
		if total >= 3 && float64(leakCount)/float64(total) > 0.5 {
			patterns.PRHeavySignals = append(patterns.PRHeavySignals,
				fmt.Sprintf("High leakage before %s announcements (%d/%d)", category, leakCount, total))
		}
	}

	// Determine communication style based on dominant tone
	toneCounts := make(map[ToneClass]int)
	for _, ann := range a.announcements {
		tone := DetectTone(ann.Headline)
		toneCounts[tone]++
	}

	maxTone := ToneDataDry
	maxCount := 0
	for tone, count := range toneCounts {
		if count > maxCount {
			maxCount = count
			maxTone = tone
		}
	}

	switch maxTone {
	case ToneOptimistic:
		patterns.PRHeavySignals = append(patterns.PRHeavySignals, "Predominantly promotional communication style")
	case ToneConservative:
		patterns.QualitySignals = append(patterns.QualitySignals, "Conservative communication style")
	case ToneDataDry:
		patterns.QualitySignals = append(patterns.QualitySignals, "Factual, data-driven communication style")
	}

	// Check for quality signals
	tightShipCount := 0
	positiveRetentionCount := 0
	for _, ann := range announcements {
		if ann.LeakageClass == LeakageTight {
			tightShipCount++
		}
		// Positive retention: price held/continued or recovered from over-reaction
		if ann.RetentionClass == RetentionPositive || ann.RetentionClass == RetentionOverReaction {
			positiveRetentionCount++
		}
	}

	if len(announcements) > 0 {
		if float64(tightShipCount)/float64(len(announcements)) > 0.7 {
			patterns.QualitySignals = append(patterns.QualitySignals, "Consistently tight information control")
		}
		if float64(positiveRetentionCount)/float64(len(announcements)) > 0.7 {
			patterns.QualitySignals = append(patterns.QualitySignals, "Strong price retention after announcements")
		}
	}

	return patterns
}

// detectDataGaps identifies gaps in price data
func (a *MQSAnalyzer) detectDataGaps() []string {
	var gaps []string

	if len(a.prices) < 30 {
		gaps = append(gaps, fmt.Sprintf("Limited price data: only %d trading days available", len(a.prices)))
	}

	if len(a.announcements) < 10 {
		gaps = append(gaps, fmt.Sprintf("Limited announcement history: only %d announcements", len(a.announcements)))
	}

	return gaps
}

// buildHighImpactAnnouncements identifies high-impact announcements from the past 12 months
// and matches them with EODHD news articles for links.
// Impact is determined by: price change, volume, AND price retention (fade analysis).
// Only HIGH_SIGNAL announcements are included - those with minimal price fade.
func (a *MQSAnalyzer) buildHighImpactAnnouncements(mqsAnnouncements []MQSAnnouncement) []HighImpactAnnouncement {
	var results []HighImpactAnnouncement

	// Filter to past 12 months
	cutoffDate := time.Now().AddDate(-1, 0, 0)

	// Build a map from MQS announcements for quick lookup
	mqsMap := make(map[string]*MQSAnnouncement)
	for i := range mqsAnnouncements {
		mqsMap[mqsAnnouncements[i].Date] = &mqsAnnouncements[i]
	}

	for _, ann := range a.announcements {
		// Skip if older than 12 months
		if ann.Date.Before(cutoffDate) {
			continue
		}

		// Get MQS data for this announcement
		mqsAnn, hasMQS := mqsMap[ann.Date.Format("2006-01-02")]
		if !hasMQS {
			continue
		}

		// Get price change and volume metrics
		dayOfChange := mqsAnn.DayOf.PriceChangePct
		priceChange := math.Abs(dayOfChange)
		volumeRatio := mqsAnn.DayOf.VolumeRatio
		day10Change := mqsAnn.LeadOut.PriceChangePct

		// Calculate retention ratio (how much of the day-of move was retained after 10 days)
		// A ratio > 0.5 means the price held well (minimal fade)
		// A ratio < 0.5 means significant fade (price reversed)
		retentionRatio := 0.0
		if math.Abs(dayOfChange) > 0.01 {
			// For positive moves: day10 should stay positive
			// For negative moves: day10 should stay negative
			// Retention = day10/dayOf - if same sign and similar magnitude, ratio is high
			retentionRatio = day10Change / dayOfChange
		}

		// Determine impact rating based on price change, volume, AND retention
		// HIGH_SIGNAL: Strong initial reaction AND price held (minimal fade)
		// - Price change >= 3% OR volume >= 2x average
		// - AND retention ratio >= 0.5 (price held at least 50% of the move)
		//
		// MODERATE_SIGNAL: Strong initial reaction BUT significant fade
		// - Price change >= 1.5% OR volume >= 1.5x average
		// - BUT retention ratio < 0.5 (price faded more than 50%)
		//
		// We only include HIGH_SIGNAL in the output

		isHighInitialReaction := priceChange >= 3.0 || volumeRatio >= 2.0
		isModerateInitialReaction := priceChange >= 1.5 || volumeRatio >= 1.5
		hasMinimalFade := retentionRatio >= 0.5

		// Only include HIGH_SIGNAL announcements
		if !isHighInitialReaction && !isModerateInitialReaction {
			// Skip low-impact announcements
			continue
		}

		// For high initial reaction with minimal fade = HIGH_SIGNAL
		// For moderate initial reaction with minimal fade = also HIGH_SIGNAL (upgraded)
		// For any reaction with significant fade = skip (not truly high impact)
		if !hasMinimalFade {
			// Significant price fade - not a true high-impact announcement
			continue
		}

		// Create high-impact announcement
		highImpact := HighImpactAnnouncement{
			Date:           ann.Date.Format("2006-01-02"),
			Headline:       ann.Headline,
			Type:           ann.Type,
			PriceSensitive: ann.PriceSensitive,
			PriceChangePct: dayOfChange,
			VolumeRatio:    volumeRatio,
			Day10ChangePct: day10Change,
			RetentionRatio: retentionRatio,
			ImpactRating:   "HIGH_SIGNAL",
			PDFURL:         ann.PDFURL,
			DocumentKey:    ann.DocumentKey,
		}

		// Try to match with EODHD news
		if len(a.newsItems) > 0 {
			for i := range a.newsItems {
				newsItem := &a.newsItems[i]
				// Check date proximity (within 2 days)
				daysDiff := ann.Date.Sub(newsItem.Date).Hours() / 24
				if daysDiff < -2 || daysDiff > 2 {
					continue
				}

				// Check headline similarity
				headlineLower := strings.ToLower(ann.Headline)
				titleLower := strings.ToLower(newsItem.Title)

				// Simple matching: check if key words overlap
				words := strings.Fields(headlineLower)
				matchCount := 0
				for _, word := range words {
					if len(word) > 3 && strings.Contains(titleLower, word) {
						matchCount++
					}
				}

				if matchCount >= 2 || strings.Contains(titleLower, headlineLower) || strings.Contains(headlineLower, titleLower) {
					highImpact.NewsLink = newsItem.Link
					highImpact.NewsTitle = newsItem.Title
					highImpact.Sentiment = newsItem.Sentiment
					// Extract source from link if possible
					if strings.Contains(newsItem.Link, "reuters") {
						highImpact.NewsSource = "Reuters"
					} else if strings.Contains(newsItem.Link, "asx.com") {
						highImpact.NewsSource = "ASX"
					} else if strings.Contains(newsItem.Link, "afr.com") {
						highImpact.NewsSource = "AFR"
					} else {
						highImpact.NewsSource = "EODHD"
					}
					break
				}
			}
		}

		results = append(results, highImpact)
	}

	// Sort by date descending (most recent first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Date > results[j].Date
	})

	return results
}

// GenerateMarkdown generates the markdown output for MQS analysis
func (output *MQSOutput) GenerateMarkdown() string {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("# Management Quality Score Analysis: %s\n\n", output.Ticker))
	sb.WriteString(fmt.Sprintf("**Analysis Date:** %s  \n", output.AnalysisDate.Format("2006-01-02")))
	sb.WriteString(fmt.Sprintf("**Requested Period:** %s to %s (36 months)  \n", output.PeriodStart.Format("2006-01-02"), output.PeriodEnd.Format("2006-01-02")))
	sb.WriteString(fmt.Sprintf("**Total Announcements:** %d  \n", output.TotalAnnouncements))
	sb.WriteString(fmt.Sprintf("**Price-Sensitive Announcements:** %d  \n\n", output.PriceSensitiveAnnouncements))

	// MQS Score Summary
	sb.WriteString("## Management Quality Score\n\n")
	sb.WriteString(fmt.Sprintf("**Tier:** %s  \n", output.ManagementQualityScore.Tier))
	sb.WriteString(fmt.Sprintf("**Composite Score:** %.2f  \n", output.ManagementQualityScore.CompositeScore))
	sb.WriteString(fmt.Sprintf("**Confidence:** %s  \n\n", output.ManagementQualityScore.Confidence))

	// Asset class and sector info
	if output.Meta.AssetClass != "" {
		sb.WriteString(fmt.Sprintf("**Asset Class:** %s  \n", output.Meta.AssetClass))
	}
	if output.Meta.Sector != "" {
		sb.WriteString(fmt.Sprintf("**Sector:** %s  \n", output.Meta.Sector))
	}
	if output.Meta.MarketCap > 0 {
		sb.WriteString(fmt.Sprintf("**Market Cap:** $%.2fB  \n\n", float64(output.Meta.MarketCap)/1e9))
	} else {
		sb.WriteString("\n")
	}

	// Tier description (per prompt_2.md)
	switch output.ManagementQualityScore.Tier {
	case TierHighTrustLeader:
		sb.WriteString("✅ **HIGH-TRUST LEADER** (0.70-1.00): High integrity, low leakage, and high communication efficiency. ")
		sb.WriteString("Management demonstrates strong information integrity and reliable guidance.\n\n")
	case TierStableSteward:
		sb.WriteString("⚠️ **STABLE STEWARD** (0.50-0.69): Generally reliable but prone to occasional leakage or \"faded\" announcements. ")
		sb.WriteString("Communication is honest but results may be mixed.\n\n")
	case TierStrategicRisk:
		sb.WriteString("🚨 **STRATEGIC RISK** (<0.50): Low efficiency, frequent leakage, and poor volatility resolution. ")
		sb.WriteString("Exercise caution with announcements from this management team.\n\n")
	}

	// Component Scores Table with 5 measures at 20% each
	sb.WriteString("### Component Scores\n\n")
	sb.WriteString("| Component | Score | Weight | Contribution |\n")
	sb.WriteString("|-----------|-------|--------|-------------|\n")
	leakageContrib := output.ManagementQualityScore.LeakageIntegrity * 0.20
	convictionContrib := output.ManagementQualityScore.InstitutionalConviction * 0.20
	clarityContrib := output.ManagementQualityScore.ClarityIndex * 0.20
	efficiencyContrib := output.ManagementQualityScore.CommunicationEfficiency * 0.20
	retentionContrib := output.ManagementQualityScore.ValueSustainability * 0.20
	sb.WriteString(fmt.Sprintf("| Information Integrity (Leakage) | %.2f | 20%% | %.2f × 0.20 = %.3f |\n",
		output.ManagementQualityScore.LeakageIntegrity, output.ManagementQualityScore.LeakageIntegrity, leakageContrib))
	sb.WriteString(fmt.Sprintf("| Institutional Conviction (Z-Score) | %.2f | 20%% | %.2f × 0.20 = %.3f |\n",
		output.ManagementQualityScore.InstitutionalConviction, output.ManagementQualityScore.InstitutionalConviction, convictionContrib))
	sb.WriteString(fmt.Sprintf("| Clarity Index (Volatility Resolution) | %.2f | 20%% | %.2f × 0.20 = %.3f |\n",
		output.ManagementQualityScore.ClarityIndex, output.ManagementQualityScore.ClarityIndex, clarityContrib))
	sb.WriteString(fmt.Sprintf("| Communication Efficiency (Signal-to-Churn) | %.2f | 20%% | %.2f × 0.20 = %.3f |\n",
		output.ManagementQualityScore.CommunicationEfficiency, output.ManagementQualityScore.CommunicationEfficiency, efficiencyContrib))
	sb.WriteString(fmt.Sprintf("| Value Sustainability (Retention) | %.2f | 20%% | %.2f × 0.20 = %.3f |\n",
		output.ManagementQualityScore.ValueSustainability, output.ManagementQualityScore.ValueSustainability, retentionContrib))
	totalContrib := leakageContrib + convictionContrib + clarityContrib + efficiencyContrib + retentionContrib
	sb.WriteString(fmt.Sprintf("| **Composite** | **%.2f** | **100%%** | **%.3f + %.3f + %.3f + %.3f + %.3f = %.3f** |\n\n",
		output.ManagementQualityScore.CompositeScore, leakageContrib, convictionContrib, clarityContrib, efficiencyContrib, retentionContrib, totalContrib))

	// Calculation Method
	sb.WriteString("**Calculation Method:**\n")
	sb.WriteString("```\n")
	sb.WriteString("Composite = (Leakage × 0.20) + (Conviction × 0.20) + (Clarity × 0.20) + (Efficiency × 0.20) + (Retention × 0.20)\n")
	sb.WriteString("```\n\n")

	// Leakage Summary - include score in heading with "higher is better"
	sb.WriteString(fmt.Sprintf("## Information Integrity (CAR-Based Leakage Analysis) — Score: %.2f *(higher is better)*\n\n", output.ManagementQualityScore.LeakageIntegrity))
	sb.WriteString("*Measures Cumulative Abnormal Return (CAR) in the 5 days before Strategic announcements. ")
	sb.WriteString("Leakage is flagged when |CAR| > 2σ (20-day rolling volatility).*\n\n")

	sb.WriteString("**Selection Criteria:**\n")
	sb.WriteString(fmt.Sprintf("- Period: %s to %s\n", output.PeriodStart.Format("2006-01-02"), output.PeriodEnd.Format("2006-01-02")))
	sb.WriteString("- Price-sensitive announcements only\n")
	sb.WriteString("- Deduplicated by date (most significant announcement per day)\n\n")

	// List all values used in calculation FIRST
	sb.WriteString("**Input Values:**\n")
	sb.WriteString(fmt.Sprintf("- Total Strategic Events: %d\n", output.LeakageSummary.TotalAnalyzed))
	sb.WriteString(fmt.Sprintf("- High Leakage Events (|CAR| > 2σ): %d\n", output.LeakageSummary.HighLeakageCount))
	sb.WriteString(fmt.Sprintf("- Tight Ship Events: %d\n", output.LeakageSummary.TightShipCount))
	sb.WriteString(fmt.Sprintf("- Average Pre-Drift CAR: %.1f%%\n\n", output.LeakageSummary.AveragePreDriftPct))

	// Calculation explanation - show how Leakage Ratio is derived
	sb.WriteString("**Calculation:**\n")
	sb.WriteString(fmt.Sprintf("- Leakage Ratio = High Leakage Events / Total Strategic Events = %d / %d = %.2f\n",
		output.LeakageSummary.HighLeakageCount, output.LeakageSummary.TotalAnalyzed, output.LeakageSummary.LeakageRatio))
	sb.WriteString(fmt.Sprintf("- Leakage Score = 1.0 - Leakage Ratio = 1.0 - %.2f = **%.2f**\n\n",
		output.LeakageSummary.LeakageRatio, output.ManagementQualityScore.LeakageIntegrity))

	if len(output.LeakageSummary.WorstLeakages) > 0 {
		sb.WriteString("### Notable Pre-Announcement Drift Events\n\n")
		sb.WriteString("| Date | Headline | Pre-Drift CAR |\n")
		sb.WriteString("|------|----------|---------------|\n")
		for _, leak := range output.LeakageSummary.WorstLeakages {
			headline := leak.Headline
			if len(headline) > 50 {
				headline = headline[:47] + "..."
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %+.1f%% |\n",
				leak.Date, headline, leak.PreDriftPct))
		}
		sb.WriteString("\n")
	}

	// Conviction Summary - Institutional Conviction with VWAP (updated per prompt_3.md)
	sb.WriteString(fmt.Sprintf("## Institutional Conviction (VWAP-Based) — Score: %.2f *(higher is better)*\n\n", output.ManagementQualityScore.InstitutionalConviction))
	sb.WriteString("*Identifies 'smart money' entry on management news using VWAP analysis.*\n\n")

	sb.WriteString("**Formula (per prompt_3.md):**\n")
	sb.WriteString("```\n")
	sb.WriteString("conviction_score = (z_score_vol × 0.6) + (price_vs_vwap × 0.4)\n")
	sb.WriteString("```\n")
	sb.WriteString("- **z_score_vol**: Volume Z-Score vs 20-day moving average\n")
	sb.WriteString("- **price_vs_vwap**: (Close - VWAP) / VWAP — positive = price above VWAP (institutional accumulation)\n\n")

	sb.WriteString("**Selection Criteria:**\n")
	sb.WriteString(fmt.Sprintf("- Period: %s to %s\n", output.PeriodStart.Format("2006-01-02"), output.PeriodEnd.Format("2006-01-02")))
	sb.WriteString("- All announcements included\n\n")

	// Event Type Definitions
	sb.WriteString("**Event Classifications:**\n")
	sb.WriteString("- **Institutional Conviction**: High volume Z-Score + Price above VWAP — 'smart money' backing\n")
	sb.WriteString("- **Retail Hype**: High price change with low volume Z-Score — speculative interest\n")
	sb.WriteString("- **Low Interest**: Low Z-Score and minimal price change — minimal market reaction\n\n")

	// List all values used in calculation FIRST
	totalConvictionEvents := output.ConvictionSummary.InstitutionalCount + output.ConvictionSummary.RetailHypeCount + output.ConvictionSummary.LowInterestCount
	sb.WriteString("**Input Values:**\n")
	sb.WriteString(fmt.Sprintf("- Total Events Analyzed: %d\n", output.ConvictionSummary.TotalAnalyzed))
	sb.WriteString(fmt.Sprintf("- Institutional Conviction Events: %d\n", output.ConvictionSummary.InstitutionalCount))
	sb.WriteString(fmt.Sprintf("- Retail Hype Events: %d\n", output.ConvictionSummary.RetailHypeCount))
	sb.WriteString(fmt.Sprintf("- Low Interest Events: %d\n", output.ConvictionSummary.LowInterestCount))
	sb.WriteString(fmt.Sprintf("- Events Above VWAP: %d (%.1f%%)\n", output.ConvictionSummary.AboveVWAPCount,
		float64(output.ConvictionSummary.AboveVWAPCount)/float64(max(output.ConvictionSummary.TotalAnalyzed, 1))*100))
	sb.WriteString(fmt.Sprintf("- High Volume + Above VWAP Ratio: %.2f\n", output.ConvictionSummary.HighVolumeAboveVWAPRatio))
	sb.WriteString(fmt.Sprintf("- Average Volume Z-Score: %.2f\n", output.ConvictionSummary.AverageVolumeZScore))
	sb.WriteString(fmt.Sprintf("- Average Price vs VWAP: %.2f%%\n", output.ConvictionSummary.AveragePriceVsVWAP*100))
	sb.WriteString(fmt.Sprintf("- Average Conviction Score: %.2f\n\n", output.ConvictionSummary.AverageConvictionScore))

	// Calculation explanation
	sb.WriteString("**Calculation:**\n")
	sb.WriteString(fmt.Sprintf("- Conviction Score = Triggered Events / Total Events = %d / %d = **%.2f**\n\n",
		output.ConvictionSummary.InstitutionalCount, totalConvictionEvents, output.ManagementQualityScore.InstitutionalConviction))

	// Clarity Index Summary (NEW per prompt_2.md)
	sb.WriteString(fmt.Sprintf("## Clarity Index (Volatility Resolution) — Score: %.2f *(higher is better)*\n\n", output.ManagementQualityScore.ClarityIndex))
	sb.WriteString("*Quantifies management's ability to resolve market uncertainty. ")
	sb.WriteString("Compares return volatility 15 days before vs 15 days after the event (excluding Day 0).*\n\n")

	sb.WriteString("**Selection Criteria:**\n")
	sb.WriteString(fmt.Sprintf("- Period: %s to %s\n", output.PeriodStart.Format("2006-01-02"), output.PeriodEnd.Format("2006-01-02")))
	sb.WriteString("- Price-sensitive announcements only\n")
	sb.WriteString("- Deduplicated by date (most significant announcement per day)\n\n")

	sb.WriteString("**Formula:** Clarity Index = σ_pre / σ_post\n")
	sb.WriteString("- Index > 1.0: Management successfully lowered stock's risk profile\n")
	sb.WriteString("- Index < 1.0: Volatility increased after announcement\n\n")

	sb.WriteString("**Input Values:**\n")
	sb.WriteString(fmt.Sprintf("- Total Events Analyzed: %d\n", output.ClaritySummary.TotalAnalyzed))
	sb.WriteString(fmt.Sprintf("- Volatility Reduced (Index > 1.0): %d\n", output.ClaritySummary.VolatilityReduced))
	sb.WriteString(fmt.Sprintf("- Volatility Increased (Index < 1.0): %d\n", output.ClaritySummary.VolatilityIncreased))
	sb.WriteString(fmt.Sprintf("- Average Pre-Volatility (σ_pre): %.4f\n", output.ClaritySummary.AveragePreVolatility))
	sb.WriteString(fmt.Sprintf("- Average Post-Volatility (σ_post): %.4f\n", output.ClaritySummary.AveragePostVolatility))
	sb.WriteString(fmt.Sprintf("- Average Clarity Index: %.2f\n\n", output.ClaritySummary.AverageClarityIndex))

	sb.WriteString("**Calculation:**\n")
	sb.WriteString(fmt.Sprintf("- Clarity Score = Volatility Reduced / Total = %d / %d = **%.2f**\n\n",
		output.ClaritySummary.VolatilityReduced, output.ClaritySummary.TotalAnalyzed, output.ManagementQualityScore.ClarityIndex))

	// Communication Efficiency Summary (updated per prompt_3.md)
	sb.WriteString(fmt.Sprintf("## Communication Efficiency (Signal-to-Churn) — Score: %.2f *(higher is better)*\n\n", output.ManagementQualityScore.CommunicationEfficiency))
	sb.WriteString("*Measures the \"Trust Premium\" — how easily the market accepts the message.*\n\n")

	sb.WriteString("**Formula (per prompt_3.md):**\n")
	sb.WriteString("```\n")
	sb.WriteString("signal_to_churn = price_delta / (vol_at_event / market_avg_vol)\n")
	sb.WriteString("                = |price_delta| / volume_ratio\n")
	sb.WriteString("```\n")
	sb.WriteString("- **Pure Signal** (> 1.0): High price move per unit volume — market trusts immediately\n")
	sb.WriteString("- **Churn/Uncertainty** (< 0.5): High volume with modest price move — lack of credibility\n\n")

	sb.WriteString("**Selection Criteria:**\n")
	sb.WriteString(fmt.Sprintf("- Period: %s to %s\n", output.PeriodStart.Format("2006-01-02"), output.PeriodEnd.Format("2006-01-02")))
	sb.WriteString("- Price-sensitive announcements only\n")
	sb.WriteString("- Deduplicated by date (most significant announcement per day)\n")
	sb.WriteString("- Volume ratio > 1.0 (above-average volume events only)\n\n")

	sb.WriteString("**Input Values:**\n")
	sb.WriteString(fmt.Sprintf("- Total Events Analyzed: %d\n", output.EfficiencySummary.TotalAnalyzed))
	sb.WriteString(fmt.Sprintf("- Pure Signal Events (> 1.0): %d\n", output.EfficiencySummary.HighEfficiencyCount))
	sb.WriteString(fmt.Sprintf("- Neutral Events: %d\n", output.EfficiencySummary.NeutralEfficiencyCount))
	sb.WriteString(fmt.Sprintf("- Churn Events (< 0.5): %d\n", output.EfficiencySummary.LowEfficiencyCount))
	sb.WriteString(fmt.Sprintf("- Average Signal-to-Churn Ratio: %.2f\n\n", output.EfficiencySummary.AverageEfficiency))

	totalEfficiencyEvents := output.EfficiencySummary.HighEfficiencyCount + output.EfficiencySummary.NeutralEfficiencyCount + output.EfficiencySummary.LowEfficiencyCount
	sb.WriteString("**Calculation:**\n")
	sb.WriteString(fmt.Sprintf("- Efficiency Score = Pure Signal Events / Total = %d / %d = **%.2f**\n\n",
		output.EfficiencySummary.HighEfficiencyCount, totalEfficiencyEvents, output.ManagementQualityScore.CommunicationEfficiency))

	// Retention Summary - Value Sustainability (renamed per prompt_2.md)
	sb.WriteString(fmt.Sprintf("## Value Sustainability (Price Retention) — Score: %.2f *(higher is better)*\n\n", output.ManagementQualityScore.ValueSustainability))
	sb.WriteString("*Differentiates between temporary \"hype\" and permanent value creation. ")
	sb.WriteString("Compares price at T+10 to baseline at T-1.*\n\n")

	sb.WriteString("**Selection Criteria:**\n")
	sb.WriteString(fmt.Sprintf("- Period: %s to %s\n", output.PeriodStart.Format("2006-01-02"), output.PeriodEnd.Format("2006-01-02")))
	sb.WriteString("- Price-sensitive announcements only\n")
	sb.WriteString("- Deduplicated by date (most significant announcement per day)\n\n")

	sb.WriteString("**Classification Rules:**\n")
	sb.WriteString("- **Positive**: Price rises on announcement AND holds/continues at T+10\n")
	sb.WriteString("- **Over-reaction Recovery**: Price falls on announcement BUT recovers at T+10\n")
	sb.WriteString("- **Fade**: Price rises on announcement BUT doesn't hold at T+10\n")
	sb.WriteString("- **Sustained Drop**: Price falls on announcement AND stays down at T+10\n")
	sb.WriteString("- **Neutral**: Day-of price change < 1% (excluded from ratio)\n\n")

	// List all values used in calculation FIRST
	sb.WriteString("**Input Values:**\n")
	sb.WriteString(fmt.Sprintf("- Total Price-Sensitive Events: %d\n", output.RetentionSummary.TotalAnalyzed))
	sb.WriteString(fmt.Sprintf("- Positive (rose and held): %d\n", output.RetentionSummary.PositiveCount))
	sb.WriteString(fmt.Sprintf("- Over-reaction Recovery (fell, recovered): %d\n", output.RetentionSummary.OverReactionCount))
	sb.WriteString(fmt.Sprintf("- Fade (rose, didn't hold): %d\n", output.RetentionSummary.FadeCount))
	sb.WriteString(fmt.Sprintf("- Sustained Drop (fell, stayed down): %d\n", output.RetentionSummary.SustainedDropCount))
	sb.WriteString(fmt.Sprintf("- Neutral (< 1%% move): %d\n", output.RetentionSummary.NeutralCount))
	sb.WriteString(fmt.Sprintf("- **Positive Events: %d** (%d + %d)\n",
		output.RetentionSummary.PositiveEvents, output.RetentionSummary.PositiveCount, output.RetentionSummary.OverReactionCount))
	sb.WriteString(fmt.Sprintf("- **Negative Events: %d** (%d + %d)\n\n",
		output.RetentionSummary.NegativeEvents, output.RetentionSummary.FadeCount, output.RetentionSummary.SustainedDropCount))

	// Calculation explanation
	totalNonNeutral := output.RetentionSummary.PositiveEvents + output.RetentionSummary.NegativeEvents
	sb.WriteString("**Calculation:**\n")
	sb.WriteString(fmt.Sprintf("- Sustainability Score = Positive Events / (Positive + Negative) = %d / %d = **%.2f**\n\n",
		output.RetentionSummary.PositiveEvents, totalNonNeutral, output.ManagementQualityScore.ValueSustainability))

	if len(output.RetentionSummary.SignificantFades) > 0 {
		sb.WriteString("### Significant Price Fades\n\n")
		sb.WriteString("| Date | Headline | Day-Of | Day+10 | Retention |\n")
		sb.WriteString("|------|----------|--------|--------|----------|\n")
		for _, fade := range output.RetentionSummary.SignificantFades {
			headline := fade.Headline
			if len(headline) > 40 {
				headline = headline[:37] + "..."
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %+.1f%% | %+.1f%% | %.2f |\n",
				fade.Date, headline, fade.DayOfChange, fade.Day10Change, fade.RetentionRatio))
		}
		sb.WriteString("\n")
	}

	// Pattern Analysis
	if len(output.Patterns.PRHeavySignals) > 0 || len(output.Patterns.QualitySignals) > 0 {
		sb.WriteString("## Pattern Analysis\n\n")
		if len(output.Patterns.QualitySignals) > 0 {
			sb.WriteString("### Quality Signals\n")
			for _, signal := range output.Patterns.QualitySignals {
				sb.WriteString(fmt.Sprintf("- ✅ %s\n", signal))
			}
			sb.WriteString("\n")
		}
		if len(output.Patterns.PRHeavySignals) > 0 {
			sb.WriteString("### Concerns\n")
			for _, signal := range output.Patterns.PRHeavySignals {
				sb.WriteString(fmt.Sprintf("- ⚠️ %s\n", signal))
			}
			sb.WriteString("\n")
		}
	}

	// High-Impact Announcements (past 12 months)
	if len(output.HighImpactAnnouncements) > 0 {
		sb.WriteString("## High-Impact Announcements (Past 12 Months)\n\n")
		sb.WriteString("Announcements with significant market reaction AND minimal price fade (retention ≥50%).\n\n")

		sb.WriteString("| Date | PS | Headline | Day-Of Δ | Day+10 Δ | Retention | Volume | Link |\n")
		sb.WriteString("|------|:--:|----------|----------|----------|-----------|--------|------|\n")

		// Group announcements by date
		currentDate := ""
		for _, ann := range output.HighImpactAnnouncements {
			// Add date separator row if date changes
			if ann.Date != currentDate {
				if currentDate != "" {
					// Add a visual separator between date groups (empty row not valid in markdown tables)
				}
				currentDate = ann.Date
			}

			// Price-sensitive indicator (check mark)
			psStr := ""
			if ann.PriceSensitive {
				psStr = "✓"
			}

			// Format day-of price change with color (soft green/red)
			dayOfStr := fmt.Sprintf("%+.1f%%", ann.PriceChangePct)
			if ann.PriceChangePct > 0 {
				dayOfStr = fmt.Sprintf("<span style=\"color:#4CAF50\">%s</span>", dayOfStr)
			} else if ann.PriceChangePct < 0 {
				dayOfStr = fmt.Sprintf("<span style=\"color:#E57373\">%s</span>", dayOfStr)
			}

			// Format day+10 price change (soft green/red)
			day10Str := fmt.Sprintf("%+.1f%%", ann.Day10ChangePct)
			if ann.Day10ChangePct > 0 {
				day10Str = fmt.Sprintf("<span style=\"color:#4CAF50\">%s</span>", day10Str)
			} else if ann.Day10ChangePct < 0 {
				day10Str = fmt.Sprintf("<span style=\"color:#E57373\">%s</span>", day10Str)
			}

			// Format retention ratio (soft green/red based on retention quality)
			retentionStr := fmt.Sprintf("%.0f%%", ann.RetentionRatio*100)
			if ann.RetentionRatio >= 0.75 {
				retentionStr = fmt.Sprintf("<span style=\"color:#4CAF50\">%s</span>", retentionStr) // Strong retention
			} else {
				retentionStr = fmt.Sprintf("<span style=\"color:#E57373\">%s</span>", retentionStr) // Moderate retention (50-75%)
			}

			// Format volume ratio
			volumeStr := fmt.Sprintf("%.1fx", ann.VolumeRatio)

			// Format link - prefer news link, fall back to PDF
			linkStr := "-"
			if ann.NewsLink != "" {
				linkStr = fmt.Sprintf("[%s](%s)", ann.NewsSource, ann.NewsLink)
			} else if ann.PDFURL != "" {
				linkStr = fmt.Sprintf("[PDF](%s)", ann.PDFURL)
			}

			// Truncate headline if too long
			headline := ann.Headline
			if len(headline) > 45 {
				headline = headline[:42] + "..."
			}

			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s | %s | %s |\n",
				ann.Date, psStr, headline, dayOfStr, day10Str, retentionStr, volumeStr, linkStr))
		}
		sb.WriteString("\n")

		sb.WriteString("*PS = Price Sensitive (ASX marked)*\n\n")

		// Add summary
		sb.WriteString(fmt.Sprintf("*Showing %d high-impact announcements with minimal price fade.*\n\n", len(output.HighImpactAnnouncements)))

		// Add sentiment summary if we have news matches
		newsMatchCount := 0
		pdfDownloadCount := 0
		for _, ann := range output.HighImpactAnnouncements {
			if ann.NewsLink != "" {
				newsMatchCount++
			}
			if ann.PDFDownloaded {
				pdfDownloadCount++
			}
		}
		if newsMatchCount > 0 {
			sb.WriteString(fmt.Sprintf("*%d announcements matched with EODHD news articles.*\n\n",
				newsMatchCount))
		}
		if pdfDownloadCount > 0 {
			sb.WriteString(fmt.Sprintf("*%d PDF documents downloaded and stored.*\n\n",
				pdfDownloadCount))
		}
	}

	return sb.String()
}

// formatCurrency formats a currency value in a human-readable format
// Uses M for millions, B for billions
func formatCurrency(value int64) string {
	if value == 0 {
		return "-"
	}

	absValue := value
	sign := ""
	if value < 0 {
		absValue = -value
		sign = "-"
	}

	switch {
	case absValue >= 1_000_000_000:
		return fmt.Sprintf("%s%.1fB", sign, float64(absValue)/1_000_000_000)
	case absValue >= 1_000_000:
		return fmt.Sprintf("%s%.1fM", sign, float64(absValue)/1_000_000)
	case absValue >= 1_000:
		return fmt.Sprintf("%s%.0fK", sign, float64(absValue)/1_000)
	default:
		return fmt.Sprintf("%s%d", sign, absValue)
	}
}
