package announcements

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ClassifyRelevance determines the relevance category of an announcement
// based on keywords in the headline and type.
// Returns: category (HIGH, MEDIUM, LOW, NOISE) and reason string
func ClassifyRelevance(headline, annType string, priceSensitive bool) (category, reason string) {
	// HIGH: Price-sensitive or major events
	if priceSensitive {
		return "HIGH", "Price-sensitive announcement"
	}

	typeUpper := strings.ToUpper(annType)
	headlineUpper := strings.ToUpper(headline)

	// HIGH types - major corporate events
	highKeywords := []string{
		"TAKEOVER", "ACQUISITION", "MERGER", "DISPOSAL",
		"DIVIDEND", "CAPITAL RAISING", "PLACEMENT", "SPP", "RIGHTS ISSUE",
		"FINANCIAL REPORT", "HALF YEAR", "FULL YEAR", "ANNUAL REPORT",
		"QUARTERLY", "PRELIMINARY FINAL", "EARNINGS",
		"GUIDANCE", "FORECAST", "OUTLOOK",
		"ASSET SALE", "DIVESTMENT",
	}

	for _, kw := range highKeywords {
		if strings.Contains(typeUpper, kw) || strings.Contains(headlineUpper, kw) {
			return "HIGH", fmt.Sprintf("Contains '%s'", kw)
		}
	}

	// MEDIUM types - governance and significant operational
	mediumKeywords := []string{
		"DIRECTOR", "CHAIRMAN", "CEO", "CFO", "MANAGING DIRECTOR",
		"APPOINTMENT", "RESIGNATION", "RETIREMENT",
		"AGM", "EGM", "GENERAL MEETING",
		"CONTRACT", "AGREEMENT", "PARTNERSHIP", "JOINT VENTURE",
		"EXPLORATION", "DRILLING", "RESOURCE", "RESERVE",
		"REGULATORY", "APPROVAL", "LICENSE", "PERMIT",
	}

	for _, kw := range mediumKeywords {
		if strings.Contains(typeUpper, kw) || strings.Contains(headlineUpper, kw) {
			return "MEDIUM", fmt.Sprintf("Contains '%s'", kw)
		}
	}

	// LOW types - routine disclosures
	lowKeywords := []string{
		"PROGRESS REPORT", "UPDATE", "INVESTOR PRESENTATION",
		"DISCLOSURE", "CLEANSING", "STATEMENT",
		"APPENDIX", "SUBSTANTIAL HOLDER",
		"CHANGE OF ADDRESS", "COMPANY SECRETARY",
	}

	for _, kw := range lowKeywords {
		if strings.Contains(typeUpper, kw) || strings.Contains(headlineUpper, kw) {
			return "LOW", fmt.Sprintf("Routine disclosure: '%s'", kw)
		}
	}

	return "NOISE", "No material indicators found"
}

// IsRoutineAnnouncement checks if announcement is a standard administrative filing
// that should be excluded from signal/noise analysis.
func IsRoutineAnnouncement(headline string) (isRoutine bool, routineType string) {
	headlineUpper := strings.ToUpper(headline)

	// Ordered by specificity (more specific patterns first)
	routinePatterns := []struct {
		pattern     string
		routineType string
	}{
		{"NOTICE OF ANNUAL GENERAL MEETING", "AGM Notice"},
		{"NOTICE OF GENERAL MEETING", "Meeting Notice"},
		{"RESULTS OF MEETING", "Meeting Results"},
		{"PROPOSED ISSUE OF SECURITIES", "Securities Issue"},
		{"APPLICATION FOR QUOTATION OF SECURITIES", "Quotation Application"},
		{"APPLICATION FOR QUOTATION", "Quotation Application"},
		{"NOTIFICATION OF CESSATION OF SECURITIES", "Securities Cessation"},
		{"NOTIFICATION OF CESSATION", "Securities Cessation"},
		{"NOTIFICATION REGARDING UNQUOTED SECURITIES", "Unquoted Securities"},
		{"NOTIFICATION REGARDING UNQUOTED", "Unquoted Securities"},
		{"CHANGE OF DIRECTOR'S INTEREST NOTICE", "Director Interest Change"},
		{"CHANGE OF DIRECTORS INTEREST", "Director Interest Change"},
		{"APPENDIX 3Y", "Director Interest (3Y)"},
		{"APPENDIX 3X", "Initial Director Interest (3X)"},
		{"APPENDIX 3B", "New Issue (3B)"},
		{"APPENDIX 3G", "Issue Notification (3G)"},
		{"CLEANSING NOTICE", "Cleansing Notice"},
		{"CLEANSING STATEMENT", "Cleansing Notice"},
	}

	for _, p := range routinePatterns {
		if strings.Contains(headlineUpper, p.pattern) {
			return true, p.routineType
		}
	}
	return false, ""
}

// DetectTradingHalt checks if an announcement is a trading halt or reinstatement
func DetectTradingHalt(headline string) (isTradingHalt, isReinstatement bool) {
	headlineUpper := strings.ToUpper(headline)

	// Trading halt keywords
	haltKeywords := []string{
		"TRADING HALT",
		"VOLUNTARY SUSPENSION",
		"SUSPENSION FROM QUOTATION",
		"SUSPENDED FROM TRADING",
	}

	for _, kw := range haltKeywords {
		if strings.Contains(headlineUpper, kw) {
			return true, false
		}
	}

	// Reinstatement keywords
	reinstatementKeywords := []string{
		"REINSTATEMENT",
		"RESUMPTION OF TRADING",
		"TRADING RESUMES",
		"LIFTED SUSPENSION",
		"END OF SUSPENSION",
	}

	for _, kw := range reinstatementKeywords {
		if strings.Contains(headlineUpper, kw) {
			return false, true
		}
	}

	return false, false
}

// DetectDividendAnnouncement checks if an announcement is dividend-related
func DetectDividendAnnouncement(headline, annType string) bool {
	headlineUpper := strings.ToUpper(headline)
	typeUpper := strings.ToUpper(annType)

	dividendKeywords := []string{
		"DIVIDEND",
		"DRP", // Dividend Reinvestment Plan
		"DISTRIBUTION",
		"EX-DATE",
		"EX DATE",
		"RECORD DATE",
		"PAYMENT DATE",
		"FRANKING",
		"UNFRANKED",
		"FRANKED",
	}

	for _, kw := range dividendKeywords {
		if strings.Contains(headlineUpper, kw) || strings.Contains(typeUpper, kw) {
			return true
		}
	}

	return false
}

// CalculateSignalNoise determines the overall signal quality based on price/volume impact
func CalculateSignalNoise(ann RawAnnouncement, impact *PriceImpactData, isTradingHalt, isReinstatement bool) SignalNoiseResult {
	var rationale strings.Builder
	result := SignalNoiseResult{}

	// Check for routine announcements FIRST - these are excluded from signal analysis
	isRoutine, routineType := IsRoutineAnnouncement(ann.Headline)
	if isRoutine {
		return SignalNoiseResult{
			Rating:    SignalNoiseRoutine,
			Rationale: fmt.Sprintf("ROUTINE: Standard administrative filing (%s). Excluded from signal analysis - not correlated with price/volume movements.", routineType),
		}
	}

	// If no price data available, base rating on announcement characteristics only
	if impact == nil {
		if ann.PriceSensitive {
			result.Rating = SignalNoiseModerate
			result.Rationale = "Price-sensitive announcement (no price data available for impact analysis)"
			return result
		}
		if isTradingHalt {
			result.Rating = SignalNoiseLow
			result.Rationale = "Trading halt announced (no price data available for impact analysis)"
			return result
		}
		result.Rating = SignalNoiseNone
		result.Rationale = "No price data available for impact analysis"
		return result
	}

	// Calculate absolute price change for comparison
	absPriceChange := impact.ChangePercent
	if absPriceChange < 0 {
		absPriceChange = -absPriceChange
	}

	// Determine direction description
	direction := "no change"
	if impact.ChangePercent > 0.1 {
		direction = fmt.Sprintf("+%.1f%% increase", impact.ChangePercent)
	} else if impact.ChangePercent < -0.1 {
		direction = fmt.Sprintf("%.1f%% decrease", impact.ChangePercent)
	}

	// Volume analysis
	volumeDesc := "normal volume"
	if impact.VolumeChangeRatio >= 2.0 {
		volumeDesc = fmt.Sprintf("%.1fx volume spike", impact.VolumeChangeRatio)
	} else if impact.VolumeChangeRatio >= 1.5 {
		volumeDesc = fmt.Sprintf("%.1fx elevated volume", impact.VolumeChangeRatio)
	} else if impact.VolumeChangeRatio <= 0.5 {
		volumeDesc = fmt.Sprintf("%.1fx reduced volume", impact.VolumeChangeRatio)
	}

	// Add pre-announcement drift info if significant
	if impact.HasSignificantPreDrift {
		rationale.WriteString(fmt.Sprintf("PRE-ANNOUNCEMENT: %s ", impact.PreDriftInterpretation))
	}

	// HIGH_SIGNAL: Significant market impact
	if absPriceChange >= 3.0 || impact.VolumeChangeRatio >= 2.0 {
		rationale.WriteString(fmt.Sprintf("HIGH SIGNAL: Significant market reaction with %s and %s. ", direction, volumeDesc))
		if ann.PriceSensitive {
			rationale.WriteString("Confirmed price-sensitive announcement. ")
		} else {
			result.IsAnomaly = true
			result.AnomalyType = "UNEXPECTED_REACTION"
			rationale.WriteString("ANOMALY: Non-price-sensitive announcement triggered significant market reaction. ")
		}
		if absPriceChange >= 5.0 {
			rationale.WriteString("Price movement exceeds 5% threshold indicating major market reassessment.")
		} else if impact.VolumeChangeRatio >= 3.0 {
			rationale.WriteString("Exceptional volume indicates strong investor interest.")
		}
		result.Rating = SignalNoiseHigh
		result.Rationale = rationale.String()
		return result
	}

	// MODERATE_SIGNAL: Notable market reaction
	if absPriceChange >= 1.5 || impact.VolumeChangeRatio >= 1.5 {
		rationale.WriteString(fmt.Sprintf("MODERATE SIGNAL: Notable market reaction with %s and %s. ", direction, volumeDesc))
		if ann.PriceSensitive {
			rationale.WriteString("Price-sensitive flag indicates company deemed this material. ")
		} else if !isTradingHalt && !isReinstatement {
			result.IsAnomaly = true
			result.AnomalyType = "UNEXPECTED_REACTION"
			rationale.WriteString("Note: Non-price-sensitive announcement showed unexpected market response. ")
		}
		if isTradingHalt || isReinstatement {
			rationale.WriteString("Associated with trading halt activity. ")
		}
		result.Rating = SignalNoiseModerate
		result.Rationale = rationale.String()
		return result
	}

	// LOW_SIGNAL: Minimal but detectable market reaction
	if absPriceChange >= 0.5 || impact.VolumeChangeRatio >= 1.2 {
		rationale.WriteString(fmt.Sprintf("LOW SIGNAL: Minor market reaction with %s and %s. ", direction, volumeDesc))
		if ann.PriceSensitive {
			result.IsAnomaly = true
			result.AnomalyType = "NO_REACTION"
			rationale.WriteString("ANOMALY: Price-sensitive flag but market showed limited reaction. ")
		}
		result.Rating = SignalNoiseLow
		result.Rationale = rationale.String()
		return result
	}

	// NOISE: No meaningful price/volume impact
	rationale.WriteString(fmt.Sprintf("NOISE: No meaningful market impact - %s with %s. ", direction, volumeDesc))
	if isTradingHalt {
		rationale.WriteString("Trading halt with no subsequent price movement indicates non-material purpose. ")
	} else if isReinstatement {
		rationale.WriteString("Reinstatement with no price change suggests halt was procedural. ")
	} else if ann.PriceSensitive {
		result.IsAnomaly = true
		result.AnomalyType = "NO_REACTION"
		rationale.WriteString("ANOMALY: Price-sensitive announcement but market showed NO reaction - verify announcement accuracy. ")
	} else {
		rationale.WriteString("Announcement had no measurable effect on price or volume. ")
	}
	result.Rating = SignalNoiseNone
	result.Rationale = rationale.String()
	return result
}

// CalculatePriceImpact calculates stock price movement around an announcement date
func CalculatePriceImpact(announcementDate time.Time, prices []PriceBar) *PriceImpactData {
	if len(prices) == 0 {
		return nil
	}

	// Build date-to-price map for O(1) lookups
	priceMap := make(map[string]PriceBar)
	for _, p := range prices {
		priceMap[p.Date.Format("2006-01-02")] = p
	}

	// Normalize announcement date to date only
	annDateStr := announcementDate.Format("2006-01-02")

	// Find price on announcement date (or closest trading day after)
	var priceOnDate PriceBar
	foundOnDate := false

	if p, ok := priceMap[annDateStr]; ok {
		priceOnDate = p
		foundOnDate = true
	} else {
		// Announcement might be on weekend/holiday - find next trading day
		for i := 1; i <= 5; i++ {
			checkDate := announcementDate.AddDate(0, 0, i).Format("2006-01-02")
			if p, ok := priceMap[checkDate]; ok {
				priceOnDate = p
				foundOnDate = true
				break
			}
		}
	}

	if !foundOnDate {
		return nil
	}

	// Find previous trading day's price
	var priceBefore PriceBar
	foundBefore := false
	for i := 1; i <= 10; i++ {
		checkDate := announcementDate.AddDate(0, 0, -i).Format("2006-01-02")
		if p, ok := priceMap[checkDate]; ok {
			priceBefore = p
			foundBefore = true
			break
		}
	}

	if !foundBefore {
		return nil
	}

	// Calculate volumes before announcement (5 trading days)
	volumeBefore := int64(0)
	volumeCount := 0
	for i := 1; i <= 15 && volumeCount < 5; i++ {
		checkDate := announcementDate.AddDate(0, 0, -i).Format("2006-01-02")
		if p, ok := priceMap[checkDate]; ok && p.Volume > 0 {
			volumeBefore += p.Volume
			volumeCount++
		}
	}
	if volumeCount > 0 {
		volumeBefore = volumeBefore / int64(volumeCount)
	}

	// Calculate volumes after announcement (5 trading days)
	volumeAfter := int64(0)
	volumeCount = 0
	for i := 0; i <= 15 && volumeCount < 5; i++ {
		checkDate := announcementDate.AddDate(0, 0, i).Format("2006-01-02")
		if p, ok := priceMap[checkDate]; ok && p.Volume > 0 {
			volumeAfter += p.Volume
			volumeCount++
		}
	}
	if volumeCount > 0 {
		volumeAfter = volumeAfter / int64(volumeCount)
	}

	// Calculate price change
	changePercent := 0.0
	if priceBefore.Close > 0 {
		changePercent = ((priceOnDate.Close - priceBefore.Close) / priceBefore.Close) * 100
	}

	// Calculate volume change ratio
	volumeChangeRatio := 1.0
	if volumeBefore > 0 {
		volumeChangeRatio = float64(volumeAfter) / float64(volumeBefore)
	}

	// Determine impact signal
	impactSignal := "MINIMAL"
	absChange := changePercent
	if absChange < 0 {
		absChange = -absChange
	}
	if absChange >= 3.0 || volumeChangeRatio >= 2.0 {
		impactSignal = "SIGNIFICANT"
	} else if absChange >= 1.5 || volumeChangeRatio >= 1.5 {
		impactSignal = "MODERATE"
	}

	// Calculate pre-announcement drift (T-5 to T-1)
	var priceT5, priceT1 PriceBar
	foundT5, foundT1 := false, false

	// Find T-1 (already have it as priceBefore)
	priceT1 = priceBefore
	foundT1 = true

	// Find T-5
	tradingDayCount := 0
	for i := 1; i <= 15 && tradingDayCount < 5; i++ {
		checkDate := announcementDate.AddDate(0, 0, -i).Format("2006-01-02")
		if p, ok := priceMap[checkDate]; ok {
			tradingDayCount++
			if tradingDayCount == 5 {
				priceT5 = p
				foundT5 = true
			}
		}
	}

	impact := &PriceImpactData{
		PriceBefore:       priceBefore.Close,
		PriceAfter:        priceOnDate.Close,
		ChangePercent:     changePercent,
		VolumeBefore:      volumeBefore,
		VolumeAfter:       volumeAfter,
		VolumeChangeRatio: volumeChangeRatio,
		ImpactSignal:      impactSignal,
	}

	// Add pre-announcement analysis if we have T-5 data
	if foundT5 && foundT1 && priceT5.Close > 0 {
		preDrift := ((priceT1.Close - priceT5.Close) / priceT5.Close) * 100
		impact.PreAnnouncementDrift = preDrift
		impact.PreAnnouncementPriceT5 = priceT5.Close
		impact.PreAnnouncementPriceT1 = priceT1.Close

		absDrift := preDrift
		if absDrift < 0 {
			absDrift = -absDrift
		}

		if absDrift >= 2.0 {
			impact.HasSignificantPreDrift = true
			if preDrift > 0 {
				impact.PreDriftInterpretation = fmt.Sprintf("Price drifted +%.1f%% in week before announcement - possible information leakage or anticipation", preDrift)
			} else {
				impact.PreDriftInterpretation = fmt.Sprintf("Price drifted %.1f%% in week before announcement - potential early positioning or concern", preDrift)
			}
		}
	}

	return impact
}

// appendixBasePattern matches "APPENDIX 3X", "APPENDIX 3Y", etc.
var appendixBasePattern = regexp.MustCompile(`APPENDIX\s+\d+[A-Z]`)

// DeduplicateAnnouncements consolidates same-day announcements with similar headlines
func DeduplicateAnnouncements(announcements []RawAnnouncement) ([]RawAnnouncement, DeduplicationStats) {
	stats := DeduplicationStats{TotalBefore: len(announcements)}

	if len(announcements) == 0 {
		return announcements, stats
	}

	// Group by date
	byDate := make(map[string][]RawAnnouncement)
	for _, ann := range announcements {
		dateKey := ann.Date.Format("2006-01-02")
		byDate[dateKey] = append(byDate[dateKey], ann)
	}

	var result []RawAnnouncement

	for dateKey, dayAnnouncements := range byDate {
		// Within each day, find similar headline groups
		used := make(map[int]bool)

		for i := 0; i < len(dayAnnouncements); i++ {
			if used[i] {
				continue
			}

			// Start a new group with this announcement
			group := []RawAnnouncement{dayAnnouncements[i]}
			used[i] = true

			// Find all similar announcements on same day
			for j := i + 1; j < len(dayAnnouncements); j++ {
				if used[j] {
					continue
				}
				if areSimilarHeadlines(dayAnnouncements[i].Headline, dayAnnouncements[j].Headline) {
					group = append(group, dayAnnouncements[j])
					used[j] = true
				}
			}

			// Keep only one representative (first one)
			result = append(result, group[0])

			// Track groups with duplicates
			if len(group) > 1 {
				headlines := make([]string, len(group))
				for k, a := range group {
					headlines[k] = a.Headline
				}
				date, _ := time.Parse("2006-01-02", dateKey)
				stats.Groups = append(stats.Groups, DeduplicationGroup{
					Date:      date,
					Headlines: headlines,
					Count:     len(group),
				})
			}
		}
	}

	stats.TotalAfter = len(result)
	stats.DuplicatesFound = stats.TotalBefore - stats.TotalAfter

	// Sort result by date descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].Date.After(result[j].Date)
	})

	return result, stats
}

// areSimilarHeadlines checks if two headlines should be considered duplicates
func areSimilarHeadlines(h1, h2 string) bool {
	// Exact match
	if h1 == h2 {
		return true
	}

	// Normalized match (removes trailing ticker code)
	norm1 := normalizeHeadline(h1)
	norm2 := normalizeHeadline(h2)
	if norm1 == norm2 {
		return true
	}

	// Appendix pattern: "Appendix 3Y XX" variants all match
	base1 := getAppendixBase(h1)
	base2 := getAppendixBase(h2)
	if base1 != "" && base2 != "" && base1 == base2 {
		return true
	}

	return false
}

// normalizeHeadline removes trailing ticker codes and whitespace for comparison
func normalizeHeadline(headline string) string {
	h := strings.TrimSpace(headline)
	// Remove trailing " - CODE" pattern (e.g., "Proposed issue of securities - EXR")
	if idx := strings.LastIndex(h, " - "); idx > 0 {
		suffix := strings.TrimSpace(h[idx+3:])
		if len(suffix) >= 2 && len(suffix) <= 4 && isAllUpperAlpha(suffix) {
			h = strings.TrimSpace(h[:idx])
		}
	}
	return strings.ToUpper(h)
}

// isAllUpperAlpha checks if string is all uppercase letters
func isAllUpperAlpha(s string) bool {
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return len(s) > 0
}

// getAppendixBase extracts the appendix base type
func getAppendixBase(headline string) string {
	headlineUpper := strings.ToUpper(headline)
	match := appendixBasePattern.FindString(headlineUpper)
	return match
}

// ProcessAnnouncements applies all classification and analysis to raw announcements
func ProcessAnnouncements(raw []RawAnnouncement, prices []PriceBar) ([]ProcessedAnnouncement, ProcessingSummary, DeduplicationStats) {
	// Deduplicate first
	dedupedRaw, dedupStats := DeduplicateAnnouncements(raw)

	var processed []ProcessedAnnouncement
	var summary ProcessingSummary

	for _, ann := range dedupedRaw {
		// Classify relevance
		category, reason := ClassifyRelevance(ann.Headline, ann.Type, ann.PriceSensitive)

		// Detect special types
		isTradingHalt, isReinstatement := DetectTradingHalt(ann.Headline)
		isDividend := DetectDividendAnnouncement(ann.Headline, ann.Type)
		isRoutine, routineType := IsRoutineAnnouncement(ann.Headline)

		// Calculate price impact
		var priceImpact *PriceImpactData
		if len(prices) > 0 {
			priceImpact = CalculatePriceImpact(ann.Date, prices)
		}

		// Calculate signal-to-noise
		signalResult := CalculateSignalNoise(ann, priceImpact, isTradingHalt, isReinstatement)

		proc := ProcessedAnnouncement{
			Date:                   ann.Date,
			Headline:               ann.Headline,
			Type:                   ann.Type,
			PDFURL:                 ann.PDFURL,
			DocumentKey:            ann.DocumentKey,
			PriceSensitive:         ann.PriceSensitive,
			RelevanceCategory:      category,
			RelevanceReason:        reason,
			SignalNoiseRating:      signalResult.Rating,
			SignalNoiseRationale:   signalResult.Rationale,
			PriceImpact:            priceImpact,
			IsTradingHalt:          isTradingHalt,
			IsReinstatement:        isReinstatement,
			IsDividendAnnouncement: isDividend,
			IsRoutine:              isRoutine,
			RoutineType:            routineType,
			IsAnomaly:              signalResult.IsAnomaly,
			AnomalyType:            signalResult.AnomalyType,
		}

		processed = append(processed, proc)

		// Update summary counts
		summary.TotalCount++
		switch category {
		case "HIGH":
			summary.HighRelevanceCount++
		case "MEDIUM":
			summary.MediumRelevanceCount++
		case "LOW":
			summary.LowRelevanceCount++
		default:
			summary.NoiseCount++
		}
		switch signalResult.Rating {
		case SignalNoiseHigh:
			summary.HighSignalCount++
		case SignalNoiseModerate:
			summary.ModerateSignalCount++
		case SignalNoiseLow:
			summary.LowSignalCount++
		case SignalNoiseRoutine:
			summary.RoutineCount++
		}
		if signalResult.IsAnomaly {
			summary.AnomalyCount++
		}
	}

	// Populate Announcements in summary
	summary.Announcements = processed

	// Calculate MQS scores
	if summary.TotalCount > 0 {
		signalCount := summary.HighSignalCount + summary.ModerateSignalCount
		noiseCount := summary.NoiseCount + summary.RoutineCount
		signalToNoiseRatio := 0.0
		if noiseCount > 0 {
			signalToNoiseRatio = float64(signalCount) / float64(noiseCount)
		} else if signalCount > 0 {
			signalToNoiseRatio = 1.0 // All signal, no noise
		}
		summary.MQSScores = &MQSScores{
			SignalToNoiseRatio: signalToNoiseRatio,
			HighSignalCount:    summary.HighSignalCount,
			RoutineCount:       summary.RoutineCount,
		}
	}

	return processed, summary, dedupStats
}
