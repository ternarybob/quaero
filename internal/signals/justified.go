package signals

// JustifiedReturnComputer calculates justified returns based on business fundamentals
type JustifiedReturnComputer struct{}

// NewJustifiedReturnComputer creates a new justified return computer
func NewJustifiedReturnComputer() *JustifiedReturnComputer {
	return &JustifiedReturnComputer{}
}

// Compute calculates justified return metrics
func (c *JustifiedReturnComputer) Compute(raw TickerRaw, pbas PBASSignal) JustifiedReturnSignal {
	bm := pbas.BusinessMomentum

	// Map business momentum to expected return
	// Higher business momentum should justify higher price appreciation
	expectedReturn := c.mapMomentumToReturn(bm)

	// Actual return
	actualReturn := raw.Price.Return52WPct

	// Divergence (positive = price ahead, negative = price behind)
	divergence := actualReturn - expectedReturn

	// Interpretation
	interpretation := c.interpretDivergence(divergence)

	return JustifiedReturnSignal{
		Expected12MPct: round(expectedReturn, 1),
		Actual12MPct:   round(actualReturn, 1),
		DivergencePct:  round(divergence, 1),
		Interpretation: interpretation,
	}
}

// mapMomentumToReturn converts business momentum to expected 12M return
func (c *JustifiedReturnComputer) mapMomentumToReturn(bm float64) float64 {
	// Business momentum ranges roughly from -0.3 to +0.3
	// Map to expected returns:
	// BM > 0.20  → 25% expected return (exceptional business performance)
	// BM 0.15-0.20 → 20%
	// BM 0.10-0.15 → 15%
	// BM 0.05-0.10 → 10%
	// BM 0-0.05 → 5%
	// BM -0.05-0 → 0%
	// BM < -0.05 → -5%

	switch {
	case bm > 0.20:
		return 25.0
	case bm > 0.15:
		return 20.0
	case bm > 0.10:
		return 15.0
	case bm > 0.05:
		return 10.0
	case bm > 0:
		return 5.0
	case bm > -0.05:
		return 0.0
	default:
		return -5.0
	}
}

// interpretDivergence determines the interpretation based on divergence
func (c *JustifiedReturnComputer) interpretDivergence(divergence float64) string {
	// Divergence is actual - expected
	// Positive = price has moved more than business justifies (ahead)
	// Negative = price has moved less than business justifies (behind)

	switch {
	case divergence > 15:
		return "price_ahead" // Significantly overvalued by price action
	case divergence > 5:
		return "slightly_ahead" // Mildly ahead
	case divergence > -5:
		return "aligned" // Within reasonable range
	case divergence > -15:
		return "slightly_behind" // Mildly behind
	default:
		return "price_behind" // Significantly undervalued by price action
	}
}

// Interpretation constants
const (
	JustifiedAligned        = "aligned"
	JustifiedSlightlyAhead  = "slightly_ahead"
	JustifiedPriceAhead     = "price_ahead"
	JustifiedSlightlyBehind = "slightly_behind"
	JustifiedPriceBehind    = "price_behind"
)
