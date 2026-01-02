package signals

import "math"

// sigmoid applies the logistic function
func sigmoid(x float64) float64 {
	return 1.0 / (1.0 + math.Exp(-x))
}

// clamp restricts a value to a range
func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// round rounds to specified decimal places
func round(value float64, places int) float64 {
	mult := math.Pow(10, float64(places))
	return math.Round(value*mult) / mult
}

// abs returns the absolute value of a float64
func abs(x float64) float64 {
	return math.Abs(x)
}

// max returns the maximum of two float64 values
func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// min returns the minimum of two float64 values
func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// max3 returns the maximum of three float64 values
func max3(a, b, c float64) float64 {
	return maxFloat(a, maxFloat(b, c))
}

// sma calculates the simple moving average of the last n values
func sma(values []float64, n int) float64 {
	if len(values) < n || n <= 0 {
		return 0
	}
	sum := 0.0
	for i := len(values) - n; i < len(values); i++ {
		sum += values[i]
	}
	return sum / float64(n)
}

// avg calculates the average of all values
func avg(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// stddev calculates the sample standard deviation
func stddev(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	mean := avg(values)
	sumSquares := 0.0
	for _, v := range values {
		diff := v - mean
		sumSquares += diff * diff
	}
	return math.Sqrt(sumSquares / float64(len(values)-1))
}

// zscore calculates the z-score of a value relative to a sample
func zscore(value float64, values []float64) float64 {
	mean := avg(values)
	sd := stddev(values)
	if sd == 0 {
		return 0
	}
	return (value - mean) / sd
}

// pctChange calculates the percentage change from old to new
func pctChange(old, newVal float64) float64 {
	if old == 0 {
		return 0
	}
	return ((newVal - old) / old) * 100
}

// returnPct calculates the return over the last n days
func returnPct(prices []float64, days int) float64 {
	n := len(prices)
	if n < days+1 || days <= 0 {
		return 0
	}
	return pctChange(prices[n-days-1], prices[n-1])
}
