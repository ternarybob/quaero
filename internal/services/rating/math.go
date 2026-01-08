package rating

import (
	"math"
	"sort"
	"time"
)

// CAGR calculates Compound Annual Growth Rate
func CAGR(start, end float64, years float64) float64 {
	if start <= 0 || years <= 0 {
		return 0
	}
	return math.Pow(end/start, 1/years) - 1
}

// Stddev calculates standard deviation
func Stddev(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	mean := 0.0
	for _, v := range values {
		mean += v
	}
	mean /= float64(len(values))

	variance := 0.0
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(values))

	return math.Sqrt(variance)
}

// Mean calculates the arithmetic mean
func Mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// DailyReturns calculates daily log returns from prices
func DailyReturns(prices []PriceBar) []float64 {
	if len(prices) < 2 {
		return nil
	}

	returns := make([]float64, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		if prices[i-1].Close > 0 {
			returns[i-1] = math.Log(prices[i].Close / prices[i-1].Close)
		}
	}
	return returns
}

// PriceWindow extracts prices around a date
func PriceWindow(prices []PriceBar, date time.Time, before, after int) []PriceBar {
	if len(prices) == 0 {
		return nil
	}

	// Sort by date
	sorted := make([]PriceBar, len(prices))
	copy(sorted, prices)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Date.Before(sorted[j].Date)
	})

	// Find index closest to date
	idx := -1
	for i, p := range sorted {
		if !p.Date.Before(date) {
			idx = i
			break
		}
	}
	if idx == -1 {
		idx = len(sorted) - 1
	}

	start := idx - before
	if start < 0 {
		start = 0
	}
	end := idx + after + 1
	if end > len(sorted) {
		end = len(sorted)
	}

	return sorted[start:end]
}

// GetPriceAtDate finds the closing price at or near a specific date
func GetPriceAtDate(prices []PriceBar, date time.Time) float64 {
	if len(prices) == 0 {
		return 0
	}

	// Sort by date
	sorted := make([]PriceBar, len(prices))
	copy(sorted, prices)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Date.Before(sorted[j].Date)
	})

	// Find exact match or closest before
	for i := len(sorted) - 1; i >= 0; i-- {
		if !sorted[i].Date.After(date) {
			return sorted[i].Close
		}
	}

	// Return first available if all prices are after date
	return sorted[0].Close
}

// GetPriceAfterDate finds the closing price after a specific date (with days offset)
func GetPriceAfterDate(prices []PriceBar, date time.Time, daysAfter int) float64 {
	targetDate := date.AddDate(0, 0, daysAfter)
	if len(prices) == 0 {
		return 0
	}

	// Sort by date
	sorted := make([]PriceBar, len(prices))
	copy(sorted, prices)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Date.Before(sorted[j].Date)
	})

	// Find first price on or after target date
	for _, p := range sorted {
		if !p.Date.Before(targetDate) {
			return p.Close
		}
	}

	// Return last available if no prices after target
	return sorted[len(sorted)-1].Close
}

// RollingVolatility calculates volatility over a rolling window
func RollingVolatility(returns []float64, window int) []float64 {
	if len(returns) < window {
		return nil
	}

	result := make([]float64, len(returns)-window+1)
	for i := 0; i <= len(returns)-window; i++ {
		windowReturns := returns[i : i+window]
		result[i] = Stddev(windowReturns)
	}
	return result
}

// ClampFloat64 constrains a value to a range
func ClampFloat64(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
