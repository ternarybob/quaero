package signals

// RSComputer computes relative strength vs benchmark
type RSComputer struct {
	benchmarkReturns map[string]float64 // period -> benchmark return %
}

// NewRSComputer creates a new RS computer
func NewRSComputer() *RSComputer {
	return &RSComputer{
		benchmarkReturns: make(map[string]float64),
	}
}

// SetBenchmarkReturns sets the benchmark returns for RS calculation
func (c *RSComputer) SetBenchmarkReturns(returns map[string]float64) {
	c.benchmarkReturns = returns
}

// Compute calculates RS for a ticker
// stockReturns should contain "3m" and "6m" keys with percentage returns
func (c *RSComputer) Compute(stockReturns map[string]float64) RSSignal {
	// Get benchmark returns (default to 0 if not set)
	bench3M := c.benchmarkReturns["3m"]
	bench6M := c.benchmarkReturns["6m"]

	// Get stock returns
	stock3M := stockReturns["3m"]
	stock6M := stockReturns["6m"]

	// Calculate RS ratios
	// RS = (1 + Stock Return) / (1 + Benchmark Return)
	rs3M := calculateRS(stock3M, bench3M)
	rs6M := calculateRS(stock6M, bench6M)

	// Estimate percentile rank based on RS value
	rsRank := estimateRank(rs3M)

	return RSSignal{
		VsXJO3M:          round(rs3M, 2),
		VsXJO6M:          round(rs6M, 2),
		RSRankPercentile: rsRank,
	}
}

// ComputeFromRaw calculates RS directly from TickerRaw data
func (c *RSComputer) ComputeFromRaw(raw TickerRaw) RSSignal {
	stockReturns := map[string]float64{
		"3m": raw.Price.Return12WPct, // 12 weeks ≈ 3 months
		"6m": raw.Price.Return26WPct, // 26 weeks ≈ 6 months
	}
	return c.Compute(stockReturns)
}

// calculateRS computes the relative strength ratio
func calculateRS(stockReturnPct, benchReturnPct float64) float64 {
	// Convert percentages to decimals
	stockMult := 1 + stockReturnPct/100.0
	benchMult := 1 + benchReturnPct/100.0

	// Avoid division by zero or very small numbers
	if benchMult < 0.1 {
		benchMult = 0.1
	}

	return stockMult / benchMult
}

// estimateRank estimates percentile rank from RS value
// This is an approximation - in a real system, you'd compare against the full universe
func estimateRank(rs float64) int {
	// RS of 1.0 = 50th percentile (matching benchmark)
	// RS of 1.2 = ~75th percentile (outperforming by 20%)
	// RS of 0.8 = ~25th percentile (underperforming by 20%)

	switch {
	case rs >= 1.4:
		return 95
	case rs >= 1.3:
		return 90
	case rs >= 1.2:
		return 80
	case rs >= 1.15:
		return 72
	case rs >= 1.1:
		return 65
	case rs >= 1.05:
		return 57
	case rs >= 1.0:
		return 50
	case rs >= 0.95:
		return 43
	case rs >= 0.9:
		return 35
	case rs >= 0.85:
		return 28
	case rs >= 0.8:
		return 20
	case rs >= 0.7:
		return 10
	default:
		return 5
	}
}
