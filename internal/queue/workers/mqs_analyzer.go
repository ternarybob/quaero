// -----------------------------------------------------------------------
// MQS Analyzer - Management Quality Score Analysis Functions
// Calculates MQS metrics from announcements and price data
// -----------------------------------------------------------------------

package workers

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// MQSAnalyzer calculates Management Quality Scores from announcement and price data
type MQSAnalyzer struct {
	announcements []ASXAnnouncement
	prices        []OHLCV
	priceMap      map[string]OHLCV // date string -> OHLCV
	ticker        string
	exchange      string
	fundamentals  *FundamentalsFinancialData // EODHD financial data (optional)
	newsItems     []EODHDNewsItem            // EODHD news for matching (optional)
}

// NewMQSAnalyzer creates a new MQS analyzer
func NewMQSAnalyzer(announcements []ASXAnnouncement, prices []OHLCV, ticker, exchange string) *MQSAnalyzer {
	// Build price map for O(1) lookups
	priceMap := make(map[string]OHLCV)
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
func (a *MQSAnalyzer) SetFundamentals(data *FundamentalsFinancialData) {
	a.fundamentals = data
}

// SetNews sets the EODHD news items for matching with high-impact announcements
func (a *MQSAnalyzer) SetNews(news []EODHDNewsItem) {
	a.newsItems = news
}

// Analyze performs the full MQS analysis and returns the output
func (a *MQSAnalyzer) Analyze() *MQSOutput {
	now := time.Now()

	// Determine analysis period
	var periodStart, periodEnd time.Time
	if len(a.announcements) > 0 {
		// Announcements are sorted newest first
		periodEnd = a.announcements[0].Date
		periodStart = a.announcements[len(a.announcements)-1].Date
	} else {
		periodEnd = now
		periodStart = now.AddDate(-2, 0, 0) // Default 2 years
	}

	output := &MQSOutput{
		Ticker:       fmt.Sprintf("%s.%s", a.exchange, a.ticker),
		Exchange:     a.exchange,
		AnalysisDate: now,
		PeriodStart:  periodStart,
		PeriodEnd:    periodEnd,
	}

	// Analyze each announcement
	mqsAnnouncements := a.analyzeAnnouncements()
	output.Announcements = mqsAnnouncements

	// Calculate component summaries
	output.LeakageSummary = a.calculateLeakageSummary(mqsAnnouncements)
	output.ConvictionSummary = a.calculateConvictionSummary(mqsAnnouncements)
	output.RetentionSummary = a.calculateRetentionSummary(mqsAnnouncements)

	// Calculate aggregate scores (Leakage 33%, Conviction 33%, Retention 34%)
	output.ManagementQualityScore = a.calculateAggregateScore(
		output.LeakageSummary,
		output.ConvictionSummary,
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
func (a *MQSAnalyzer) analyzeSingleAnnouncement(ann ASXAnnouncement) *MQSAnnouncement {
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
	var priceT5, priceT1 OHLCV
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
	var dayPrice OHLCV
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

	return &DayOfMetrics{
		Open:           dayPrice.Open,
		High:           dayPrice.High,
		Low:            dayPrice.Low,
		Close:          dayPrice.Close,
		Volume:         dayPrice.Volume,
		PriceChangePct: priceChange,
		VolumeRatio:    volumeRatio,
	}
}

// calculateLeadOut calculates the 10 trading days after announcement
func (a *MQSAnalyzer) calculateLeadOut(annDate time.Time) *LeadMetrics {
	// Find T+1 and T+10 prices
	var priceT1, priceT10 OHLCV
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

// calculateLeakageSummary aggregates leakage metrics across all announcements
func (a *MQSAnalyzer) calculateLeakageSummary(announcements []MQSAnnouncement) LeakageSummary {
	summary := LeakageSummary{
		TotalAnalyzed: len(announcements),
		WorstLeakages: []LeakageIncident{},
	}

	var totalPreDrift float64
	var leakages []LeakageIncident

	for _, ann := range announcements {
		switch ann.LeakageClass {
		case LeakageHigh:
			summary.HighLeakageCount++
			// Determine direction alignment
			direction := "ALIGNED"
			if (ann.LeadIn.PriceChangePct > 0 && ann.DayOf.PriceChangePct < 0) ||
				(ann.LeadIn.PriceChangePct < 0 && ann.DayOf.PriceChangePct > 0) {
				direction = "OPPOSING"
			}
			leakages = append(leakages, LeakageIncident{
				Date:        ann.Date,
				Headline:    ann.Headline,
				PreDriftPct: ann.LeadIn.PriceChangePct,
				Direction:   direction,
			})
		case LeakageTight:
			summary.TightShipCount++
		case LeakageNeutral:
			summary.NeutralCount++
		}
		totalPreDrift += math.Abs(ann.LeadIn.PriceChangePct)
	}

	if len(announcements) > 0 {
		summary.AveragePreDriftPct = totalPreDrift / float64(len(announcements))
		summary.LeakageRatio = float64(summary.HighLeakageCount) / float64(len(announcements))
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
func (a *MQSAnalyzer) calculateConvictionSummary(announcements []MQSAnnouncement) ConvictionSummary {
	summary := ConvictionSummary{
		TotalAnalyzed:        len(announcements),
		HighConvictionEvents: []ConvictionEvent{},
	}

	var totalVolumeRatio float64
	var highConviction []ConvictionEvent

	for _, ann := range announcements {
		totalVolumeRatio += ann.DayOf.VolumeRatio

		switch ann.ConvictionClass {
		case ConvictionInstitutional:
			summary.InstitutionalCount++
			highConviction = append(highConviction, ConvictionEvent{
				Date:        ann.Date,
				Headline:    ann.Headline,
				PriceChange: ann.DayOf.PriceChangePct,
				VolumeRatio: ann.DayOf.VolumeRatio,
				Class:       string(ann.ConvictionClass),
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
		summary.AverageVolumeRatio = totalVolumeRatio / float64(len(announcements))
		summary.InstitutionalRatio = float64(summary.InstitutionalCount) / float64(len(announcements))
	}

	// Take top 5 high conviction events
	if len(highConviction) > 5 {
		highConviction = highConviction[:5]
	}
	summary.HighConvictionEvents = highConviction

	return summary
}

// calculateRetentionSummary aggregates retention metrics
func (a *MQSAnalyzer) calculateRetentionSummary(announcements []MQSAnnouncement) RetentionSummary {
	summary := RetentionSummary{
		TotalAnalyzed:    len(announcements),
		SignificantFades: []FadeEvent{},
	}

	var totalRetention float64
	var fades []FadeEvent
	analyzedCount := 0

	for _, ann := range announcements {
		// Skip if no meaningful day-of change
		if math.Abs(ann.DayOf.PriceChangePct) < 0.01 {
			continue
		}

		analyzedCount++
		retentionRatio := ann.LeadOut.PriceChangePct / ann.DayOf.PriceChangePct
		totalRetention += retentionRatio

		switch ann.RetentionClass {
		case RetentionAbsorbed:
			summary.AbsorbedCount++
		case RetentionContinued:
			summary.ContinuedCount++
		case RetentionSoldNews:
			summary.SoldNewsCount++
			fades = append(fades, FadeEvent{
				Date:           ann.Date,
				Headline:       ann.Headline,
				DayOfChange:    ann.DayOf.PriceChangePct,
				Day10Change:    ann.LeadOut.PriceChangePct,
				RetentionRatio: retentionRatio,
			})
		case RetentionReversed:
			summary.ReversedCount++
			fades = append(fades, FadeEvent{
				Date:           ann.Date,
				Headline:       ann.Headline,
				DayOfChange:    ann.DayOf.PriceChangePct,
				Day10Change:    ann.LeadOut.PriceChangePct,
				RetentionRatio: retentionRatio,
			})
		}
	}

	summary.TotalAnalyzed = analyzedCount
	if analyzedCount > 0 {
		summary.AverageRetentionRatio = totalRetention / float64(analyzedCount)
		summary.RetentionRate = float64(summary.AbsorbedCount+summary.ContinuedCount) / float64(analyzedCount)
	}

	// Take top 5 worst fades
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
	var matchedPeriod *FundamentalsFinancialPeriod
	var priorPeriod *FundamentalsFinancialPeriod

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
func (a *MQSAnalyzer) findMatchingAnnualPeriod(announcementDate time.Time) (*FundamentalsFinancialPeriod, *FundamentalsFinancialPeriod) {
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

	var matched, prior *FundamentalsFinancialPeriod
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
func (a *MQSAnalyzer) findMatchingHalfYearPeriod(announcementDate time.Time) (*FundamentalsFinancialPeriod, *FundamentalsFinancialPeriod) {
	if len(a.fundamentals.QuarterlyData) == 0 {
		return nil, nil
	}

	// For half-year, we look for the most recent quarter before the announcement
	// and combine with the prior quarter
	var matched *FundamentalsFinancialPeriod
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
	var prior *FundamentalsFinancialPeriod
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
func (a *MQSAnalyzer) findMatchingQuarterlyPeriod(announcementDate time.Time) (*FundamentalsFinancialPeriod, *FundamentalsFinancialPeriod) {
	if len(a.fundamentals.QuarterlyData) == 0 {
		return nil, nil
	}

	var matched, prior *FundamentalsFinancialPeriod
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
func (a *MQSAnalyzer) calculateAggregateScore(
	leakage LeakageSummary,
	conviction ConvictionSummary,
	retention RetentionSummary,
	announcementCount int,
) MQSScore {
	// Calculate component scores (0-1 scale)
	leakageScore := 1.0 - leakage.LeakageRatio // Lower leakage = higher score
	convictionScore := conviction.InstitutionalRatio
	retentionScore := retention.RetentionRate

	// Calculate composite (Leakage 33%, Conviction 33%, Retention 34%)
	composite := CalculateCompositeMQS(leakageScore, convictionScore, retentionScore)

	// Determine tier
	tier := DetermineMQSTier(composite, leakageScore, retentionScore)

	// Determine confidence
	confidence := DetermineConfidence(announcementCount)

	return MQSScore{
		CompositeScore:  composite,
		LeakageScore:    leakageScore,
		ConvictionScore: convictionScore,
		RetentionScore:  retentionScore,
		Tier:            tier,
		Confidence:      confidence,
	}
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
	absorbedCount := 0
	for _, ann := range announcements {
		if ann.LeakageClass == LeakageTight {
			tightShipCount++
		}
		if ann.RetentionClass == RetentionAbsorbed || ann.RetentionClass == RetentionContinued {
			absorbedCount++
		}
	}

	if len(announcements) > 0 {
		if float64(tightShipCount)/float64(len(announcements)) > 0.7 {
			patterns.QualitySignals = append(patterns.QualitySignals, "Consistently tight information control")
		}
		if float64(absorbedCount)/float64(len(announcements)) > 0.7 {
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
	sb.WriteString(fmt.Sprintf("**Period:** %s to %s  \n\n", output.PeriodStart.Format("2006-01-02"), output.PeriodEnd.Format("2006-01-02")))

	// MQS Score Summary
	sb.WriteString("## Management Quality Score\n\n")
	sb.WriteString(fmt.Sprintf("**Tier:** %s  \n", output.ManagementQualityScore.Tier))
	sb.WriteString(fmt.Sprintf("**Composite Score:** %.2f  \n", output.ManagementQualityScore.CompositeScore))
	sb.WriteString(fmt.Sprintf("**Confidence:** %s  \n\n", output.ManagementQualityScore.Confidence))

	// Tier description
	switch output.ManagementQualityScore.Tier {
	case TierOperator:
		sb.WriteString("✅ **TIER 1 - OPERATOR**: Management demonstrates strong information integrity, ")
		sb.WriteString("institutional conviction in announcements, and consistent price retention. ")
		sb.WriteString("Communication is factual and guidance is reliable.\n\n")
	case TierHonestStruggler:
		sb.WriteString("⚠️ **TIER 2 - HONEST STRUGGLER**: Management maintains reasonable information integrity ")
		sb.WriteString("but may face operational challenges. Communication is honest but results may be mixed.\n\n")
	case TierPromoter:
		sb.WriteString("🚨 **TIER 3 - PROMOTER**: Significant concerns about information integrity, ")
		sb.WriteString("price retention, or communication style. Exercise caution with announcements.\n\n")
	}

	// Component Scores Table with Calculation Formula
	sb.WriteString("### Component Scores\n\n")
	sb.WriteString("| Component | Score | Weight | Contribution |\n")
	sb.WriteString("|-----------|-------|--------|-------------|\n")
	leakageContrib := output.ManagementQualityScore.LeakageScore * 0.33
	convictionContrib := output.ManagementQualityScore.ConvictionScore * 0.33
	retentionContrib := output.ManagementQualityScore.RetentionScore * 0.34
	sb.WriteString(fmt.Sprintf("| Leakage (Information Integrity) | %.2f | 33%% | %.2f × 0.33 = %.3f |\n",
		output.ManagementQualityScore.LeakageScore, output.ManagementQualityScore.LeakageScore, leakageContrib))
	sb.WriteString(fmt.Sprintf("| Conviction (Volume/Price Alignment) | %.2f | 33%% | %.2f × 0.33 = %.3f |\n",
		output.ManagementQualityScore.ConvictionScore, output.ManagementQualityScore.ConvictionScore, convictionContrib))
	sb.WriteString(fmt.Sprintf("| Retention (Price Sustainability) | %.2f | 34%% | %.2f × 0.34 = %.3f |\n",
		output.ManagementQualityScore.RetentionScore, output.ManagementQualityScore.RetentionScore, retentionContrib))
	sb.WriteString(fmt.Sprintf("| **Composite** | **%.2f** | **100%%** | **%.3f + %.3f + %.3f = %.3f** |\n\n",
		output.ManagementQualityScore.CompositeScore, leakageContrib, convictionContrib, retentionContrib,
		leakageContrib+convictionContrib+retentionContrib))

	// Calculation Method
	sb.WriteString("**Calculation Method:**\n")
	sb.WriteString("```\n")
	sb.WriteString("Composite = (Leakage × 0.33) + (Conviction × 0.33) + (Retention × 0.34)\n")
	sb.WriteString("```\n\n")

	// Leakage Summary - include score in heading
	sb.WriteString(fmt.Sprintf("## Information Integrity (Leakage Analysis) — Score: %.2f\n\n", output.ManagementQualityScore.LeakageScore))
	sb.WriteString("*Measures abnormal price movement in the 5 days before announcements. ")
	sb.WriteString("High pre-drift suggests information leakage or insider activity.*\n\n")

	// Calculation explanation - show how Leakage Ratio is derived
	sb.WriteString("**Calculation:**\n")
	sb.WriteString(fmt.Sprintf("- Leakage Ratio = High Leakage Events / Total Analyzed = %d / %d = %.2f\n",
		output.LeakageSummary.HighLeakageCount, output.LeakageSummary.TotalAnalyzed, output.LeakageSummary.LeakageRatio))
	sb.WriteString(fmt.Sprintf("- Leakage Score = 1.0 - Leakage Ratio = 1.0 - %.2f = **%.2f**\n\n",
		output.LeakageSummary.LeakageRatio, output.ManagementQualityScore.LeakageScore))

	sb.WriteString(fmt.Sprintf("- **Total Analyzed:** %d announcements\n", output.LeakageSummary.TotalAnalyzed))
	sb.WriteString(fmt.Sprintf("- **High Leakage Events:** %d (%.1f%%)\n",
		output.LeakageSummary.HighLeakageCount,
		output.LeakageSummary.LeakageRatio*100))
	sb.WriteString(fmt.Sprintf("- **Tight Ship Events:** %d\n", output.LeakageSummary.TightShipCount))
	sb.WriteString(fmt.Sprintf("- **Average Pre-Drift:** %.1f%%\n\n", output.LeakageSummary.AveragePreDriftPct))

	if len(output.LeakageSummary.WorstLeakages) > 0 {
		sb.WriteString("### Notable Pre-Announcement Drift Events\n\n")
		sb.WriteString("| Date | Headline | Pre-Drift | Direction |\n")
		sb.WriteString("|------|----------|-----------|----------|\n")
		for _, leak := range output.LeakageSummary.WorstLeakages {
			headline := leak.Headline
			if len(headline) > 50 {
				headline = headline[:47] + "..."
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %+.1f%% | %s |\n",
				leak.Date, headline, leak.PreDriftPct, leak.Direction))
		}
		sb.WriteString("\n")
	}

	// Conviction Summary - include score in heading
	sb.WriteString(fmt.Sprintf("## Conviction Analysis — Score: %.2f\n\n", output.ManagementQualityScore.ConvictionScore))
	sb.WriteString("*Evaluates volume/price alignment on announcement days. ")
	sb.WriteString("Institutional conviction shows smart money validation; retail hype suggests speculative interest.*\n\n")

	// Event Type Definitions
	sb.WriteString("**Event Classifications:**\n")
	sb.WriteString("- **Institutional Conviction**: Price change > 2% AND volume > 3x average — indicates institutional investors backing the announcement\n")
	sb.WriteString("- **Retail Hype**: Price change > 5% AND volume < 2x average — speculative interest without institutional backing\n")
	sb.WriteString("- **Low Interest**: Price change < 1% AND volume < 1x average — minimal market reaction\n\n")

	// Calculation explanation
	totalConvictionEvents := output.ConvictionSummary.InstitutionalCount + output.ConvictionSummary.RetailHypeCount + output.ConvictionSummary.LowInterestCount
	sb.WriteString("**Calculation:**\n")
	sb.WriteString(fmt.Sprintf("- Conviction Score = Institutional Events / Total Events = %d / %d = **%.2f**\n\n",
		output.ConvictionSummary.InstitutionalCount, totalConvictionEvents, output.ManagementQualityScore.ConvictionScore))

	sb.WriteString(fmt.Sprintf("- **Institutional Conviction Events:** %d (%.1f%%)\n",
		output.ConvictionSummary.InstitutionalCount,
		output.ConvictionSummary.InstitutionalRatio*100))
	sb.WriteString(fmt.Sprintf("- **Retail Hype Events:** %d\n", output.ConvictionSummary.RetailHypeCount))
	sb.WriteString(fmt.Sprintf("- **Low Interest Events:** %d\n", output.ConvictionSummary.LowInterestCount))
	sb.WriteString(fmt.Sprintf("- **Average Volume Ratio:** %.1fx\n\n", output.ConvictionSummary.AverageVolumeRatio))

	// Retention Summary - include score in heading
	sb.WriteString(fmt.Sprintf("## Price Retention Analysis — Score: %.2f\n\n", output.ManagementQualityScore.RetentionScore))
	sb.WriteString("*Measures how well prices hold 10 days after announcements. ")
	sb.WriteString("High retention indicates market believes in sustainability; fading suggests 'sell the news' behavior.*\n\n")

	// Calculation explanation
	retentionHeldCount := output.RetentionSummary.AbsorbedCount + output.RetentionSummary.ContinuedCount
	totalRetentionEvents := retentionHeldCount + output.RetentionSummary.SoldNewsCount + output.RetentionSummary.ReversedCount
	sb.WriteString("**Calculation:** `Retention Score = (Absorbed + Continued) / Total Events`\n")
	sb.WriteString(fmt.Sprintf("= (%d + %d) / %d = **%.2f**\n\n",
		output.RetentionSummary.AbsorbedCount, output.RetentionSummary.ContinuedCount, totalRetentionEvents, output.ManagementQualityScore.RetentionScore))

	sb.WriteString(fmt.Sprintf("- **Absorbed (Price Held):** %d\n", output.RetentionSummary.AbsorbedCount))
	sb.WriteString(fmt.Sprintf("- **Continued (Price Increased):** %d\n", output.RetentionSummary.ContinuedCount))
	sb.WriteString(fmt.Sprintf("- **Sold the News:** %d\n", output.RetentionSummary.SoldNewsCount))
	sb.WriteString(fmt.Sprintf("- **Reversed:** %d\n", output.RetentionSummary.ReversedCount))
	sb.WriteString(fmt.Sprintf("- **Retention Rate:** %.1f%%\n\n", output.RetentionSummary.RetentionRate*100))

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

		sb.WriteString("| Date | Headline | Day-Of Δ | Day+10 Δ | Retention | Volume | Link |\n")
		sb.WriteString("|------|----------|----------|----------|-----------|--------|------|\n")

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

			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s | %s |\n",
				ann.Date, headline, dayOfStr, day10Str, retentionStr, volumeStr, linkStr))
		}
		sb.WriteString("\n")

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
