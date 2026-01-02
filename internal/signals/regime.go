package signals

// RegimeClassifier classifies price action regimes
type RegimeClassifier struct{}

// NewRegimeClassifier creates a new regime classifier
func NewRegimeClassifier() *RegimeClassifier {
	return &RegimeClassifier{}
}

// Classify determines the current regime for a ticker
func (c *RegimeClassifier) Classify(raw TickerRaw) RegimeSignal {
	price := raw.Price
	vol := raw.Volume

	// EMA positions
	aboveEMA20 := price.Current > price.EMA20
	aboveEMA50 := price.Current > price.EMA50
	aboveEMA200 := price.Current > price.EMA200

	// EMA stack (bullish = 20 > 50 > 200)
	emaStackBullish := price.EMA20 > price.EMA50 && price.EMA50 > price.EMA200
	emaStackBearish := price.EMA20 < price.EMA50 && price.EMA50 < price.EMA200

	// Near 52-week levels
	distToHigh := 0.0
	if price.High52W > 0 {
		distToHigh = (price.High52W - price.Current) / price.High52W
	}
	near52WkHigh := distToHigh < 0.05 && distToHigh >= 0 // Within 5%

	distToLow := 0.0
	if price.Low52W > 0 {
		distToLow = (price.Current - price.Low52W) / price.Low52W
	}
	near52WkLow := distToLow < 0.10 && distToLow >= 0 // Within 10%

	// Volume confirmation
	volExpanding := vol.ZScore20 > 0.5
	volRising := vol.Trend5Dvs20D == "rising"

	// Returns
	return4W := price.Return4WPct
	return12W := price.Return12WPct

	// Classification logic (priority order)
	var regime RegimeType
	var confidence float64

	switch {
	// Breakout: Near highs with volume expansion and bullish EMAs
	case near52WkHigh && volExpanding && emaStackBullish:
		regime = RegimeBreakout
		confidence = 0.80

	// Trend Up: Bullish structure, above key EMAs, positive momentum
	case emaStackBullish && aboveEMA20 && return4W > 0:
		regime = RegimeTrendUp
		confidence = 0.70

	// Trend Down: Bearish structure, below key EMAs, negative momentum
	case emaStackBearish && !aboveEMA50 && return4W < 0:
		regime = RegimeTrendDown
		confidence = 0.70

	// Decay: Below 200 EMA, volume on down moves, significant losses
	case !aboveEMA200 && volRising && return12W < -10:
		regime = RegimeDecay
		confidence = 0.65

	// Distribution: Near highs but no volume expansion, weak recent returns
	case near52WkHigh && !volExpanding && return4W < 3:
		regime = RegimeDistribution
		confidence = 0.60

	// Accumulation: Flat price, rising volume with expansion
	case abs(return4W) < 5 && volRising && volExpanding:
		regime = RegimeAccumulation
		confidence = 0.55

	// Range: Mean-reverting, no clear trend
	case abs(return12W) < 10 && !emaStackBullish && !emaStackBearish:
		regime = RegimeRange
		confidence = 0.50

	// Undefined: No clear regime
	default:
		// Try to infer a basic direction
		if aboveEMA200 && return4W > 0 {
			regime = RegimeTrendUp
			confidence = 0.40
		} else if !aboveEMA200 && return4W < 0 {
			regime = RegimeTrendDown
			confidence = 0.40
		} else if near52WkLow && volExpanding {
			regime = RegimeAccumulation
			confidence = 0.45
		} else {
			regime = RegimeUndefined
			confidence = 0.30
		}
	}

	// Trend bias
	trendBias := "neutral"
	if aboveEMA200 {
		trendBias = "bullish"
	} else if !aboveEMA200 && price.EMA200 > 0 {
		trendBias = "bearish"
	}

	// EMA stack description
	emaStack := "mixed"
	if emaStackBullish {
		emaStack = "bullish"
	} else if emaStackBearish {
		emaStack = "bearish"
	}

	return RegimeSignal{
		Classification: string(regime),
		Confidence:     round(confidence, 2),
		TrendBias:      trendBias,
		EMAStack:       emaStack,
	}
}
