package rating

import (
	"fmt"
	"sort"
)

// CalculatePPS calculates the Price Progression Score.
// Score: 0.0 to 1.0
//
// Measures how well positive price movements are retained after announcements.
// Higher retention = better price progression quality
//
// Process:
// 1. Find price-sensitive announcements
// 2. For each, measure price before and after (e.g., 5 days)
// 3. Calculate retention percentage
// 4. Average all retention values
func CalculatePPS(announcements []Announcement, prices []PriceBar) PPSResult {
	if len(announcements) == 0 || len(prices) == 0 {
		return PPSResult{
			Score:        0.5, // Neutral if no data
			EventDetails: nil,
			Reasoning:    "Neutral: Insufficient data for price progression analysis",
		}
	}

	// Sort prices by date
	sortedPrices := make([]PriceBar, len(prices))
	copy(sortedPrices, prices)
	sort.Slice(sortedPrices, func(i, j int) bool {
		return sortedPrices[i].Date.Before(sortedPrices[j].Date)
	})

	// Find price-sensitive announcements that had positive initial reaction
	var events []PPSEventDetail
	for _, ann := range announcements {
		if !ann.IsPriceSensitive {
			continue
		}

		priceBefore := GetPriceAtDate(sortedPrices, ann.Date.AddDate(0, 0, -1))
		priceDay0 := GetPriceAtDate(sortedPrices, ann.Date)
		priceAfter := GetPriceAfterDate(sortedPrices, ann.Date, 5) // 5 trading days after

		if priceBefore <= 0 || priceDay0 <= 0 || priceAfter <= 0 {
			continue
		}

		// Only consider announcements with positive initial reaction
		initialMove := (priceDay0 - priceBefore) / priceBefore
		if initialMove <= 0.02 { // Minimum 2% move
			continue
		}

		// Calculate retention: how much of the move was kept after 5 days
		totalMove := priceAfter - priceBefore
		retention := 0.0
		if initialMove > 0 {
			retention = totalMove / (initialMove * priceBefore)
			retention = ClampFloat64(retention, 0, 1)
		}

		events = append(events, PPSEventDetail{
			Date:         ann.Date,
			Headline:     ann.Headline,
			PriceBefore:  priceBefore,
			PriceAfter:   priceAfter,
			RetentionPct: retention * 100,
		})
	}

	// Calculate average retention
	score := 0.5 // Default neutral
	if len(events) > 0 {
		totalRetention := 0.0
		for _, e := range events {
			totalRetention += e.RetentionPct / 100
		}
		score = ClampFloat64(totalRetention/float64(len(events)), 0, 1)
	}

	// Build reasoning
	var reasoning string
	if len(events) == 0 {
		reasoning = "Neutral: No significant price-sensitive events found"
	} else if score >= 0.7 {
		reasoning = fmt.Sprintf("Strong retention: %.0f%% average across %d events",
			score*100, len(events))
	} else if score >= 0.4 {
		reasoning = fmt.Sprintf("Moderate retention: %.0f%% average across %d events",
			score*100, len(events))
	} else {
		reasoning = fmt.Sprintf("Weak retention: %.0f%% average across %d events",
			score*100, len(events))
	}

	return PPSResult{
		Score:        score,
		EventDetails: events,
		Reasoning:    reasoning,
	}
}
